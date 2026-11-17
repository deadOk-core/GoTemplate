//go:generate go tool mockgen -source=contract.go -destination=mocks_test.go -package=createposition_test

package createposition

import (
	"context"

	"github.com/google/uuid"

	domaintrip "github.com/deadOk-core/GoTemplate/internal/domain/trip"
	"github.com/deadOk-core/GoTemplate/internal/model"
)

type positionUseCase interface {
	Create(ctx context.Context, tripID uuid.UUID, params domaintrip.PositionParams) (model.TripPosition, error)
}
