package finish_test

import (
	"context"
	"testing"
	"time"

	domaintrip "github.com/deadOk-core/GoTemplate/internal/domain/trip"
	"github.com/deadOk-core/GoTemplate/internal/model"
	finishusecase "github.com/deadOk-core/GoTemplate/internal/usecase/trip/finish"
	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"
)

func TestUseCaseFinish(t *testing.T) {
	tests := []struct {
		name    string
		status  domaintrip.Status
		wantErr error
	}{
		{
			name:   "active trip",
			status: domaintrip.StatusActive,
		},
		{
			name:    "completed trip",
			status:  domaintrip.StatusCompleted,
			wantErr: domaintrip.ErrTripCompleted,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctrl := gomock.NewController(t)
			repository := NewMocktripRepository(ctrl)
			manager := NewMocktxManager(ctrl)
			useCase := finishusecase.NewUseCase(repository, manager)

			ctx := t.Context()
			txCtx, cancel := context.WithCancel(ctx)
			defer cancel()

			input := model.Trip{
				ID:     uuid.New(),
				Status: tt.status,
			}

			// Для завершённой поездки задаём прежнее время завершения.
			if tt.status == domaintrip.StatusCompleted {
				at := time.Date(2026, 10, 24, 12, 0, 0, 0, time.UTC)
				input.FinishedAt = &at
			}

			var savedTrip model.Trip

			manager.EXPECT().
				Do(ctx, gomock.Any()).
				DoAndReturn(func(
					_ context.Context,
					fn func(context.Context) error,
				) error {
					return fn(txCtx)
				})

			getCall := repository.EXPECT().
				GetByID(txCtx, input.ID).
				Return(input, nil)

			if tt.wantErr == nil {
				fromStatus := domaintrip.StatusActive
				finishedAt := time.Date(2026, 10, 25, 13, 0, 0, 0, time.UTC)

				// Имитируем завершённую поездку, возвращённую репозиторием.
				savedTrip = input
				savedTrip.Status = domaintrip.StatusCompleted
				savedTrip.FinishedAt = &finishedAt
				savedTrip.UpdatedAt = finishedAt

				gomock.InOrder(
					getCall,
					repository.EXPECT().
						Finish(txCtx, input.ID).
						Return(savedTrip, nil),
					repository.EXPECT().
						AddStatusHistory(txCtx, model.StatusHistory{
							TripID:     input.ID,
							FromStatus: &fromStatus,
							ToStatus:   domaintrip.StatusCompleted,
							Reason:     "trip completed",
						}).
						Return(nil),
				)
			}

			result, err := useCase.Finish(ctx, input.ID)

			if tt.wantErr != nil {
				// Для завершённой поездки сохранение и история
				require.ErrorIs(t, err, tt.wantErr)
				assert.Equal(t, model.Trip{}, result)
				return
			}

			require.NoError(t, err)
			assert.Equal(t, savedTrip, result)
		})
	}
}
