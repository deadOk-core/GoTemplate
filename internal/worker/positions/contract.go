//go:generate go tool mockgen -source=contract.go -destination=mocks_test.go -package=positions_test

package positions

import (
	"context"

	"github.com/google/uuid"
)

type pushGateway interface {
	RequestPosition(ctx context.Context, driverID, tripID uuid.UUID) error
}
