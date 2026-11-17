package positions

import (
	"context"
	"time"

	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/codes"
	"go.opentelemetry.io/otel/metric"
	"go.opentelemetry.io/otel/trace"

	"github.com/deadOk-core/GoTemplate/internal/model"
)

const dropped = "dropped"

type systemTicker struct {
	*time.Ticker
}

func (t systemTicker) Ticks() <-chan time.Time {
	return t.C
}

func (p *Pool) scan(ctx context.Context, clock ticker) {
	defer clock.Stop()
	ticks := clock.Ticks()
	for {
		select {
		case <-ctx.Done():
			return
		case _, ok := <-ticks:
			if !ok || ctx.Err() != nil {
				return
			}
			p.scanOnce(ctx)
		}
	}
}

func (p *Pool) scanOnce(ctx context.Context) {
	ctx, span := p.tracer.Start(ctx, "positions.scan")
	defer span.End()
	scanSpan := span.SpanContext()
	trips, err := p.repository.ListStaleTrips(ctx, p.settings.StaleAfter)
	if err != nil {
		if ctx.Err() == nil {
			span.RecordError(err)
			span.SetStatus(codes.Error, "stale trip query failed")
			p.logger.WarnContext(ctx, "scan stale trips failed", "error", err)
		}
		return
	}
	for _, trip := range trips {
		if ctx.Err() != nil {
			return
		}
		result := p.enqueue(trip, scanSpan)
		p.checks.Add(ctx, 1, metric.WithAttributes(attribute.String("result", result)))
		if result == dropped {
			p.logger.WarnContext(ctx, "position request queue full", "trip_id", trip.ID.String())
		}
	}
}

func (p *Pool) enqueue(trip model.StaleTrip, scanSpan trace.SpanContext) string {
	p.mu.Lock()
	defer p.mu.Unlock()
	if _, exists := p.pending[trip.ID]; exists {
		return "duplicate"
	}
	job := &queuedTrip{trip: trip, scanSpan: scanSpan}
	select {
	case p.queue <- job:
		p.pending[trip.ID] = job
		return "queued"
	default:
		return dropped
	}
}
