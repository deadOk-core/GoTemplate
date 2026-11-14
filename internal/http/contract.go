//go:generate go tool mockgen -source=contract.go -destination=mocks_test.go -package=http_test

package http

import (
	"context"
	"net/http"

	api "github.com/deadOk-core/GoTemplate/internal/generated"
)

type tripCreator interface {
	CreateTrip(w http.ResponseWriter, r *http.Request, params api.CreateTripParams)
}

type tripGetter interface {
	GetTrip(w http.ResponseWriter, r *http.Request, tripID api.TripId)
}

type tripFinisher interface {
	FinishTrip(w http.ResponseWriter, r *http.Request, tripID api.TripId)
}

type databasePinger interface {
	Ping(ctx context.Context) error
}
