package middleware_test

import (
	"context"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/go-chi/chi/v5"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/codes"
	"go.opentelemetry.io/otel/propagation"
	sdkmetric "go.opentelemetry.io/otel/sdk/metric"
	"go.opentelemetry.io/otel/sdk/metric/metricdata"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
	"go.opentelemetry.io/otel/sdk/trace/tracetest"
	"go.opentelemetry.io/otel/trace"

	"github.com/deadOk-core/GoTemplate/internal/http/middleware"
)

type logEntry struct {
	record slog.Record
	span   trace.SpanContext
}

type logHandler struct {
	slog.Handler
	entries []logEntry
}

func (h *logHandler) Handle(ctx context.Context, record slog.Record) error {
	h.entries = append(h.entries, logEntry{record: record.Clone(), span: trace.SpanContextFromContext(ctx)})
	return nil
}

func TestMiddleware(t *testing.T) {
	for _, status := range []int{200, 201, 400, 404, 409, 500, 0} {
		t.Run(http.StatusText(status), func(t *testing.T) {
			reader := sdkmetric.NewManualReader()
			meters := sdkmetric.NewMeterProvider(sdkmetric.WithReader(reader))

			t.Cleanup(func() { require.NoError(t, meters.Shutdown(context.Background())) })
			recorder := tracetest.NewSpanRecorder()
			tracers := sdktrace.NewTracerProvider(sdktrace.WithSpanProcessor(recorder))

			t.Cleanup(func() { require.NoError(t, tracers.Shutdown(context.Background())) })

			logs := &logHandler{Handler: slog.NewJSONHandler(io.Discard, nil)}
			router := chi.NewRouter()
			router.Get("/api/v1/trips/{tripId}", func(w http.ResponseWriter, r *http.Request) {
				var metrics metricdata.ResourceMetrics

				require.NoError(t, reader.Collect(r.Context(), &metrics))
				active := findMetric(t, metrics, "http.server.active_requests").Data.(metricdata.Sum[int64])
				require.Len(t, active.DataPoints, 1)
				assert.EqualValues(t, 1, active.DataPoints[0].Value)
				if status == 0 {
					panic("test panic")
				}
				if status != http.StatusOK {
					w.WriteHeader(status)
				}
			})
			chain, err := middleware.New(router, slog.New(logs), meters, tracers)
			require.NoError(t, err)
			request := httptest.NewRequest(http.MethodGet, "/api/v1/trips/123?secret=hidden", nil)
			parent := trace.NewSpanContext(trace.SpanContextConfig{
				TraceID: trace.TraceID{1}, SpanID: trace.SpanID{2}, TraceFlags: trace.FlagsSampled,
			})
			propagation.TraceContext{}.Inject(trace.ContextWithSpanContext(t.Context(), parent), propagation.HeaderCarrier(request.Header))
			response := httptest.NewRecorder()
			chain.Handler().ServeHTTP(response, request)
			wantStatus := status
			if status == 0 {
				wantStatus = http.StatusInternalServerError
				assert.Contains(t, response.Body.String(), `"code":"internal_error"`)
				assert.Equal(t, "application/problem+json", response.Header().Get("Content-Type"))
			}
			assert.Equal(t, wantStatus, response.Code)
			spans := recorder.Ended()
			require.Len(t, spans, 1)
			assert.Equal(t, "GET /api/v1/trips/{tripId}", spans[0].Name())
			assert.Equal(t, trace.SpanKindServer, spans[0].SpanKind())
			assert.Equal(t, parent.TraceID(), spans[0].SpanContext().TraceID())
			assert.Equal(t, parent.SpanID(), spans[0].Parent().SpanID())

			if wantStatus >= 500 {
				assert.Equal(t, codes.Error, spans[0].Status().Code)
				attrs := attribute.NewSet(spans[0].Attributes()...)
				assert.Equal(t, attribute.StringValue("500"), attributeValue(attrs, "error.type"))
				assert.Equal(t, attribute.IntValue(wantStatus), attributeValue(attrs, "http.response.status_code"))
			}

			accessCount := 0
			for _, entry := range logs.entries {
				if entry.record.Message != "request handled" {
					assert.Equal(t, "request panic", entry.record.Message)
					assert.Equal(t, spans[0].SpanContext().SpanID(), entry.span.SpanID())
					assert.Equal(t, spans[0].SpanContext().TraceID(), entry.span.TraceID())
					continue
				}

				accessCount++
				assert.Equal(t, spans[0].SpanContext().SpanID(), entry.span.SpanID())
				attrs := map[string]any{}
				entry.record.Attrs(func(attr slog.Attr) bool { attrs[attr.Key] = attr.Value.Any(); return true })
				assert.Equal(t, "/api/v1/trips/{tripId}", attrs["route"])
				assert.EqualValues(t, wantStatus, attrs["status"])
				wantLevel := slog.LevelInfo
				if wantStatus >= 500 {
					wantLevel = slog.LevelError
				}
				assert.Equal(t, wantLevel, entry.record.Level)
			}
			assert.Equal(t, 1, accessCount)

			var metrics metricdata.ResourceMetrics

			require.NoError(t, reader.Collect(t.Context(), &metrics))
			active := findMetric(t, metrics, "http.server.active_requests").Data.(metricdata.Sum[int64])
			assert.Zero(t, active.DataPoints[0].Value)
			duration := findMetric(t, metrics, "http.server.request.duration")
			assert.Equal(t, "s", duration.Unit)
			histogram := duration.Data.(metricdata.Histogram[float64])
			require.Len(t, histogram.DataPoints, 1)
			assert.EqualValues(t, 1, histogram.DataPoints[0].Count)
			attrs := histogram.DataPoints[0].Attributes
			assert.Equal(t, attribute.StringValue("/api/v1/trips/{tripId}"), attributeValue(attrs, "http.route"))
			assert.Equal(t, attribute.IntValue(wantStatus), attributeValue(attrs, "http.response.status_code"))
		})
	}
}

func findMetric(t *testing.T, metrics metricdata.ResourceMetrics, name string) metricdata.Metrics {
	t.Helper()
	for _, scope := range metrics.ScopeMetrics {
		for _, metric := range scope.Metrics {
			if metric.Name == name {
				return metric
			}
		}
	}
	t.Fatalf("metric %s not found", name)
	return metricdata.Metrics{}
}

func attributeValue(set attribute.Set, key string) attribute.Value {
	value, _ := set.Value(attribute.Key(key))
	return value
}
