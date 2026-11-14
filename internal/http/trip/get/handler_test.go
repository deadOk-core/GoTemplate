package get_test

import (
	"encoding/json"
	"errors"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"

	domaintrip "github.com/deadOk-core/GoTemplate/internal/domain/trip"
	api "github.com/deadOk-core/GoTemplate/internal/generated"
	gethandler "github.com/deadOk-core/GoTemplate/internal/http/trip/get"
	"github.com/deadOk-core/GoTemplate/internal/model"
)

func TestHandlerGetTrip(t *testing.T) {
	tripID := uuid.New()
	input := model.Trip{
		ID:             tripID,
		UserID:         uuid.New(),
		DriverID:       uuid.New(),
		StartLatitude:  55.75,
		StartLongitude: 37.61,
		EndLatitude:    55.80,
		EndLongitude:   37.70,
		Price:          1500,
		Status:         domaintrip.StatusActive,
		StartedAt:      time.Date(2026, 10, 24, 12, 0, 0, 0, time.UTC),
	}

	tests := []struct {
		name       string
		trip       model.Trip
		err        error
		wantStatus int
		wantCode   string
	}{
		{
			name:       "success",
			trip:       input,
			wantStatus: http.StatusOK,
		},
		{
			name:       "not found",
			err:        domaintrip.ErrTripNotFound,
			wantStatus: http.StatusNotFound,
			wantCode:   "trip_not_found",
		},
		{
			name:       "internal error",
			err:        errors.New("database unavailable: private diagnostic"),
			wantStatus: http.StatusInternalServerError,
			wantCode:   "internal_error",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctrl := gomock.NewController(t)
			useCase := NewMocktripUseCase(ctrl)
			handler := gethandler.NewHandler(useCase, slog.New(slog.DiscardHandler))

			path := "/api/v1/trips/" + tripID.String()
			request := httptest.NewRequest(http.MethodGet, path, nil).
				WithContext(t.Context())
			recorder := httptest.NewRecorder()

			// Хендлер должен передать контекст запроса и исходный ID.
			useCase.EXPECT().
				GetByID(request.Context(), tripID).
				Return(tt.trip, tt.err)

			// UUID передаём отдельно, как это делает OpenAPI-роутер.
			handler.GetTrip(recorder, request, tripID)

			response := recorder.Result()
			defer response.Body.Close()

			require.Equal(t, tt.wantStatus, response.StatusCode)

			if tt.wantStatus == http.StatusOK {
				assert.Equal(
					t,
					"application/json",
					response.Header.Get("Content-Type"),
				)

				var body api.Trip
				require.NoError(t, json.NewDecoder(response.Body).Decode(&body))

				assert.Equal(t, input.ID, body.Id)
				assert.Equal(t, input.UserID, body.UserId)
				assert.Equal(t, input.DriverID, body.DriverId)
				assert.Equal(t, input.Price, body.Price)
				assert.Equal(t, api.TripStatus(input.Status), body.Status)
				assert.True(t, input.StartedAt.Equal(body.StartedAt))
				assert.Equal(t, input.StartLatitude, body.StartPoint.Latitude)
				assert.Equal(t, input.StartLongitude, body.StartPoint.Longitude)
				assert.Equal(t, input.EndLatitude, body.EndPoint.Latitude)
				assert.Equal(t, input.EndLongitude, body.EndPoint.Longitude)
				assert.Nil(t, body.FinishedAt)
				assert.Nil(t, body.LastPositionAt)
				return
			}

			assert.Equal(
				t,
				"application/problem+json",
				response.Header.Get("Content-Type"),
			)

			var body api.Problem
			require.NoError(t, json.NewDecoder(response.Body).Decode(&body))

			assert.Equal(t, tt.wantStatus, int(body.Status))
			assert.Equal(t, tt.wantCode, body.Code)
			require.NotNil(t, body.Instance)
			assert.Equal(t, path, *body.Instance)

			if tt.wantStatus == http.StatusInternalServerError {
				// В ответе нет исходного диагностического сообщения.
				require.NotNil(t, body.Detail)
				assert.Equal(t, "Internal server error", *body.Detail)
				assert.NotContains(t, recorder.Body.String(), tt.err.Error())
			}
		})
	}
}
