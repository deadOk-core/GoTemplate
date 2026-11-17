package http_test

import (
	"errors"
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
	httpapi "github.com/deadOk-core/GoTemplate/internal/http"
)

func TestPositionsRoutes(t *testing.T) {
	const invalidRequest = "invalid_request"
	const notFound = "trip_not_found"
	const queryError = "database details"
	tests := []struct {
		name       string
		method     string
		invalidID  bool
		domainErr  error
		wantStatus int
		wantCode   string
	}{
		{name: "position created", method: http.MethodPost, wantStatus: http.StatusCreated},
		{name: "empty route", method: http.MethodGet, wantStatus: http.StatusOK},
		{name: "invalid body", method: http.MethodPost, domainErr: &domaintrip.ValidationError{Field: "body"}, wantStatus: http.StatusBadRequest, wantCode: invalidRequest},
		{name: "invalid trip ID on create", method: http.MethodPost, invalidID: true, wantStatus: http.StatusBadRequest, wantCode: invalidRequest},
		{name: "invalid trip ID on list", method: http.MethodGet, invalidID: true, wantStatus: http.StatusBadRequest, wantCode: invalidRequest},
		{name: "create missing trip", method: http.MethodPost, domainErr: domaintrip.ErrTripNotFound, wantStatus: http.StatusNotFound, wantCode: notFound},
		{name: "route missing trip", method: http.MethodGet, domainErr: domaintrip.ErrTripNotFound, wantStatus: http.StatusNotFound, wantCode: notFound},
		{name: "completed trip", method: http.MethodPost, domainErr: domaintrip.ErrTripCompleted, wantStatus: http.StatusConflict, wantCode: "trip_completed"},
		{name: "repository error", method: http.MethodGet, domainErr: errors.New(queryError), wantStatus: http.StatusInternalServerError, wantCode: "internal_error"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctrl := gomock.NewController(t)
			creator, lister := NewMocktripPositionCreator(ctrl), NewMocktripPositionLister(ctrl)
			root := httpapi.NewHandler(nil, nil, nil, creator, lister, nil, time.Second, slog.New(slog.DiscardHandler))
			router := api.HandlerWithOptions(root, api.ChiServerOptions{ErrorHandlerFunc: root.HandleRequestError})
			id := uuid.New()
			pathID := id.String()
			switch {
			case tt.invalidID:
				pathID = "invalid"
			case tt.method == http.MethodPost:
				creator.EXPECT().Create(gomock.Any(), id, gomock.Any()).Return(api.TripPosition{Id: 42, TripId: id}, tt.domainErr)
			default:
				lister.EXPECT().List(gomock.Any(), id).Return(api.TripPositionsResponse{Positions: []api.TripPosition{}}, tt.domainErr)
			}

			path := "/api/v1/trips/" + pathID + "/positions"
			request := httptest.NewRequestWithContext(t.Context(), tt.method, path, strings.NewReader(`{}`))
			response := httptest.NewRecorder()
			router.ServeHTTP(response, request)
			require.Equal(t, tt.wantStatus, response.Code, response.Body.String())
			if tt.wantCode != "" {
				assert.Equal(t, "application/problem+json", response.Header().Get("Content-Type"))
				assert.Contains(t, response.Body.String(), `"code":"`+tt.wantCode+`"`)
				assert.Contains(t, response.Body.String(), `"instance":"`+path+`"`)
				assert.NotContains(t, response.Body.String(), queryError)
				return
			}
			assert.Equal(t, "application/json", response.Header().Get("Content-Type"))
			if tt.method == http.MethodGet {
				assert.JSONEq(t, `{"positions":[]}`, response.Body.String())
			} else {
				assert.Contains(t, response.Body.String(), `"id":42`)
			}
		})
	}
}
