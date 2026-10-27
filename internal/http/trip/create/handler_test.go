package create_test

import (
	"encoding/json"
	"fmt"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"

	domaintrip "github.com/deadOk-core/GoTemplate/internal/domain/trip"
	api "github.com/deadOk-core/GoTemplate/internal/generated"
	createhandler "github.com/deadOk-core/GoTemplate/internal/http/trip/create"
	"github.com/deadOk-core/GoTemplate/internal/model"
)

const invalidRequestCode = "invalid_request"

func TestHandlerCreateTrip(t *testing.T) {
	userID := uuid.New()
	driverID := uuid.New()

	validBody := fmt.Sprintf(`{
		"price":1500,
		"user_id":"%s",
		"driver_id":"%s",
		"start_point":{"latitude":55.75,"longitude":37.61},
		"end_point":{"latitude":55.80,"longitude":37.70}
	}`, userID.String(), driverID.String())

	tests := []struct {
		name        string
		body        string
		price       int64
		useCaseErr  error
		callUseCase bool
		wantStatus  int
		wantCode    string
	}{
		{
			name:        "success",
			body:        validBody,
			price:       1500,
			callUseCase: true,
			wantStatus:  http.StatusCreated,
		},
		{
			name:        "zero price",
			body:        strings.Replace(validBody, `"price":1500`, `"price":0`, 1),
			callUseCase: true,
			wantStatus:  http.StatusCreated,
		},
		{
			name:        "driver busy",
			body:        validBody,
			price:       1500,
			useCaseErr:  domaintrip.ErrDriverBusy,
			callUseCase: true,
			wantStatus:  http.StatusConflict,
			wantCode:    "driver_busy",
		},
		{
			name:       "invalid JSON",
			body:       "{",
			wantStatus: http.StatusBadRequest,
			wantCode:   invalidRequestCode,
		},
		{
			name:       "missing price",
			body:       strings.Replace(validBody, `"price":1500,`, "", 1),
			wantStatus: http.StatusBadRequest,
			wantCode:   invalidRequestCode,
		},
		{
			name:       "null price",
			body:       strings.Replace(validBody, `"price":1500`, `"price":null`, 1),
			wantStatus: http.StatusBadRequest,
			wantCode:   invalidRequestCode,
		},
		{
			name:       "missing latitude",
			body:       strings.Replace(validBody, `"latitude":55.75,`, "", 1),
			wantStatus: http.StatusBadRequest,
			wantCode:   invalidRequestCode,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctrl := gomock.NewController(t)
			useCase := NewMocktripUseCase(ctrl)
			handler := createhandler.NewHandler(useCase, slog.New(slog.DiscardHandler))

			path := "/api/v1/trips"
			request := httptest.NewRequest(
				http.MethodPost,
				path,
				strings.NewReader(tt.body),
			).WithContext(t.Context())
			request.Header.Set("Content-Type", "application/json")

			recorder := httptest.NewRecorder()

			createdTrip := model.Trip{
				ID:             uuid.New(),
				UserID:         userID,
				DriverID:       driverID,
				StartLatitude:  55.75,
				StartLongitude: 37.61,
				EndLatitude:    55.80,
				EndLongitude:   37.70,
				Price:          tt.price,
				Status:         domaintrip.StatusActive,
				StartedAt:      time.Date(2026, 10, 25, 12, 0, 0, 0, time.UTC),
			}

			if tt.callUseCase {
				expectedParams := domaintrip.CreateParams{
					UserID:         userID,
					DriverID:       driverID,
					StartLatitude:  55.75,
					StartLongitude: 37.61,
					EndLatitude:    55.80,
					EndLongitude:   37.70,
					Price:          tt.price,
				}

				result := createdTrip
				if tt.useCaseErr != nil {
					result = model.Trip{}
				}

				useCase.EXPECT().
					Create(request.Context(), expectedParams).
					Return(result, tt.useCaseErr)
			}
			// Для невалидного тела ожидания нет:
			// вызов usecase будет ошибкой теста.

			handler.CreateTrip(recorder, request, api.CreateTripParams{})

			response := recorder.Result()
			defer response.Body.Close()

			require.Equal(t, tt.wantStatus, response.StatusCode)

			if tt.wantStatus == http.StatusCreated {
				assert.Equal(
					t,
					"application/json",
					response.Header.Get("Content-Type"),
				)
				assert.Equal(
					t,
					path+"/"+createdTrip.ID.String(),
					response.Header.Get("Location"),
				)

				var body api.Trip
				require.NoError(t, json.NewDecoder(response.Body).Decode(&body))

				assert.Equal(t, createdTrip.ID, body.Id)
				assert.Equal(t, userID, body.UserId)
				assert.Equal(t, driverID, body.DriverId)
				assert.Equal(t, tt.price, body.Price)
				assert.Equal(
					t,
					api.TripStatus(domaintrip.StatusActive),
					body.Status,
				)
				assert.True(t, createdTrip.StartedAt.Equal(body.StartedAt))
				assert.Nil(t, body.FinishedAt)
				return
			}

			assert.Equal(
				t,
				"application/problem+json",
				response.Header.Get("Content-Type"),
			)
			assert.Empty(t, response.Header.Get("Location"))

			var body api.Problem
			require.NoError(t, json.NewDecoder(response.Body).Decode(&body))

			assert.Equal(t, tt.wantStatus, int(body.Status))
			assert.Equal(t, tt.wantCode, body.Code)
			require.NotNil(t, body.Instance)
			assert.Equal(t, path, *body.Instance)
		})
	}
}
