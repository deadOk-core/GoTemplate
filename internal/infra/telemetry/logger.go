package telemetry

import (
	"context"
	"io"
	"log/slog"

	"go.opentelemetry.io/otel/trace"
)

type traceHandler struct {
	slog.Handler
}

func NewLogger(
	out io.Writer,
	level slog.Level,
	service string,
) *slog.Logger {
	handler := slog.NewJSONHandler(out, &slog.HandlerOptions{
		Level: level,
	})

	return slog.New(&traceHandler{
		Handler: handler,
	}).With("service", service)
}

func (h *traceHandler) Handle(
	ctx context.Context,
	record slog.Record,
) error {
	spanContext := trace.SpanContextFromContext(ctx)

	traceID := ""
	spanID := ""

	if spanContext.IsValid() {
		traceID = spanContext.TraceID().String()
		spanID = spanContext.SpanID().String()
	}

	record.AddAttrs(
		slog.String("trace_id", traceID),
		slog.String("span_id", spanID),
	)

	return h.Handler.Handle(ctx, record)
}

func (h *traceHandler) WithAttrs(attrs []slog.Attr) slog.Handler {
	return &traceHandler{
		Handler: h.Handler.WithAttrs(attrs),
	}
}

func (h *traceHandler) WithGroup(name string) slog.Handler {
	return &traceHandler{
		Handler: h.Handler.WithGroup(name),
	}
}
