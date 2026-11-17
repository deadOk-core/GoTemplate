package positions

import (
	"context"
	"fmt"

	"go.opentelemetry.io/otel/metric"
)

func (p *Pool) registerGauges(meter metric.Meter) error {
	queueSize, err := meter.Int64ObservableGauge("tripgo.position_requests.queue.size", metric.WithUnit("{request}"))
	if err != nil {
		return fmt.Errorf("create position queue size metric: %w", err)
	}
	activeWorkers, err := meter.Int64ObservableGauge("tripgo.position_requests.workers.active", metric.WithUnit("{worker}"))
	if err != nil {
		return fmt.Errorf("create active position workers metric: %w", err)
	}
	_, err = meter.RegisterCallback(func(_ context.Context, observer metric.Observer) error {
		observer.ObserveInt64(queueSize, int64(len(p.queue)))
		observer.ObserveInt64(activeWorkers, p.active.Load())
		return nil
	}, queueSize, activeWorkers)
	if err != nil {
		return fmt.Errorf("register position pool observer: %w", err)
	}
	return nil
}
