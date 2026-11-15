package trip

import (
	api "github.com/deadOk-core/GoTemplate/internal/generated"
	"github.com/deadOk-core/GoTemplate/internal/model"
)

func PositionResponse(position model.TripPosition) api.TripPosition {
	return api.TripPosition{
		Id:     position.ID,
		TripId: position.TripID,
		Coordinates: api.Coordinates{
			Latitude:  position.Latitude,
			Longitude: position.Longitude,
		},
		RecordedAt: position.RecordedAt,
	}
}
