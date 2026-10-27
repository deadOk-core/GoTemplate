package create

import (
	"bytes"
	"encoding/json"

	domaintrip "github.com/deadOk-core/GoTemplate/internal/domain/trip"
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

func fromRequest(body []byte) (domaintrip.CreateParams, error) {
	invalidRequest := &domaintrip.ValidationError{
		Field:   "body",
		Message: "invalid request",
	}

	// Проверяем наличие обязательных полей до превращения чисел в нули.
	if !hasRequiredTripFields(body) {
		return domaintrip.CreateParams{}, invalidRequest
	}

	var request api.CreateTripJSONRequestBody

	decoder := json.NewDecoder(bytes.NewReader(body))
	decoder.DisallowUnknownFields()

	if err := decoder.Decode(&request); err != nil {
		return domaintrip.CreateParams{}, invalidRequest
	}

	return domaintrip.CreateParams{
		UserID:         request.UserId,
		DriverID:       request.DriverId,
		StartLatitude:  request.StartPoint.Latitude,
		StartLongitude: request.StartPoint.Longitude,
		EndLatitude:    request.EndPoint.Latitude,
		EndLongitude:   request.EndPoint.Longitude,
		Price:          request.Price,
	}, nil
}

func hasRequiredTripFields(body []byte) bool {
	var fields map[string]*json.RawMessage

	if err := json.Unmarshal(body, &fields); err != nil {
		return false
	}

	requiredFields := []string{
		"user_id",
		"driver_id",
		"start_point",
		"end_point",
		"price",
	}

	for _, name := range requiredFields {
		if fields[name] == nil {
			return false
		}
	}

	return hasRequiredCoordinateFields(*fields["start_point"]) &&
		hasRequiredCoordinateFields(*fields["end_point"])
}

func hasRequiredCoordinateFields(raw json.RawMessage) bool {
	var fields map[string]*json.RawMessage

	if err := json.Unmarshal(raw, &fields); err != nil {
		return false
	}

	return fields["latitude"] != nil &&
		fields["longitude"] != nil
}
