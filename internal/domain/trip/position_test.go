package trip_test

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	domaintrip "github.com/deadOk-core/GoTemplate/internal/domain/trip"
)

func TestPositionValidation(t *testing.T) {
	now := time.Date(2026, time.August, 24, 12, 0, 0, 0, time.UTC)
	tests := []struct {
		name      string
		latitude  float64
		longitude float64
		offset    time.Duration
		wantField string
	}{
		{name: "zero coordinates", offset: -time.Hour},
		{name: "inclusive limits", latitude: -90, longitude: 180, offset: time.Minute},
		{name: "invalid latitude", latitude: 90.01, wantField: "coordinates.latitude"},
		{name: "invalid longitude", longitude: -180.01, wantField: "coordinates.longitude"},
		{name: "too far in the future", offset: time.Minute + time.Second, wantField: "recorded_at"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			params := domaintrip.PositionParams{
				Latitude:   tt.latitude,
				Longitude:  tt.longitude,
				RecordedAt: now.Add(tt.offset),
			}
			err := params.Validate(now)
			if tt.wantField == "" {
				require.NoError(t, err)
				return
			}
			var validationErr *domaintrip.ValidationError
			require.ErrorAs(t, err, &validationErr)
			assert.Equal(t, tt.wantField, validationErr.Field)
		})
	}
}
