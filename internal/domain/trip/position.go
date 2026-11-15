package trip

import "time"

type PositionParams struct {
	Latitude   float64
	Longitude  float64
	RecordedAt time.Time
}

func (p PositionParams) Validate(now time.Time) error {
	if err := validateCoordinates("coordinates", p.Latitude, p.Longitude); err != nil {
		return err
	}
	if p.RecordedAt.After(now.Add(time.Minute)) {
		return &ValidationError{
			Field:   "recorded_at",
			Message: "must be no more than one minute in the future",
		}
	}
	return nil
}
