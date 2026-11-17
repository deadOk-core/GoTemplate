package listpositions

import (
	"context"
	"fmt"

	"github.com/google/uuid"

	"github.com/deadOk-core/GoTemplate/internal/model"
)

type UseCase struct {
	repository tripRepository
}

func NewUseCase(repository tripRepository) *UseCase {
	return &UseCase{repository: repository}
}

func (u *UseCase) List(ctx context.Context, tripID uuid.UUID) ([]model.TripPosition, error) {
	if _, err := u.repository.GetByID(ctx, tripID); err != nil {
		return nil, fmt.Errorf("find trip for route: %w", err)
	}
	positions, err := u.repository.ListPositions(ctx, tripID)
	if err != nil {
		return nil, fmt.Errorf("list trip positions: %w", err)
	}
	return positions, nil
}
