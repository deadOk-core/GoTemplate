package createposition_test

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/otel/metric"
	"go.uber.org/mock/gomock"

	domaintrip "github.com/deadOk-core/GoTemplate/internal/domain/trip"
	"github.com/deadOk-core/GoTemplate/internal/model"
	"github.com/deadOk-core/GoTemplate/internal/usecase/trip/createposition"
)

func TestCreatePositionTransaction(t *testing.T) {
	insertFailure := errors.New("insert failed")
	commitFailure := errors.New("commit failed")
	tests := []struct {
		name      string
		updateErr error
		insertErr error
		commitErr error
		wantErr   error
	}{
		{name: "committed"},
		{name: "missing trip", updateErr: domaintrip.ErrTripNotFound, wantErr: domaintrip.ErrTripNotFound},
		{name: "completed trip", updateErr: domaintrip.ErrTripCompleted, wantErr: domaintrip.ErrTripCompleted},
		{name: "insert failed", insertErr: insertFailure, wantErr: insertFailure},
		{name: "commit failed", commitErr: commitFailure, wantErr: commitFailure},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctrl := gomock.NewController(t)
			repo := NewMocktripRepository(ctrl)
			manager := NewMocktxManager(ctrl)
			canceller := NewMockpositionRequestCanceller(ctrl)
			counter := NewMockpositionCounter(ctrl)
			useCase := createposition.NewUseCase(repo, manager, canceller, counter)
			ctx := t.Context()
			txCtx, cancel := context.WithCancel(ctx)
			defer cancel()

			input := model.TripPosition{TripID: uuid.New(), PositionParams: domaintrip.PositionParams{RecordedAt: time.Now()}}
			stored := input
			stored.ID = 42
			committed := false
			manager.EXPECT().Do(ctx, gomock.Any()).DoAndReturn(func(_ context.Context, fn func(context.Context) error) error {
				if err := fn(txCtx); err != nil {
					return err
				}
				committed = tt.commitErr == nil
				return tt.commitErr
			})
			update := repo.EXPECT().UpdateLastPosition(txCtx, input.TripID, input.RecordedAt).Return(tt.updateErr)
			if tt.updateErr == nil {
				repo.EXPECT().InsertPosition(txCtx, input).Return(stored, tt.insertErr).After(update)
			}
			if tt.wantErr == nil {
				canceller.EXPECT().Cancel(ctx, input.TripID).
					Do(func(context.Context, uuid.UUID) { assert.True(t, committed) })
				counter.EXPECT().Add(ctx, int64(1)).
					Do(func(context.Context, int64, ...metric.AddOption) { assert.True(t, committed) })
			}

			got, err := useCase.Create(ctx, input.TripID, input.PositionParams)
			if tt.wantErr != nil {
				require.ErrorIs(t, err, tt.wantErr)
				assert.Zero(t, got)
				return
			}
			require.NoError(t, err)
			assert.Equal(t, stored, got)
		})
	}
}

func TestCreatePositionRejectsInvalidInputBeforeTransaction(t *testing.T) {
	ctrl := gomock.NewController(t)
	useCase := createposition.NewUseCase(NewMocktripRepository(ctrl), NewMocktxManager(ctrl),
		NewMockpositionRequestCanceller(ctrl), NewMockpositionCounter(ctrl))
	params := domaintrip.PositionParams{RecordedAt: time.Now().Add(2 * time.Minute)}
	_, err := useCase.Create(t.Context(), uuid.New(), params)
	var validationErr *domaintrip.ValidationError
	require.ErrorAs(t, err, &validationErr)
	assert.Equal(t, "recorded_at", validationErr.Field)
}
