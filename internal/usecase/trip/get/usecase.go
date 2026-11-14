package get

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
	return &UseCase{
		repository: repository,
	}
}

func (u *UseCase) GetByID(ctx context.Context, id uuid.UUID) (model.Trip, error) {
	foundTrip, err := u.repository.GetByID(ctx, id)
	if err != nil {
		return model.Trip{}, fmt.Errorf("get trip: %w", err)
	}

	return foundTrip, nil
}
