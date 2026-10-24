package trip

import (
	"context"

	"github.com/deadOk-core/GoTemplate/internal/model"
	"github.com/google/uuid"
)

type tripRepository interface {
	Create(ctx context.Context, newTrip model.Trip) (model.Trip, error)
	GetByID(ctx context.Context, id uuid.UUID) (model.Trip, error)
	Finish(ctx context.Context, id uuid.UUID) (model.Trip, error)
	AddStatusHistory(ctx context.Context, history model.StatusHistory) error
}
