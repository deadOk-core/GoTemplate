//go:generate go tool mockgen -source=contract.go -destination=mocks_test.go -package=positions_test

package positions

import (
	"context"
	"time"

	"github.com/google/uuid"

	"github.com/deadOk-core/GoTemplate/internal/model"
)

type pushGateway interface {
	RequestPosition(ctx context.Context, driverID, tripID uuid.UUID) error
}

type staleTripRepository interface {
	ListStaleTrips(ctx context.Context, staleAfter time.Duration) ([]model.StaleTrip, error)
}

type ticker interface {
	Ticks() <-chan time.Time
	Stop()
}
