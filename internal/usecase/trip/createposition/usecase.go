package createposition

import (
	"context"
	"fmt"
	"time"

	"github.com/google/uuid"

	domaintrip "github.com/deadOk-core/GoTemplate/internal/domain/trip"
	"github.com/deadOk-core/GoTemplate/internal/model"
)

type UseCase struct {
	repository tripRepository
	txManager  txManager
	canceller  positionRequestCanceller
	saved      positionCounter
}

func NewUseCase(repository tripRepository, txManager txManager, canceller positionRequestCanceller, saved positionCounter) *UseCase {
	return &UseCase{repository: repository, txManager: txManager, canceller: canceller, saved: saved}
}

func (u *UseCase) Create(ctx context.Context, tripID uuid.UUID, params domaintrip.PositionParams) (model.TripPosition, error) {
	if err := params.Validate(time.Now()); err != nil {
		return model.TripPosition{}, err
	}

	var saved model.TripPosition
	err := u.txManager.Do(ctx, func(txCtx context.Context) error {
		if err := u.repository.UpdateLastPosition(txCtx, tripID, params.RecordedAt); err != nil {
			return err
		}
		var err error
		saved, err = u.repository.InsertPosition(txCtx, model.TripPosition{TripID: tripID, PositionParams: params})
		return err
	})
	if err != nil {
		return model.TripPosition{}, fmt.Errorf("save trip position: %w", err)
	}
	u.canceller.Cancel(ctx, tripID)
	u.saved.Add(ctx, 1)
	return saved, nil
}
