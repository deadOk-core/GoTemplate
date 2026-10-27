package telemetry

import (
	"context"
	"fmt"

	"go.opentelemetry.io/otel/metric"
)

func RegisterActiveTrips(repository activeTripCounter, meter metric.Meter) error {
	gauge, err := meter.Int64ObservableGauge("tripgo.trips.active", metric.WithUnit("{trip}"))
	if err != nil {
		return fmt.Errorf("create active trips metric: %w", err)
	}
	_, err = meter.RegisterCallback(func(ctx context.Context, observer metric.Observer) error {
		count, err := repository.CountActive(ctx)
		if err != nil {
			return fmt.Errorf("observe active trips: %w", err)
		}
		observer.ObserveInt64(gauge, count)
		return nil
	}, gauge)
	if err != nil {
		return fmt.Errorf("register active trips observer: %w", err)
	}
	return nil
}
