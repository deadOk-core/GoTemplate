//go:generate go tool mockgen -source=contract.go -destination=mocks_test.go -package=finish_test

package finish

import (
	"context"

	"github.com/deadOk-core/GoTemplate/internal/model"
	"github.com/google/uuid"
)

type tripUseCase interface {
	Finish(ctx context.Context, id uuid.UUID) (model.Trip, error)
}
