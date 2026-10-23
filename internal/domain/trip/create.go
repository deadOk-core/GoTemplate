package trip

import (
	"time"

	"github.com/google/uuid"
)

type CreateParams struct {
	UserID   uuid.UUID
	DriverID uuid.UUID

	StartLatitude  float64
	StartLongitude float64
	EndLatitude    float64
	EndLongitude   float64

	Price int64
}

func NewTrip(params CreateParams, at time.Time) (Trip, error) {
	if err := params.Validate(); err != nil {
		return Trip{}, err
	}

	return Trip{
		ID:       uuid.New(),
		UserID:   params.UserID,
		DriverID: params.DriverID,

		StartLatitude:  params.StartLatitude,
		StartLongitude: params.StartLongitude,
		EndLatitude:    params.EndLatitude,
		EndLongitude:   params.EndLongitude,

		Price:     params.Price,
		Status:    StatusActive,
		StartedAt: at.UTC(),
	}, nil
}

func (p *CreateParams) Validate() error {
	if p.UserID == uuid.Nil {
		return &ValidationError{
			Field:   "user_id",
			Message: "must not be empty",
		}
	}

	if p.DriverID == uuid.Nil {
		return &ValidationError{
			Field:   "driver_id",
			Message: "must not be empty",
		}
	}

	if p.Price < 0 {
		return &ValidationError{
			Field:   "price",
			Message: "must not be negative",
		}
	}

	if err := validateCoordinates("start_point", p.StartLatitude, p.StartLongitude); err != nil {
		return err
	}

	if err := validateCoordinates("end_point", p.EndLatitude, p.EndLongitude); err != nil {
		return err
	}

	return nil

}

func validateCoordinates(field string, latitude, longitude float64) error {
	if !(latitude >= -90 && latitude <= 90) {
		return &ValidationError{
			Field:   field + ".latitude",
			Message: "must be between -90 and 90",
		}
	}

	if !(longitude >= -180 && longitude <= 180) {
		return &ValidationError{
			Field:   field + ".longitude",
			Message: "must be between -180 and 180",
		}
	}

	return nil
}
