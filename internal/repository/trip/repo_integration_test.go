//go:build integration

package trip_test

import (
	"context"
	"errors"
	"os"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	domaintrip "github.com/deadOk-core/GoTemplate/internal/domain/trip"
	"github.com/deadOk-core/GoTemplate/internal/infra/postgres/transaction"
	"github.com/deadOk-core/GoTemplate/internal/model"
	triprepository "github.com/deadOk-core/GoTemplate/internal/repository/trip"
)

func newTestRepository(t *testing.T) (*pgxpool.Pool, *triprepository.Repository) {
	t.Helper()

	databaseURL := os.Getenv("TEST_DATABASE_URL")
	require.NotEmpty(t, databaseURL, "set TEST_DATABASE_URL")

	cfg, err := pgxpool.ParseConfig(databaseURL)
	require.NoError(t, err)

	cfg.MaxConns = 2
	cfg.MinConns = 0

	ctx, cancel := context.WithTimeout(t.Context(), 5*time.Second)
	defer cancel()

	pool, err := pgxpool.NewWithConfig(ctx, cfg)
	require.NoError(t, err)
	t.Cleanup(pool.Close)

	require.NoError(t, pool.Ping(ctx))

	return pool, triprepository.NewRepository(pool, 5*time.Second)
}

func TestRepositoryCreateAndGet(t *testing.T) {
	pool, repository := newTestRepository(t)
	ctx := t.Context()

	params := domaintrip.CreateParams{
		UserID:         uuid.New(),
		DriverID:       uuid.New(),
		StartLatitude:  55.75,
		StartLongitude: 37.61,
		EndLatitude:    55.80,
		EndLongitude:   37.70,
		Price:          1500,
	}

	t.Cleanup(func() {
		cleanupCtx, cancel := context.WithTimeout(
			context.Background(),
			5*time.Second,
		)
		defer cancel()

		_, err := pool.Exec(
			cleanupCtx,
			"DELETE FROM trips WHERE driver_id = $1",
			params.DriverID,
		)
		assert.NoError(t, err)
	})

	startedAt := time.Now().UTC().Truncate(time.Microsecond)

	entity, err := domaintrip.NewTrip(params, startedAt)
	require.NoError(t, err)

	created, err := repository.Create(ctx, model.FromDomain(entity))
	require.NoError(t, err)

	found, err := repository.GetByID(ctx, created.ID)
	require.NoError(t, err)

	assert.Equal(t, entity.ID, found.ID)
	assert.Equal(t, params.UserID, found.UserID)
	assert.Equal(t, params.DriverID, found.DriverID)
	assert.Equal(t, params.StartLatitude, found.StartLatitude)
	assert.Equal(t, params.StartLongitude, found.StartLongitude)
	assert.Equal(t, params.EndLatitude, found.EndLatitude)
	assert.Equal(t, params.EndLongitude, found.EndLongitude)
	assert.Equal(t, params.Price, found.Price)
	assert.Equal(t, domaintrip.StatusActive, found.Status)
	assert.Nil(t, found.FinishedAt)
	assert.WithinDuration(t, startedAt, found.StartedAt, time.Microsecond)
	assert.False(t, found.CreatedAt.IsZero())
	assert.False(t, found.UpdatedAt.IsZero())

	secondEntity, err := domaintrip.NewTrip(params, startedAt)
	require.NoError(t, err)

	_, err = repository.Create(ctx, model.FromDomain(secondEntity))
	require.ErrorIs(t, err, domaintrip.ErrDriverBusy)

	_, err = repository.GetByID(ctx, secondEntity.ID)
	require.ErrorIs(t, err, domaintrip.ErrTripNotFound)
}

func newTestTrip(t *testing.T, pool *pgxpool.Pool) model.Trip {
	t.Helper()

	entity, err := domaintrip.NewTrip(domaintrip.CreateParams{
		UserID:         uuid.New(),
		DriverID:       uuid.New(),
		StartLatitude:  55.75,
		StartLongitude: 37.61,
		EndLatitude:    55.80,
		EndLongitude:   37.70,
		Price:          1500,
	}, time.Now().UTC().Truncate(time.Microsecond))
	require.NoError(t, err)

	t.Cleanup(func() {
		ctx, cancel := context.WithTimeout(
			context.Background(),
			5*time.Second,
		)
		defer cancel()

		_, err := pool.Exec(
			ctx,
			"DELETE FROM trips WHERE id = $1",
			entity.ID,
		)
		assert.NoError(t, err)
	})

	return model.FromDomain(entity)
}

func TestRepositoryFinish(t *testing.T) {
	pool, repository := newTestRepository(t)
	ctx := t.Context()

	_, err := repository.Finish(ctx, uuid.New())
	require.ErrorIs(t, err, domaintrip.ErrTripNotFound)

	input := newTestTrip(t, pool)

	_, err = repository.Create(ctx, input)
	require.NoError(t, err)

	finished, err := repository.Finish(ctx, input.ID)
	require.NoError(t, err)

	assert.Equal(t, input.ID, finished.ID)
	assert.Equal(t, domaintrip.StatusCompleted, finished.Status)
	require.NotNil(t, finished.FinishedAt)
	assert.False(t, finished.FinishedAt.IsZero())

	_, err = repository.Finish(ctx, input.ID)
	require.ErrorIs(t, err, domaintrip.ErrTripCompleted)

	found, err := repository.GetByID(ctx, input.ID)
	require.NoError(t, err)

	assert.Equal(t, domaintrip.StatusCompleted, found.Status)
	require.NotNil(t, found.FinishedAt)
	assert.True(t, finished.FinishedAt.Equal(*found.FinishedAt))
}

func TestRepositoryTransaction(t *testing.T) {
	rollbackCause := errors.New("force transaction rollback")

	tests := []struct {
		name        string
		cause       error
		wantHistory int
	}{
		{
			name:        "commit",
			wantHistory: 1,
		},
		{
			name:        "rollback",
			cause:       rollbackCause,
			wantHistory: 0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			pool, repository := newTestRepository(t)
			manager := transaction.NewTransactionManager(pool)
			ctx := t.Context()
			input := newTestTrip(t, pool)

			err := manager.Do(ctx, func(txCtx context.Context) error {
				if _, err := repository.Create(txCtx, input); err != nil {
					return err
				}

				err := repository.AddStatusHistory(txCtx, model.StatusHistory{
					TripID:   input.ID,
					ToStatus: domaintrip.StatusActive,
					Reason:   "trip created",
				})
				if err != nil {
					return err
				}

				return tt.cause
			})

			if tt.cause != nil {
				require.ErrorIs(t, err, tt.cause)

				_, err = repository.GetByID(ctx, input.ID)
				require.ErrorIs(t, err, domaintrip.ErrTripNotFound)
			} else {
				require.NoError(t, err)

				found, err := repository.GetByID(ctx, input.ID)
				require.NoError(t, err)
				assert.Equal(t, input.ID, found.ID)
				assert.Equal(t, input.Price, found.Price)
				assert.Equal(t, domaintrip.StatusActive, found.Status)
			}

			var historyCount int
			err = pool.QueryRow(
				ctx,
				"SELECT count(*) FROM trip_status_history WHERE trip_id = $1",
				input.ID,
			).Scan(&historyCount)
			require.NoError(t, err)

			assert.Equal(t, tt.wantHistory, historyCount)
		})
	}
}
