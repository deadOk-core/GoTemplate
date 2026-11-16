package positions

import (
	"context"
	"fmt"
	"log/slog"

	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/metric"

	"github.com/deadOk-core/GoTemplate/internal/model"
)

type Requester struct {
	gateway  pushGateway
	logger   *slog.Logger
	requests metric.Int64Counter
}

func NewRequester(gateway pushGateway, logger *slog.Logger, meter metric.Meter) (*Requester, error) {
	requests, err := meter.Int64Counter("tripgo.position_requests", metric.WithUnit("{request}"))
	if err != nil {
		return nil, fmt.Errorf("create position requests metric: %w", err)
	}
	return &Requester{gateway: gateway, logger: logger, requests: requests}, nil
}

func (r *Requester) Request(ctx context.Context, trip model.StaleTrip) {
	result := "success"
	if err := r.gateway.RequestPosition(ctx, trip.DriverID, trip.ID); err != nil {
		result = "error"
		r.logger.WarnContext(ctx, "request trip position failed", "trip_id", trip.ID.String(), "error", err)
	}
	r.recordResult(ctx, result)
}

func (r *Requester) recordResult(ctx context.Context, result string) {
	r.requests.Add(ctx, 1, metric.WithAttributes(attribute.String("result", result)))
}
