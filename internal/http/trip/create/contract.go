//go:generate go tool mockgen -source=contract.go -destination=mocks_test.go -package=create_test

package create

import (
	"context"

	domaintrip "github.com/deadOk-core/GoTemplate/internal/domain/trip"
	"github.com/deadOk-core/GoTemplate/internal/model"
)

type tripUseCase interface {
	Create(ctx context.Context, params domaintrip.CreateParams) (model.Trip, error)
}
