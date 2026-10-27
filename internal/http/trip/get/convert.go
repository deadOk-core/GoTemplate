package get

import (
	api "github.com/deadOk-core/GoTemplate/internal/generated"
	"github.com/deadOk-core/GoTemplate/internal/model"
)

func toAPITrip(input model.Trip) api.Trip {
	return api.Trip{
		Id:       input.ID,
		UserId:   input.UserID,
		DriverId: input.DriverID,
		StartPoint: api.Coordinates{
			Latitude:  input.StartLatitude,
			Longitude: input.StartLongitude,
		},
		EndPoint: api.Coordinates{
			Latitude:  input.EndLatitude,
			Longitude: input.EndLongitude,
		},
		Price:      input.Price,
		Status:     api.TripStatus(input.Status),
		StartedAt:  input.StartedAt,
		FinishedAt: input.FinishedAt,
	}
}
