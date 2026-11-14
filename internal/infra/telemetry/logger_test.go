package telemetry_test

import (
	"bytes"
	"encoding/json"
	"log/slog"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/otel/trace"

	"github.com/deadOk-core/GoTemplate/internal/infra/telemetry"
)

const serviceName = "trip-service"

func TestLoggerTraceContext(t *testing.T) {
	spanContext := trace.NewSpanContext(trace.SpanContextConfig{
		TraceID: trace.TraceID{1},
		SpanID:  trace.SpanID{2},
	})

	tests := []struct {
		name        string
		spanContext trace.SpanContext
		wantTraceID string
		wantSpanID  string
	}{
		{
			name: "without span",
		},
		{
			name:        "with span",
			spanContext: spanContext,
			wantTraceID: spanContext.TraceID().String(),
			wantSpanID:  spanContext.SpanID().String(),
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var output bytes.Buffer

			logger := telemetry.NewLogger(
				&output,
				slog.LevelInfo,
				serviceName,
			).With("component", "test")

			ctx := trace.ContextWithSpanContext(
				t.Context(),
				tt.spanContext,
			)

			logger.InfoContext(ctx, "test event")

			var entry map[string]any
			require.NoError(
				t,
				json.Unmarshal(output.Bytes(), &entry),
			)

			assert.NotEmpty(t, entry["time"])
			assert.Equal(t, "INFO", entry["level"])
			assert.Equal(t, "test event", entry["msg"])
			assert.Equal(t, serviceName, entry["service"])
			assert.Equal(t, "test", entry["component"])
			assert.Equal(t, tt.wantTraceID, entry["trace_id"])
			assert.Equal(t, tt.wantSpanID, entry["span_id"])
		})
	}
}

func TestLoggerLevel(t *testing.T) {
	var output bytes.Buffer

	logger := telemetry.NewLogger(
		&output,
		slog.LevelWarn,
		serviceName,
	)

	logger.DebugContext(t.Context(), "debug event")
	logger.InfoContext(t.Context(), "info event")

	require.Empty(t, output.String())

	logger.ErrorContext(t.Context(), "error event")

	var entry map[string]any
	require.NoError(
		t,
		json.Unmarshal(output.Bytes(), &entry),
	)

	assert.Equal(t, "ERROR", entry["level"])
	assert.Equal(t, "error event", entry["msg"])
}
