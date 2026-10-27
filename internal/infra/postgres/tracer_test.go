package postgres

import (
	"context"
	"testing"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/codes"
	sdkmetric "go.opentelemetry.io/otel/sdk/metric"
	"go.opentelemetry.io/otel/sdk/metric/metricdata"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
	"go.opentelemetry.io/otel/sdk/trace/tracetest"
)

func TestQueryTracer(t *testing.T) {
	const tripsCollection = "trips"
	recorder := tracetest.NewSpanRecorder()
	provider := sdktrace.NewTracerProvider(sdktrace.WithSpanProcessor(recorder))
	previous := otel.GetTracerProvider()
	otel.SetTracerProvider(provider)
	t.Cleanup(func() {
		otel.SetTracerProvider(previous)
		require.NoError(t, provider.Shutdown(context.Background()))
	})
	reader := sdkmetric.NewManualReader()
	meters := sdkmetric.NewMeterProvider(sdkmetric.WithReader(reader))
	t.Cleanup(func() { require.NoError(t, meters.Shutdown(context.Background())) })
	tracer, err := newQueryTracer(meters)
	require.NoError(t, err)
	ctx, parent := provider.Tracer("test").Start(t.Context(), "HTTP request")
	tests := []struct {
		sql, operation, collection string
		err                        error
		errorType                  string
	}{
		{sql: "SELECT id FROM trips WHERE id = $1", operation: "SELECT", collection: tripsCollection, err: pgx.ErrNoRows},
		{sql: "INSERT INTO trip_status_history (trip_id) VALUES ($1)", operation: "INSERT", collection: "trip_status_history"},
		{sql: "UPDATE trips SET status = $1", operation: "UPDATE", collection: tripsCollection},
		{sql: "INSERT INTO trips (id) VALUES ($1)", operation: "INSERT", collection: tripsCollection, err: &pgconn.PgError{Code: "23505"}, errorType: "23505"},
		{sql: "SELECT id FROM trips", operation: "SELECT", collection: tripsCollection, err: context.DeadlineExceeded, errorType: "timeout"},
	}
	for _, tt := range tests {
		queryCtx := tracer.TraceQueryStart(ctx, nil, pgx.TraceQueryStartData{SQL: tt.sql, Args: []any{"private"}})
		tracer.TraceQueryEnd(queryCtx, nil, pgx.TraceQueryEndData{Err: tt.err})
	}
	parent.End()
	spans := recorder.Ended()
	require.Len(t, spans, len(tests)+1)
	for i, tt := range tests {
		attrs := attribute.NewSet(spans[i].Attributes()...)
		value, _ := attrs.Value("db.operation.name")
		assert.Equal(t, tt.operation, value.AsString())
		value, _ = attrs.Value("db.collection.name")
		assert.Equal(t, tt.collection, value.AsString())
		value, _ = attrs.Value("error.type")
		assert.Equal(t, tt.errorType, value.AsString())
		assert.Equal(t, parent.SpanContext().SpanID(), spans[i].Parent().SpanID())
		if tt.errorType != "" {
			assert.Equal(t, codes.Error, spans[i].Status().Code)
		}
		_, hasQuery := attrs.Value("db.query.text")
		assert.False(t, hasQuery)
	}
	var metrics metricdata.ResourceMetrics
	require.NoError(t, reader.Collect(t.Context(), &metrics))
	require.Len(t, metrics.ScopeMetrics, 1)
	require.Len(t, metrics.ScopeMetrics[0].Metrics, 1, "no duplicate built-in metric")
	metric := metrics.ScopeMetrics[0].Metrics[0]
	assert.Equal(t, "db.client.operation.duration", metric.Name)
	assert.Equal(t, "s", metric.Unit)
	histogram := metric.Data.(metricdata.Histogram[float64])
	var count uint64
	for _, point := range histogram.DataPoints {
		count += point.Count
		assert.True(t, point.Attributes.HasValue("db.operation.name"))
		assert.True(t, point.Attributes.HasValue("db.collection.name"))
		assert.True(t, point.Attributes.HasValue("error.type"))
	}
	assert.EqualValues(t, len(tests), count)
}
