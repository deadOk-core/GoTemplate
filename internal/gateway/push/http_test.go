package push_test

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/deadOk-core/GoTemplate/internal/gateway/push"
)

func newGateway(t *testing.T, server *httptest.Server, timeout time.Duration) *push.Gateway {
	t.Helper()
	t.Cleanup(server.Close)
	baseURL, err := url.Parse(server.URL)
	require.NoError(t, err)
	gateway := push.NewGateway(baseURL, timeout)
	t.Cleanup(gateway.CloseIdleConnections)
	return gateway
}

func TestRequestPositionPayload(t *testing.T) {
	driverID, tripID := uuid.New(), uuid.New()
	seen := make(chan string, 2)
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, http.MethodPost, r.Method)
		assert.Equal(t, "/api/v1/push", r.URL.Path)
		assert.Equal(t, "application/json", r.Header.Get("Content-Type"))

		var body struct {
			RequestID   string            `json:"request_id"`
			RecipientID uuid.UUID         `json:"recipient_id"`
			Kind        string            `json:"kind"`
			Data        map[string]string `json:"data"`
		}
		assert.NoError(t, json.NewDecoder(r.Body).Decode(&body))
		assert.Equal(t, driverID, body.RecipientID)
		assert.Equal(t, "REQUEST_POSITION", body.Kind)
		assert.Equal(t, map[string]string{"trip_id": tripID.String()}, body.Data)
		seen <- body.RequestID
		w.WriteHeader(http.StatusAccepted)
		_, _ = w.Write([]byte(`{"message_id":"accepted","accepted_at":"2026-11-15T12:00:00Z"}`))
	}))
	gateway := newGateway(t, server, time.Second)

	require.NoError(t, gateway.RequestPosition(t.Context(), driverID, tripID))
	require.NoError(t, gateway.RequestPosition(t.Context(), driverID, tripID))
	first, second := <-seen, <-seen
	_, err := uuid.Parse(first)
	require.NoError(t, err)
	_, err = uuid.Parse(second)
	require.NoError(t, err)
	assert.NotEqual(t, first, second)
}

func TestRequestPositionHTTPStatus(t *testing.T) {
	tests := []struct {
		name   string
		status int
	}{
		{name: "unexpected success code", status: http.StatusOK},
		{name: "redirect", status: http.StatusTemporaryRedirect},
		{name: "invalid request", status: http.StatusBadRequest},
		{name: "too many requests", status: http.StatusTooManyRequests},
		{name: "service unavailable", status: http.StatusServiceUnavailable},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var attempts atomic.Int32
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
				attempts.Add(1)
				w.Header().Set("Location", "/other")
				w.Header().Set("Retry-After", "1")
				w.WriteHeader(tt.status)
			}))
			gateway := newGateway(t, server, time.Second)

			err := gateway.RequestPosition(t.Context(), uuid.New(), uuid.New())
			var statusErr *push.HTTPError
			require.ErrorAs(t, err, &statusErr)
			assert.Equal(t, tt.status, statusErr.StatusCode)
			assert.EqualValues(t, 1, attempts.Load())
		})
	}
}

func TestRequestPositionTimeoutAndCancellation(t *testing.T) {
	exit := make(chan struct{})
	server := httptest.NewServer(http.HandlerFunc(func(_ http.ResponseWriter, r *http.Request) {
		_, err := io.Copy(io.Discard, r.Body)
		assert.NoError(t, err)
		select {
		case <-r.Context().Done():
		case <-exit:
		}
	}))
	gateway := newGateway(t, server, 100*time.Millisecond)
	t.Cleanup(func() { close(exit) })
	tripID, driverID := uuid.New(), uuid.New()

	err := gateway.RequestPosition(t.Context(), driverID, tripID)
	var transportErr *url.Error
	require.ErrorAs(t, err, &transportErr)
	assert.ErrorIs(t, err, context.DeadlineExceeded)

	ctx, cancel := context.WithCancel(t.Context())
	cancel()
	err = gateway.RequestPosition(ctx, driverID, tripID)
	require.ErrorAs(t, err, &transportErr)
	assert.ErrorIs(t, err, context.Canceled)
}

func TestRequestPositionClosesUnreadResponse(t *testing.T) {
	closed := make(chan struct{})
	exit := make(chan struct{})
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, err := io.Copy(io.Discard, r.Body)
		assert.NoError(t, err)
		w.WriteHeader(http.StatusAccepted)
		_, err = w.Write([]byte(strings.Repeat("x", 8192)))
		assert.NoError(t, err)
		assert.NoError(t, http.NewResponseController(w).Flush())
		select {
		case <-r.Context().Done():
			close(closed)
		case <-exit:
		}
	}))
	gateway := newGateway(t, server, 20*time.Second)
	t.Cleanup(func() { close(exit) })

	require.NoError(t, gateway.RequestPosition(t.Context(), uuid.New(), uuid.New()))
	ctx, cancel := context.WithTimeout(t.Context(), 3*time.Second)
	defer cancel()
	select {
	case <-closed:
	case <-ctx.Done():
		t.Fatal("gateway left the response body open")
	}
}
