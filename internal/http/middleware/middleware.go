package middleware

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"net/http"
	"runtime/debug"
	"strconv"
	"time"

	"github.com/go-chi/chi/v5"
	chimiddleware "github.com/go-chi/chi/v5/middleware"
	"go.opentelemetry.io/contrib/instrumentation/net/http/otelhttp"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/codes"
	"go.opentelemetry.io/otel/metric"
	"go.opentelemetry.io/otel/metric/noop"
	"go.opentelemetry.io/otel/propagation"
	"go.opentelemetry.io/otel/trace"

	"github.com/deadOk-core/GoTemplate/internal/http/domainerror"
)

type Middleware struct {
	router   chi.Router
	logger   *slog.Logger
	tracers  trace.TracerProvider
	active   metric.Int64UpDownCounter
	duration metric.Float64Histogram
}

type requestKey struct{}

type requestState struct {
	ctx           context.Context
	writer        chimiddleware.WrapResponseWriter
	logRequest    func()
	recordMetrics func()
}

func (s *requestState) status() int {
	if status := s.writer.Status(); status != 0 {
		return status
	}
	return http.StatusOK
}

func New(router chi.Router, logger *slog.Logger, meters metric.MeterProvider,
	tracers trace.TracerProvider,
) (*Middleware, error) {
	meter := meters.Meter("trip-service/http")
	active, err := meter.Int64UpDownCounter(
		"http.server.active_requests", metric.WithUnit("{request}"),
	)
	if err != nil {
		return nil, fmt.Errorf("create active requests metric: %w", err)
	}
	duration, err := meter.Float64Histogram("http.server.request.duration", metric.WithUnit("s"),
		metric.WithExplicitBucketBoundaries(0.005, 0.01, 0.025, 0.05, 0.075, 0.1, 0.25, 0.5, 0.75, 1, 2.5, 5, 7.5, 10),
	)
	if err != nil {
		return nil, fmt.Errorf("create request duration metric: %w", err)
	}
	return &Middleware{router: router, logger: logger, tracers: tracers, active: active, duration: duration}, nil
}

func (m *Middleware) Handler() http.Handler {
	return m.Recover(m.Tracing(m.Logging(m.Metrics(m.router))))
}

func (m *Middleware) route(r *http.Request) string {
	routeContext := chi.NewRouteContext()
	if m.router.Match(routeContext, r.Method, r.URL.Path) {
		return routeContext.RoutePattern()
	}
	return "unmatched"
}

func (m *Middleware) Tracing(next http.Handler) http.Handler {
	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		state := r.Context().Value(requestKey{}).(*requestState)
		state.ctx = r.Context()
		span := trace.SpanFromContext(r.Context())
		completed := false
		defer func() {
			status := state.status()
			if !completed {
				status = http.StatusInternalServerError
				span.SetStatus(codes.Error, "request panic")
				span.SetAttributes(attribute.Int("http.response.status_code", status))
			}
			if status >= http.StatusInternalServerError {
				span.SetAttributes(attribute.String("error.type", strconv.Itoa(status)))
			}
		}()
		route := attribute.String("http.route", m.route(r))
		span.SetAttributes(route)
		next.ServeHTTP(w, r)
		completed = true
	})
	return otelhttp.NewHandler(handler, "HTTP",
		// Metrics records duration after the outer Recover has prepared the final response.
		otelhttp.WithMeterProvider(noop.NewMeterProvider()),
		otelhttp.WithTracerProvider(m.tracers),
		otelhttp.WithPropagators(propagation.TraceContext{}),
		otelhttp.WithSpanNameFormatter(func(_ string, r *http.Request) string {
			return r.Method + " " + m.route(r)
		}),
	)
}

func (m *Middleware) Logging(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		started := time.Now()
		state := r.Context().Value(requestKey{}).(*requestState)
		state.logRequest = func() {
			status := state.status()
			level := slog.LevelInfo
			if status >= http.StatusInternalServerError {
				level = slog.LevelError
			}
			m.logger.Log(r.Context(), level, "request handled",
				"method", r.Method, "route", m.route(r), "status", status,
				"duration_ms", float64(time.Since(started).Microseconds())/1000,
			)
		}
		next.ServeHTTP(w, r)
	})
}

func (m *Middleware) Metrics(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		started := time.Now()
		state := r.Context().Value(requestKey{}).(*requestState)
		attrs := metric.WithAttributes(
			attribute.String("http.request.method", r.Method),
			attribute.String("http.route", m.route(r)),
		)
		m.active.Add(r.Context(), 1, attrs)
		state.recordMetrics = func() {
			m.active.Add(r.Context(), -1, attrs)
			m.duration.Record(r.Context(), time.Since(started).Seconds(), metric.WithAttributes(
				attribute.String("http.request.method", r.Method),
				attribute.String("http.route", m.route(r)),
				attribute.Int("http.response.status_code", state.status()),
			))
		}
		next.ServeHTTP(w, r)
	})
}

func (m *Middleware) Recover(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		state := &requestState{ctx: r.Context(), writer: chimiddleware.NewWrapResponseWriter(w, r.ProtoMajor)}
		r = r.WithContext(context.WithValue(r.Context(), requestKey{}, state))
		// These actions run after the single recovery block, including on panic.
		defer func() {
			if state.recordMetrics != nil {
				state.recordMetrics()
			}
			if state.logRequest != nil {
				state.logRequest()
			}
		}()
		defer func() {
			if value := recover(); value != nil {
				m.logger.ErrorContext(state.ctx, "request panic", "panic", fmt.Sprint(value), "stack", string(debug.Stack()))
				status, problem := domainerror.Problem(fmt.Errorf("request panic"))
				state.writer.Header().Set("Content-Type", "application/problem+json")
				state.writer.WriteHeader(status)
				if err := json.NewEncoder(state.writer).Encode(problem); err != nil {
					m.logger.ErrorContext(state.ctx, "encode panic response", "error", err)
				}
			}
		}()
		next.ServeHTTP(state.writer, r)
	})
}
