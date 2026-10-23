package trip_test

import (
	"testing"

	domaintrip "github.com/deadOk-core/GoTemplate/internal/domain/trip"
	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestCreateParamsValidate(t *testing.T) {
	userID := uuid.New()
	driverID := uuid.New()

	tests := []struct {
		name      string
		params    domaintrip.CreateParams
		wantField string
	}{
		{
			name: "valid trip",
			params: domaintrip.CreateParams{
				UserID: userID, DriverID: driverID, Price: 1450,
			},
		},
		{
			name: "zero price",
			params: domaintrip.CreateParams{
				UserID: userID, DriverID: driverID,
			},
		},
		{
			name: "coordinate boundaries",
			params: domaintrip.CreateParams{
				UserID: userID, DriverID: driverID,
				StartLatitude: -90, StartLongitude: -180,
				EndLatitude: 90, EndLongitude: 180,
			},
		},
		{
			name: "empty user",
			params: domaintrip.CreateParams{
				DriverID: driverID,
			},
			wantField: "user_id",
		},
		{
			name: "empty driver",
			params: domaintrip.CreateParams{
				UserID: userID,
			},
			wantField: "driver_id",
		},
		{
			name: "negative price",
			params: domaintrip.CreateParams{
				UserID: userID, DriverID: driverID, Price: -1,
			},
			wantField: "price",
		},
		{
			name: "invalid start latitude",
			params: domaintrip.CreateParams{
				UserID: userID, DriverID: driverID, StartLatitude: 91,
			},
			wantField: "start_point.latitude",
		},
		{
			name: "invalid start longitude",
			params: domaintrip.CreateParams{
				UserID: userID, DriverID: driverID, StartLongitude: -181,
			},
			wantField: "start_point.longitude",
		},
		{
			name: "invalid end latitude",
			params: domaintrip.CreateParams{
				UserID: userID, DriverID: driverID, EndLatitude: -91,
			},
			wantField: "end_point.latitude",
		},
		{
			name: "invalid end longitude",
			params: domaintrip.CreateParams{
				UserID: userID, DriverID: driverID, EndLongitude: 181,
			},
			wantField: "end_point.longitude",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.params.Validate()

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
