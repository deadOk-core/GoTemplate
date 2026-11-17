package positions

import (
	"context"
	"fmt"
	"log/slog"
	"time"

	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/codes"
	"go.opentelemetry.io/otel/metric"
	"go.opentelemetry.io/otel/trace"

	"github.com/deadOk-core/GoTemplate/internal/model"
)

type Requester struct {
	gateway  pushGateway
	logger   *slog.Logger
	requests metric.Int64Counter
	duration metric.Float64Histogram
}

func NewRequester(gateway pushGateway, logger *slog.Logger, meter metric.Meter) (*Requester, error) {
	requests, err := meter.Int64Counter("tripgo.position_requests", metric.WithUnit("{request}"))
	if err != nil {
		return nil, fmt.Errorf("create position requests metric: %w", err)
	}
	duration, err := meter.Float64Histogram("tripgo.position_requests.duration", metric.WithUnit("s"),
		metric.WithExplicitBucketBoundaries(0.05, 0.1, 0.3, 0.8, 1, 2, 5))
	if err != nil {
		return nil, fmt.Errorf("create position request duration metric: %w", err)
	}
	return &Requester{gateway: gateway, logger: logger, requests: requests, duration: duration}, nil
}

func (r *Requester) Request(ctx context.Context, trip model.StaleTrip) {
	started := time.Now()
	err := r.gateway.RequestPosition(ctx, trip.DriverID, trip.ID)
	duration := time.Since(started).Seconds()
	result := "success"
	if err != nil {
		result = "error"
		span := trace.SpanFromContext(ctx)
		span.RecordError(err)
		span.SetStatus(codes.Error, "position request failed")
		r.logger.WarnContext(ctx, "request trip position failed", "trip_id", trip.ID.String(), "error", err)
	}
	r.recordResult(ctx, result)
	r.duration.Record(ctx, duration, metric.WithAttributes(attribute.String("result", result)))
}

func (r *Requester) recordResult(ctx context.Context, result string) {
	r.requests.Add(ctx, 1, metric.WithAttributes(attribute.String("result", result)))
}
