package get_test

import (
	"errors"
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"

	domaintrip "github.com/deadOk-core/GoTemplate/internal/domain/trip"
	"github.com/deadOk-core/GoTemplate/internal/model"
	getusecase "github.com/deadOk-core/GoTemplate/internal/usecase/trip/get"
)

func TestUseCaseGetByID(t *testing.T) {
	tripID := uuid.New()
	repositoryErr := errors.New("repository unavailable")

	// Задаём данные и ошибку, которые вернёт мок репозитория.
	tests := []struct {
		name string
		trip model.Trip
		err  error
	}{
		{
			name: "found",
			trip: model.Trip{
				ID:       tripID,
				UserID:   uuid.New(),
				DriverID: uuid.New(),
				Price:    1500,
				Status:   domaintrip.StatusActive,
			},
		},
		{
			name: "not found",
			err:  domaintrip.ErrTripNotFound,
		},
		{
			name: "repository error",
			err:  repositoryErr,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctrl := gomock.NewController(t)
			repository := NewMocktripRepository(ctrl)
			useCase := getusecase.NewUseCase(repository)
			ctx := t.Context()

			// Проверяем, что usecase передаёт исходные контекст и ID.
			repository.EXPECT().
				GetByID(ctx, tripID).
				Return(tt.trip, tt.err)

			foundTrip, err := useCase.GetByID(ctx, tripID)

			if tt.err != nil {
				// Исходная ошибка должна сохраняться после оборачивания.
				require.ErrorIs(t, err, tt.err)
			} else {
				require.NoError(t, err)
			}

			// При успехе получаем данные репозитория,
			// при ошибке — пустую model.Trip.
			assert.Equal(t, tt.trip, foundTrip)
		})
	}
}
