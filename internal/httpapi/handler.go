package httpapi

import (
	"time"

	api "github.com/deadOk-core/GoTemplate/internal/generated"
	"github.com/deadOk-core/GoTemplate/internal/trip"
	"github.com/jackc/pgx/v5/pgxpool"
)

type Handler struct {
	service      *trip.Service
	pool         *pgxpool.Pool
	queryTimeout time.Duration
}

func NewHandler(service *trip.Service, pool *pgxpool.Pool, queryTimeout time.Duration) *Handler {
	return &Handler{
		service:      service,
		pool:         pool,
		queryTimeout: queryTimeout,
	}
}

func toAPITrip(model trip.Trip) api.Trip {
	return api.Trip{
		DriverId: model.DriverID,

		// EndPoint Географические координаты
		EndPoint: api.Coordinates{
			Latitude:  model.EndLatitude,
			Longitude: model.EndLongitude,
		},
		FinishedAt:     model.FinishedAt,
		Id:             model.ID,
		LastPositionAt: nil,

		// Price Стоимость поездки в целых рублях. Это учебное упрощение: копейки не используются.
		Price: model.Price,

		// StartPoint Географические координаты
		StartPoint: api.Coordinates{
			Latitude:  model.StartLatitude,
			Longitude: model.StartLongitude,
		},
		StartedAt: model.StartedAt,
		Status:    api.TripStatus(model.Status),
		UserId:    model.UserID,
	}
}
