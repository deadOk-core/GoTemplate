package http_test

import (
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	httpapi "github.com/deadOk-core/GoTemplate/internal/http"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"
)

func TestHandlerReady(t *testing.T) {
	tests := []struct {
		name       string
		pingErr    error
		wantStatus int
		wantBody   string
	}{
		{
			name:       "database available",
			wantStatus: http.StatusOK,
			wantBody:   `{"status":"ok"}`,
		},
		{
			name:       "database unavailable",
			pingErr:    errors.New("database unavailable"),
			wantStatus: http.StatusServiceUnavailable,
			wantBody:   `{"status":"unavailable"}`,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctrl := gomock.NewController(t)
			pinger := NewMockdatabasePinger(ctrl)

			// Ready не использует сервис поездок, поэтому передаём nil.
			handler := httpapi.NewHandler(nil, nil, nil, pinger, 5*time.Second)

			request := httptest.NewRequest(http.MethodGet, "/ready", nil).
				WithContext(t.Context())
			recorder := httptest.NewRecorder()

			pinger.EXPECT().
				Ping(gomock.Any()).
				Return(tt.pingErr)

			handler.Ready(recorder, request)

			response := recorder.Result()
			defer response.Body.Close()

			require.Equal(t, tt.wantStatus, response.StatusCode)
			assert.Equal(
				t,
				"application/json",
				response.Header.Get("Content-Type"),
			)
			assert.JSONEq(t, tt.wantBody, recorder.Body.String())
		})
	}
}
