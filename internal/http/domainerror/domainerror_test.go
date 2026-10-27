package domainerror_test

import (
	"errors"
	"fmt"
	"net/http"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	domaintrip "github.com/deadOk-core/GoTemplate/internal/domain/trip"
	"github.com/deadOk-core/GoTemplate/internal/http/domainerror"
)

func TestProblem(t *testing.T) {
	tests := []struct {
		name       string
		err        error
		wantStatus int
		wantCode   string
		wantDetail string
	}{
		{
			name:       "trip not found",
			err:        fmt.Errorf("get trip: %w", domaintrip.ErrTripNotFound),
			wantStatus: http.StatusNotFound,
			wantCode:   "trip_not_found",
			wantDetail: "Trip was not found",
		},
		{
			name:       "trip completed",
			err:        domaintrip.ErrTripCompleted,
			wantStatus: http.StatusConflict,
			wantCode:   "trip_completed",
			wantDetail: "Operation is not allowed for a completed trip",
		},
		{
			name:       "driver busy",
			err:        domaintrip.ErrDriverBusy,
			wantStatus: http.StatusConflict,
			wantCode:   "driver_busy",
			wantDetail: "Driver already has an active trip",
		},
		{
			name: "validation error",
			err: fmt.Errorf("create trip: %w", &domaintrip.ValidationError{
				Field:   "price",
				Message: "must not be negative",
			}),
			wantStatus: http.StatusBadRequest,
			wantCode:   "invalid_request",
			wantDetail: "Request validation failed",
		},
		{
			name:       "internal error",
			err:        errors.New("database unavailable: internal diagnostic"),
			wantStatus: http.StatusInternalServerError,
			wantCode:   "internal_error",
			wantDetail: "Internal server error",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			status, body := domainerror.Problem(tt.err)

			assert.Equal(t, tt.wantStatus, status)
			assert.Equal(t, tt.wantStatus, int(body.Status))
			assert.Equal(t, tt.wantCode, body.Code)

			require.NotNil(t, body.Detail)
			assert.Equal(t, tt.wantDetail, *body.Detail)

			assert.NotEmpty(t, body.Type)
			assert.NotEmpty(t, body.Title)
			assert.Nil(t, body.Instance)
		})
	}
}
