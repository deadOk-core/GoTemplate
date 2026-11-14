//go:generate go tool mockgen -source=contract.go -destination=mocks_test.go -package=telemetry_test

package telemetry

import "context"

type activeTripCounter interface {
	CountActive(ctx context.Context) (int64, error)
}
