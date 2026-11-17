//go:generate go tool mockgen -source=contract.go -destination=mocks_test.go -package=listpositions_test

package listpositions

import (
	"context"

	"github.com/google/uuid"

	"github.com/deadOk-core/GoTemplate/internal/model"
)

type tripRepository interface {
	GetByID(ctx context.Context, tripID uuid.UUID) (model.Trip, error)
	ListPositions(ctx context.Context, tripID uuid.UUID) ([]model.TripPosition, error)
}
