package telemetry_test

import (
	"context"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/otel"
	collectormetric "go.opentelemetry.io/proto/otlp/collector/metrics/v1"
	collectortrace "go.opentelemetry.io/proto/otlp/collector/trace/v1"
	"google.golang.org/protobuf/proto"

	"github.com/deadOk-core/GoTemplate/internal/config"
	"github.com/deadOk-core/GoTemplate/internal/infra/telemetry"
)

func TestSetupExportsSignals(t *testing.T) {
	var mu sync.Mutex
	traces := &collectortrace.ExportTraceServiceRequest{}
	metrics := &collectormetric.ExportMetricsServiceRequest{}
	receiver := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, err := io.ReadAll(r.Body)
		if !assert.NoError(t, err) {
			w.WriteHeader(http.StatusBadRequest)
			return
		}
		mu.Lock()
		defer mu.Unlock()
		var message proto.Message
		switch r.URL.Path {
		case "/v1/traces":
			message = traces
		case "/v1/metrics":
			message = metrics
		default:
			t.Errorf("unexpected OTLP path: %s", r.URL.Path)
			w.WriteHeader(http.StatusNotFound)
			return
		}
		if !assert.NoError(t, proto.Unmarshal(body, message)) {
			w.WriteHeader(http.StatusBadRequest)
			return
		}
		w.Header().Set("Content-Type", "application/x-protobuf")
	}))

	t.Cleanup(receiver.Close)
	configureSDKTest(t, receiver.URL)
	shutdown, err := telemetry.Setup(t.Context(), config.Config{ServiceName: "trip-service", ShutdownTimeout: time.Second}, slog.New(slog.DiscardHandler))
	require.NoError(t, err)
	closed := false

	t.Cleanup(func() {
		if !closed {
			ctx, cancel := context.WithTimeout(context.Background(), time.Second)
			defer cancel()
			require.NoError(t, shutdown(ctx))
		}
	})

	ctx, span := otel.Tracer("test").Start(t.Context(), "test request")
	counter, err := otel.Meter("test").Int64Counter("tripgo.trips.created")
	require.NoError(t, err)
	counter.Add(ctx, 1)
	span.End()

	flushCtx, cancel := context.WithTimeout(t.Context(), 2*time.Second)
	defer cancel()

	closed = true
	require.NoError(t, shutdown(flushCtx))
	mu.Lock()
	defer mu.Unlock()

	require.Len(t, traces.ResourceSpans, 1)
	require.Len(t, metrics.ResourceMetrics, 1)
	attrs := map[string]string{}
	for _, attr := range traces.ResourceSpans[0].Resource.Attributes {
		attrs[attr.Key] = attr.Value.GetStringValue()
	}

	assert.Equal(t, "trip-service", attrs["service.name"])
	assert.Equal(t, "tripgo", attrs["service.namespace"])
	assert.Equal(t, "local", attrs["deployment.environment"])
	assert.NotEmpty(t, attrs["service.version"])
	assert.EqualValues(t, 1, metrics.ResourceMetrics[0].ScopeMetrics[0].Metrics[0].GetSum().DataPoints[0].GetAsInt())
}

func TestSetupWithoutCollector(t *testing.T) {
	receiver := httptest.NewServer(http.NotFoundHandler())
	receiver.Close()
	configureSDKTest(t, receiver.URL)
	shutdown, err := telemetry.Setup(t.Context(), config.Config{ServiceName: "trip-service", ShutdownTimeout: time.Second}, slog.New(slog.DiscardHandler))
	require.NoError(t, err, "collector is not a startup dependency")
	_, span := otel.Tracer("test").Start(t.Context(), "request without collector")
	span.End()
	ctx, cancel := context.WithTimeout(t.Context(), 50*time.Millisecond)
	defer cancel()
	started := time.Now()
	assert.Error(t, shutdown(ctx))
	assert.Less(t, time.Since(started), time.Second, "shutdown must respect its budget")
}

func configureSDKTest(t *testing.T, endpoint string) {
	t.Helper()
	for _, signal := range []string{"", "TRACES_", "METRICS_"} {
		for _, option := range []string{"ENDPOINT", "HEADERS", "COMPRESSION"} {
			t.Setenv("OTEL_EXPORTER_OTLP_"+signal+option, "")
		}
	}
	t.Setenv("OTEL_EXPORTER_OTLP_ENDPOINT", endpoint)
	t.Setenv("OTEL_RESOURCE_ATTRIBUTES", "service.namespace=tripgo,deployment.environment=local")
	t.Setenv("OTEL_TRACES_SAMPLER", "always_on")
	t.Setenv("OTEL_METRIC_EXPORT_INTERVAL", "60000")
	traces, metrics, propagator, handler := otel.GetTracerProvider(), otel.GetMeterProvider(), otel.GetTextMapPropagator(), otel.GetErrorHandler()
	t.Cleanup(func() {
		otel.SetTracerProvider(traces)
		otel.SetMeterProvider(metrics)
		otel.SetTextMapPropagator(propagator)
		otel.SetErrorHandler(handler)
	})
}
