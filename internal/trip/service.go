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
	repository tripRepository
	txManager  TxManager
}

func NewService(
	repository tripRepository,
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

func (s *Service) GetByID(ctx context.Context, id uuid.UUID) (Trip, error) {
	foundTrip, err := s.repository.GetByID(ctx, id)

	if err != nil {
		return Trip{}, fmt.Errorf("get trip: %w", err)
	}

	return foundTrip, nil
}

func (s *Service) Finish(ctx context.Context, id uuid.UUID) (Trip, error) {
	var finishedTrip Trip
	err := s.txManager.Do(ctx, func(ctx context.Context) error {
		var err error
		finishedTrip, err = s.repository.Finish(ctx, id)
		if err != nil {
			return err
		}

		fromStatus := StatusActive

		history := StatusHistory{
			TripID:     finishedTrip.ID,
			FromStatus: &fromStatus,
			ToStatus:   StatusCompleted,
			Reason:     "trip completed",
		}
		return s.repository.AddStatusHistory(ctx, history)
	})

	if err != nil {
		return Trip{}, fmt.Errorf("finish trip: %w", err)
	}

	return finishedTrip, nil
}
