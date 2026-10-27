//go:generate go tool mockgen -source=contract.go -destination=mocks_test.go -package=finish_test

package finish

import (
	"context"

	"github.com/google/uuid"
	"go.opentelemetry.io/otel/metric"

	"github.com/deadOk-core/GoTemplate/internal/model"
)

type tripRepository interface {
	GetByID(ctx context.Context, id uuid.UUID) (model.Trip, error)
	Finish(ctx context.Context, id uuid.UUID) (model.Trip, error)
	AddStatusHistory(ctx context.Context, history model.StatusHistory) error
}

type txManager interface {
	Do(ctx context.Context, fn func(context.Context) error) error
}

type tripCounter interface {
	Add(ctx context.Context, value int64, options ...metric.AddOption)
}
