package positions_test

import (
	"bytes"
	"context"
	"errors"
	"log/slog"
	"sync/atomic"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/otel/attribute"
	sdkmetric "go.opentelemetry.io/otel/sdk/metric"
	"go.opentelemetry.io/otel/sdk/metric/metricdata"
	"go.uber.org/mock/gomock"

	"github.com/deadOk-core/GoTemplate/internal/config"
	"github.com/deadOk-core/GoTemplate/internal/model"
	"github.com/deadOk-core/GoTemplate/internal/worker/positions"
)

const (
	staleAfter    = time.Minute
	resultSuccess = "success"
)

type poolFixture struct {
	pool     *positions.Pool
	repo     *MockstaleTripRepository
	gateway  *MockpushGateway
	reader   *sdkmetric.ManualReader
	logs     *bytes.Buffer
	ctx      context.Context
	stopScan context.CancelFunc
	ticks    chan time.Time
	done     chan struct{}
}

func newPoolFixture(t *testing.T, workers, capacity int, requestTimeout time.Duration) *poolFixture {
	t.Helper()
	ctrl := gomock.NewController(t)
	reader := sdkmetric.NewManualReader()
	provider := sdkmetric.NewMeterProvider(sdkmetric.WithReader(reader))
	t.Cleanup(func() { assert.NoError(t, provider.Shutdown(context.Background())) })
	meter := provider.Meter("positions-test")
	logs := &bytes.Buffer{}
	logger := slog.New(slog.NewJSONHandler(logs, nil))
	gateway := NewMockpushGateway(ctrl)
	requester, err := positions.NewRequester(gateway, logger, meter)
	require.NoError(t, err)
	repo := NewMockstaleTripRepository(ctrl)
	settings := config.PositionsConfig{
		CheckInterval: 30 * time.Second,
		StaleAfter:    staleAfter,
		Workers:       workers,
		QueueSize:     capacity,
	}
	pool, err := positions.NewPool(repo, requester, logger, meter, settings, requestTimeout)
	require.NoError(t, err)
	ctx, cancel := context.WithTimeout(t.Context(), 5*time.Second)
	scanCtx, stopScan := context.WithCancel(ctx)
	ticks := make(chan time.Time)
	clock := NewMockticker(ctrl)
	clock.EXPECT().Ticks().Return((<-chan time.Time)(ticks))
	clock.EXPECT().Stop()
	done := make(chan struct{})
	go func() {
		defer close(done)
		pool.RunOnTicker(scanCtx, ctx, clock)
	}()
	t.Cleanup(func() {
		stopScan()
		cancel()
		select {
		case <-done:
		case <-time.After(time.Second):
			t.Error("positions pool did not stop")
		}
	})
	return &poolFixture{
		pool:     pool,
		repo:     repo,
		gateway:  gateway,
		reader:   reader,
		logs:     logs,
		ctx:      ctx,
		stopScan: stopScan,
		ticks:    ticks,
		done:     done,
	}
}

func (f *poolFixture) sendTick(t *testing.T) {
	t.Helper()
	select {
	case f.ticks <- time.Time{}:
	case <-f.ctx.Done():
		t.Fatal("scanner failed to accept a tick", f.ctx.Err())
	}
}

func (f *poolFixture) scan(t *testing.T, trips ...model.StaleTrip) {
	t.Helper()
	queried := make(chan struct{})
	f.repo.EXPECT().ListStaleTrips(gomock.Any(), staleAfter).
		DoAndReturn(func(context.Context, time.Duration) ([]model.StaleTrip, error) {
			close(queried)
			return trips, nil
		})
	f.sendTick(t)
	waitValue(t, f.ctx, queried)
}

func (f *poolFixture) stop(t *testing.T) {
	t.Helper()
	f.stopScan()
	waitValue(t, f.ctx, f.done)
}

func (f *poolFixture) results(t *testing.T, name string) map[string]int64 {
	t.Helper()
	var metrics metricdata.ResourceMetrics
	require.NoError(t, f.reader.Collect(t.Context(), &metrics))
	for _, scope := range metrics.ScopeMetrics {
		for _, instrument := range scope.Metrics {
			if instrument.Name != name {
				continue
			}
			sum, ok := instrument.Data.(metricdata.Sum[int64])
			require.True(t, ok)
			results := make(map[string]int64, len(sum.DataPoints))
			for _, point := range sum.DataPoints {
				result, ok := point.Attributes.Value(attribute.Key("result"))
				require.True(t, ok)
				results[result.AsString()] = point.Value
			}
			return results
		}
	}
	return nil
}

func waitValue[T any](t *testing.T, ctx context.Context, source <-chan T) T {
	t.Helper()
	select {
	case value := <-source:
		return value
	case <-ctx.Done():
		t.Fatal("timed out waiting for pool", ctx.Err())
		var zero T
		return zero
	}
}

func newTrip() model.StaleTrip {
	return model.StaleTrip{ID: uuid.New(), DriverID: uuid.New()}
}

func TestPoolDeduplicatesAndDropsWithoutBlocking(t *testing.T) {
	f := newPoolFixture(t, 1, 1, time.Minute)
	active, queued, dropped := newTrip(), newTrip(), newTrip()
	started := make(chan struct{})
	release := make(chan struct{})
	queuedStarted := make(chan struct{})
	first := f.gateway.EXPECT().RequestPosition(gomock.Any(), active.DriverID, active.ID).
		DoAndReturn(func(ctx context.Context, _, _ uuid.UUID) error {
			close(started)
			select {
			case <-release:
				return nil
			case <-ctx.Done():
				return ctx.Err()
			}
		})
	second := f.gateway.EXPECT().RequestPosition(gomock.Any(), queued.DriverID, queued.ID).
		Do(func(context.Context, uuid.UUID, uuid.UUID) { close(queuedStarted) }).Return(nil).After(first)
	f.gateway.EXPECT().RequestPosition(gomock.Any(), dropped.DriverID, dropped.ID).Return(nil).After(second)

	f.scan(t, active)
	waitValue(t, f.ctx, started)
	f.pool.Cancel(t.Context(), active.ID)
	f.scan(t, queued)
	f.scan(t, queued, active, dropped)
	f.scan(t)
	assert.Equal(t, map[string]int64{"queued": 2, "duplicate": 2, "dropped": 1}, f.results(t, "tripgo.stale_checks"))
	assert.Empty(t, f.results(t, "tripgo.position_requests"))
	close(release)
	waitValue(t, f.ctx, queuedStarted)
	f.scan(t, dropped)
	f.scan(t)
	f.stop(t)

	assert.Equal(t, map[string]int64{resultSuccess: 3}, f.results(t, "tripgo.position_requests"))
	assert.Contains(t, f.logs.String(), `"level":"WARN"`)
	assert.Contains(t, f.logs.String(), dropped.ID.String())
}

func TestPoolSkipsCancelledQueueEntryAfterReschedule(t *testing.T) {
	f := newPoolFixture(t, 1, 3, time.Minute)
	active, cancelled, next := newTrip(), newTrip(), newTrip()
	started := make(chan struct{})
	release := make(chan struct{})
	first := f.gateway.EXPECT().RequestPosition(gomock.Any(), active.DriverID, active.ID).
		DoAndReturn(func(ctx context.Context, _, _ uuid.UUID) error {
			close(started)
			select {
			case <-release:
				return nil
			case <-ctx.Done():
				return ctx.Err()
			}
		})
	second := f.gateway.EXPECT().RequestPosition(gomock.Any(), next.DriverID, next.ID).Return(nil).After(first)
	f.gateway.EXPECT().RequestPosition(gomock.Any(), cancelled.DriverID, cancelled.ID).Return(nil).After(second)

	f.scan(t, active)
	waitValue(t, f.ctx, started)
	f.scan(t, cancelled, next)
	f.scan(t)
	f.pool.Cancel(t.Context(), cancelled.ID)
	f.pool.Cancel(t.Context(), cancelled.ID)
	f.scan(t, cancelled)
	f.scan(t)
	assert.Equal(t, map[string]int64{"cancelled": 1}, f.results(t, "tripgo.position_requests"))
	close(release)
	f.stop(t)
	assert.Equal(t, map[string]int64{resultSuccess: 3, "cancelled": 1}, f.results(t, "tripgo.position_requests"))
}

func TestPoolProcessesAllTripsWithConfiguredWorkerCount(t *testing.T) {
	const workers = 2
	trips := []model.StaleTrip{newTrip(), newTrip(), newTrip(), newTrip(), newTrip()}
	f := newPoolFixture(t, workers, len(trips), time.Minute)
	started := make(chan uuid.UUID, len(trips))
	release := make(chan struct{})
	var active atomic.Int32
	for _, trip := range trips {
		f.gateway.EXPECT().RequestPosition(gomock.Any(), trip.DriverID, trip.ID).
			DoAndReturn(func(ctx context.Context, _, tripID uuid.UUID) error {
				assert.LessOrEqual(t, active.Add(1), int32(workers))
				defer active.Add(-1)
				started <- tripID
				select {
				case <-release:
					return nil
				case <-ctx.Done():
					return ctx.Err()
				}
			})
	}

	f.scan(t, trips...)
	first := waitValue(t, f.ctx, started)
	second := waitValue(t, f.ctx, started)
	assert.NotEqual(t, first, second)
	assert.EqualValues(t, workers, active.Load())
	f.scan(t)
	close(release)
	f.stop(t)
	assert.Equal(t, map[string]int64{resultSuccess: int64(len(trips))}, f.results(t, "tripgo.position_requests"))
}

func TestPoolContinuesAfterPushErrorAndAppliesTaskTimeout(t *testing.T) {
	const requestTimeout = 25 * time.Millisecond
	f := newPoolFixture(t, 1, 3, requestTimeout)
	failed, timedOut, next := newTrip(), newTrip(), newTrip()
	first := f.gateway.EXPECT().RequestPosition(gomock.Any(), failed.DriverID, failed.ID).Return(errors.New("push unavailable"))
	second := f.gateway.EXPECT().RequestPosition(gomock.Any(), timedOut.DriverID, timedOut.ID).
		DoAndReturn(func(ctx context.Context, _, _ uuid.UUID) error {
			<-ctx.Done()
			assert.ErrorIs(t, ctx.Err(), context.DeadlineExceeded)
			return ctx.Err()
		}).After(first)
	f.gateway.EXPECT().RequestPosition(gomock.Any(), next.DriverID, next.ID).Return(nil).After(second)

	f.scan(t, failed, timedOut, next)
	f.scan(t)
	f.stop(t)
	assert.Equal(t, map[string]int64{resultSuccess: 1, "error": 2}, f.results(t, "tripgo.position_requests"))
	assert.Contains(t, f.logs.String(), "push unavailable")
	assert.Contains(t, f.logs.String(), "context deadline exceeded")
}

func TestPoolDoesNotOverlapScans(t *testing.T) {
	f := newPoolFixture(t, 1, 1, time.Minute)
	entered := make(chan struct{})
	release := make(chan struct{})
	f.repo.EXPECT().ListStaleTrips(gomock.Any(), staleAfter).
		DoAndReturn(func(ctx context.Context, _ time.Duration) ([]model.StaleTrip, error) {
			close(entered)
			select {
			case <-release:
				return nil, nil
			case <-ctx.Done():
				return nil, ctx.Err()
			}
		})
	f.sendTick(t)
	waitValue(t, f.ctx, entered)
	select {
	case f.ticks <- time.Time{}:
		t.Fatal("scanner accepted another tick during a blocked query")
	default:
	}
	close(release)
	f.scan(t)
	f.stop(t)
}
