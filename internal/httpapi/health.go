package httpapi

import (
	"context"
	"net/http"

	api "github.com/deadOk-core/GoTemplate/internal/generated"
)

func (h *Handler) Health(w http.ResponseWriter, _ *http.Request) {
	writeJSON(w, http.StatusOK, api.HealthResponse{Status: api.Ok})
}

func (h *Handler) Ready(w http.ResponseWriter, r *http.Request) {
	pingCtx, cancel := context.WithTimeout(r.Context(), h.queryTimeout)
	defer cancel()

	if err := h.pool.Ping(pingCtx); err != nil {
		writeJSON(w, http.StatusServiceUnavailable, api.HealthResponse{Status: api.Unavailable})
		return
	}

	writeJSON(w, http.StatusOK, api.HealthResponse{Status: api.Ok})
}
