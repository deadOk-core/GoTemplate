//go:generate go tool mockgen -source=contract.go -destination=mocks_test.go -package=create_test

package create

import (
	"context"

	"github.com/deadOk-core/GoTemplate/internal/model"
)

type tripRepository interface {
	Create(ctx context.Context, newTrip model.Trip) (model.Trip, error)
	AddStatusHistory(ctx context.Context, history model.StatusHistory) error
}

type txManager interface {
	Do(ctx context.Context, fn func(ctx context.Context) error) error
}
