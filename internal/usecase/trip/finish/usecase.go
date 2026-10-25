package finish

import (
	"context"
	"fmt"
	"log/slog"
	"time"

	"github.com/google/uuid"

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

func (u *UseCase) Finish(ctx context.Context, id uuid.UUID) (model.Trip, error) {
	var finishTrip model.Trip

	err := u.txManager.Do(ctx, func(txCtx context.Context) error {
		foundTrip, err := u.repository.GetByID(txCtx, id)
		if err != nil {
			return err
		}

		entity := foundTrip.ToDomain()
		fromStatus := entity.Status

		if err := entity.Finish(time.Now().UTC()); err != nil {
			return err
		}

		finishTrip, err = u.repository.Finish(txCtx, id)
		if err != nil {
			return err
		}

		err = u.repository.AddStatusHistory(txCtx, model.StatusHistory{
			TripID:     finishTrip.ID,
			FromStatus: &fromStatus,
			ToStatus:   finishTrip.Status,
			Reason:     "trip completed",
		})
		if err != nil {
			return err
		}

		return nil
	})
	if err != nil {
		return model.Trip{}, fmt.Errorf("finish trip: %w", err)
	}
	u.logger.InfoContext(
		ctx,
		"trip completed",
		"trip_id", finishTrip.ID.String(),
	)

	return finishTrip, nil
}
