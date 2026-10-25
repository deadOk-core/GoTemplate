package create

import (
	"context"
	"fmt"
	"log/slog"
	"time"

	domaintrip "github.com/deadOk-core/GoTemplate/internal/domain/trip"
	"github.com/deadOk-core/GoTemplate/internal/model"
)

type UseCase struct {
	repository tripRepository
	txManager  txManager
	logger     *slog.Logger
}

func NewUseCase(
	repository tripRepository,
	txManager txManager,
	logger *slog.Logger,
) *UseCase {
	return &UseCase{
		repository: repository,
		txManager:  txManager,
		logger:     logger,
	}
}

func (u *UseCase) Create(ctx context.Context, params domaintrip.CreateParams) (model.Trip, error) {
	entity, err := domaintrip.NewTrip(params, time.Now())
	if err != nil {
		return model.Trip{}, err
	}

	trip := model.FromDomain(entity)

	var createdTrip model.Trip

	err = u.txManager.Do(ctx, func(txCtx context.Context) error {
		createdTrip, err = u.repository.Create(txCtx, trip)
		if err != nil {
			return err
		}

		err := u.repository.AddStatusHistory(txCtx, model.StatusHistory{
			TripID:   createdTrip.ID,
			ToStatus: domaintrip.StatusActive,
			Reason:   "trip created",
		})
		if err != nil {
			return err
		}

		return nil
	})
	if err != nil {
		return model.Trip{}, fmt.Errorf("create trip: %w", err)
	}
	u.logger.InfoContext(
		ctx,
		"trip created",
		"trip_id", createdTrip.ID.String(),
	)

	return createdTrip, nil
}
