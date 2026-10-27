package finish_test

import (
	"encoding/json"
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
	finishhandler "github.com/deadOk-core/GoTemplate/internal/http/trip/finish"
	"github.com/deadOk-core/GoTemplate/internal/model"
)

func TestHandlerFinishTrip(t *testing.T) {
	tripID := uuid.New()
	startedAt := time.Date(2026, 10, 24, 12, 0, 0, 0, time.UTC)
	finishedAt := startedAt.Add(time.Hour)

	input := model.Trip{
		ID:         tripID,
		UserID:     uuid.New(),
		DriverID:   uuid.New(),
		Price:      1500,
		Status:     domaintrip.StatusCompleted,
		StartedAt:  startedAt,
		FinishedAt: &finishedAt,
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
			name:       "already completed",
			err:        domaintrip.ErrTripCompleted,
			wantStatus: http.StatusConflict,
			wantCode:   "trip_completed",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctrl := gomock.NewController(t)
			useCase := NewMocktripUseCase(ctrl)
			handler := finishhandler.NewHandler(useCase, slog.New(slog.DiscardHandler))

			path := "/api/v1/trips/" + tripID.String() + "/finish"
			request := httptest.NewRequest(http.MethodPost, path, nil).
				WithContext(t.Context())
			recorder := httptest.NewRecorder()

			// Проверяем передачу исходных контекста и ID.
			useCase.EXPECT().
				Finish(request.Context(), tripID).
				Return(tt.trip, tt.err)

			handler.FinishTrip(recorder, request, tripID)

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
				assert.Equal(
					t,
					api.TripStatus(domaintrip.StatusCompleted),
					body.Status,
				)
				assert.True(t, startedAt.Equal(body.StartedAt))
				require.NotNil(t, body.FinishedAt)
				assert.True(t, finishedAt.Equal(*body.FinishedAt))
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
		})
	}
}
