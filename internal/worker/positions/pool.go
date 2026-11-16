package positions

import (
	"context"
	"fmt"
	"log/slog"
	"sync"
	"time"

	"github.com/google/uuid"
	"go.opentelemetry.io/otel/metric"

	"github.com/deadOk-core/GoTemplate/internal/config"
	"github.com/deadOk-core/GoTemplate/internal/model"
)

type Pool struct {
	repository     staleTripRepository
	requester      *Requester
	logger         *slog.Logger
	checks         metric.Int64Counter
	settings       config.PositionsConfig
	requestTimeout time.Duration
	queue          chan *queuedTrip
	mu             sync.Mutex
	pending        map[uuid.UUID]*queuedTrip
}

type queuedTrip struct {
	trip    model.StaleTrip
	started bool
}

func NewPool(
	repository staleTripRepository,
	requester *Requester,
	logger *slog.Logger,
	meter metric.Meter,
	settings config.PositionsConfig,
	requestTimeout time.Duration,
) (*Pool, error) {
	checks, err := meter.Int64Counter("tripgo.stale_checks", metric.WithUnit("{trip}"))
	if err != nil {
		return nil, fmt.Errorf("create stale checks metric: %w", err)
	}
	return &Pool{
		repository:     repository,
		requester:      requester,
		logger:         logger,
		checks:         checks,
		settings:       settings,
		requestTimeout: requestTimeout,
		queue:          make(chan *queuedTrip, settings.QueueSize),
		pending:        make(map[uuid.UUID]*queuedTrip),
	}, nil
}

func (p *Pool) Run(scanCtx, workerCtx context.Context) {
	p.run(scanCtx, workerCtx, systemTicker{time.NewTicker(p.settings.CheckInterval)})
}

func (p *Pool) run(scanCtx, workerCtx context.Context, clock ticker) {
	var workers sync.WaitGroup
	for range p.settings.Workers {
		workers.Add(1)
		go func() {
			defer workers.Done()
			p.work(workerCtx)
		}()
	}
	p.scan(scanCtx, clock)
	close(p.queue)
	workers.Wait()
}

func (p *Pool) Cancel(ctx context.Context, tripID uuid.UUID) {
	if p.removeQueued(tripID) {
		p.requester.recordResult(ctx, "cancelled")
	}
}

func (p *Pool) removeQueued(tripID uuid.UUID) bool {
	p.mu.Lock()
	defer p.mu.Unlock()
	job, exists := p.pending[tripID]
	if !exists || job.started {
		return false
	}
	delete(p.pending, tripID)
	return true
}

func (p *Pool) work(ctx context.Context) {
	for {
		select {
		case <-ctx.Done():
			return
		case job, ok := <-p.queue:
			if !ok || ctx.Err() != nil {
				return
			}
			p.process(ctx, job)
		}
	}
}

func (p *Pool) process(ctx context.Context, job *queuedTrip) {
	if !p.claim(job) {
		return
	}
	requestCtx, cancel := context.WithTimeout(ctx, p.requestTimeout)
	defer cancel()
	p.requester.Request(requestCtx, job.trip)

	p.mu.Lock()
	delete(p.pending, job.trip.ID)
	p.mu.Unlock()
}

func (p *Pool) claim(job *queuedTrip) bool {
	p.mu.Lock()
	defer p.mu.Unlock()
	if p.pending[job.trip.ID] != job {
		return false
	}
	job.started = true
	return true
}
