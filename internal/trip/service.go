package trip

import (
	"context"
	"fmt"
	"time"

	"github.com/google/uuid"
)

type TxManager interface {
	Do(ctx context.Context, fn func(ctx context.Context) error) error
}

type Service struct {
	repository *Repository
	txManager  TxManager
}

func NewService(
	repository *Repository,
	txManager TxManager,
) *Service {
	return &Service{
		repository: repository,
		txManager:  txManager,
	}
}

func (s *Service) Create(ctx context.Context, params CreateParams) (Trip, error) {
	// Служебные поля определяет сервис, а не HTTP-клиент.
	newTrip := Trip{
		ID:       uuid.New(),
		UserID:   params.UserID,
		DriverID: params.DriverID,

		StartLatitude:  params.StartLatitude,
		StartLongitude: params.StartLongitude,
		EndLatitude:    params.EndLatitude,
		EndLongitude:   params.EndLongitude,

		Price:  params.Price,
		Status: StatusActive,

		StartedAt:  time.Now().UTC(),
		FinishedAt: nil,
	}

	var createdTrip Trip

	// Обе вставки должны либо сохраниться вместе, либо вместе откатиться.
	err := s.txManager.Do(ctx, func(ctx context.Context) error {
		var err error

		createdTrip, err = s.repository.Create(ctx, newTrip)
		if err != nil {
			return err
		}

		history := StatusHistory{
			TripID:     createdTrip.ID,
			FromStatus: nil,
			ToStatus:   StatusActive,
			Reason:     "trip created",
		}

		return s.repository.AddStatusHistory(ctx, history)
	})

	if err != nil {
		return Trip{}, fmt.Errorf("create trip: %w", err)
	}

	return createdTrip, nil
}
