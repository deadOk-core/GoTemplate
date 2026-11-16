//go:generate go tool mockgen -source=contract.go -destination=mocks_test.go -package=createposition_test

package createposition

import (
	"context"
	"time"

	"github.com/google/uuid"

	"github.com/deadOk-core/GoTemplate/internal/model"
)

type tripRepository interface {
	UpdateLastPosition(ctx context.Context, tripID uuid.UUID, recordedAt time.Time) error
	InsertPosition(ctx context.Context, position model.TripPosition) (model.TripPosition, error)
}

type txManager interface {
	Do(ctx context.Context, fn func(ctx context.Context) error) error
}

type positionRequestCanceller interface {
	Cancel(ctx context.Context, tripID uuid.UUID)
}
