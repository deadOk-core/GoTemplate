package postgres

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/exaring/otelpgx"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/metric"
	"go.opentelemetry.io/otel/metric/noop"
	"go.opentelemetry.io/otel/trace"
)

// otelpgx owns spans; this adapter supplies the collection/error labels required by Lab2.
type queryTracer struct {
	*otelpgx.Tracer
	duration metric.Float64Histogram
}

type queryKey struct{}

type queryMeasurement struct {
	started time.Time
	attrs   []attribute.KeyValue
}

func newQueryTracer(meters metric.MeterProvider) (*queryTracer, error) {
	duration, err := meters.Meter("trip-service/postgres").Float64Histogram(
		"db.client.operation.duration", metric.WithUnit("s"),
		metric.WithExplicitBucketBoundaries(0.001, 0.005, 0.01, 0.05, 0.1, 0.5, 1, 5, 10),
	)
	if err != nil {
		return nil, fmt.Errorf("create database duration metric: %w", err)
	}
	return &queryTracer{
		Tracer: otelpgx.NewTracer(
			// Record duration once here: otelpgx does not provide per-query collection/error labels.
			otelpgx.WithMeterProvider(noop.NewMeterProvider()),
			otelpgx.WithDisableAcquireTracer(),
			otelpgx.WithDisableSQLStatementInAttributes(),
		),
		duration: duration,
	}, nil
}

func (t *queryTracer) TraceQueryStart(ctx context.Context, conn *pgx.Conn, data pgx.TraceQueryStartData) context.Context {
	ctx = t.Tracer.TraceQueryStart(ctx, conn, data)
	return startQuery(ctx, data.SQL)
}

func (t *queryTracer) TraceQueryEnd(ctx context.Context, conn *pgx.Conn, data pgx.TraceQueryEndData) {
	t.endQuery(ctx, data.Err)
	t.Tracer.TraceQueryEnd(ctx, conn, data)
}

func (t *queryTracer) TracePrepareStart(ctx context.Context, conn *pgx.Conn, data pgx.TracePrepareStartData) context.Context {
	ctx = t.Tracer.TracePrepareStart(ctx, conn, data)
	trace.SpanFromContext(ctx).SetAttributes(queryAttributes(ctx, data.SQL)...)
	return ctx
}

func (t *queryTracer) TracePrepareEnd(ctx context.Context, conn *pgx.Conn, data pgx.TracePrepareEndData) {
	setQueryError(ctx, data.Err)
	t.Tracer.TracePrepareEnd(ctx, conn, data)
}

func startQuery(ctx context.Context, sql string) context.Context {
	attrs := queryAttributes(ctx, sql)
	trace.SpanFromContext(ctx).SetAttributes(attrs...)
	return context.WithValue(ctx, queryKey{}, queryMeasurement{started: time.Now(), attrs: attrs})
}

func (t *queryTracer) endQuery(ctx context.Context, err error) {
	setQueryError(ctx, err)
	measurement, ok := ctx.Value(queryKey{}).(queryMeasurement)
	if !ok {
		return
	}
	measurement.attrs = append(measurement.attrs, attribute.String("error.type", queryErrorType(err)))
	t.duration.Record(ctx, time.Since(measurement.started).Seconds(), metric.WithAttributes(measurement.attrs...))
}

func (t *queryTracer) TraceConnectEnd(ctx context.Context, data pgx.TraceConnectEndData) {
	setQueryError(ctx, data.Err)
	t.Tracer.TraceConnectEnd(ctx, data)
}

func setQueryError(ctx context.Context, err error) {
	if kind := queryErrorType(err); kind != "" {
		trace.SpanFromContext(ctx).SetAttributes(attribute.String("error.type", kind))
	}
}

func queryErrorType(err error) string {
	if err == nil || errors.Is(err, pgx.ErrNoRows) {
		return ""
	}
	var pgErr *pgconn.PgError
	if errors.As(err, &pgErr) {
		return pgErr.Code
	}
	if errors.Is(err, context.DeadlineExceeded) {
		return "timeout"
	}
	if errors.Is(err, context.Canceled) {
		return "canceled"
	}
	return "database_error"
}

func queryAttributes(ctx context.Context, sql string) []attribute.KeyValue {
	operation := otelpgx.SQLOperationName(ctx, sql)
	collection := ""
	// Only the fixed tables used by this service are metric labels; SQL/arguments are never labels.
	words := strings.Fields(sql)
	for i := 0; i+1 < len(words); i++ {
		switch strings.ToUpper(words[i]) {
		case "FROM", "INTO", "UPDATE":
			name := strings.Trim(words[i+1], "\"();")
			if name == "trips" || name == "trip_status_history" {
				collection = name
			}
		}
	}
	return []attribute.KeyValue{
		attribute.String("db.operation.name", operation),
		attribute.String("db.collection.name", collection),
	}
}
