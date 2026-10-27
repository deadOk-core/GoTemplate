package http

import (
	"context"
	"net/http"

	api "github.com/deadOk-core/GoTemplate/internal/generated"
)

func (h *Handler) Health(
	w http.ResponseWriter,
	r *http.Request,
) {
	h.writeJSON(
		r.Context(),
		w,
		http.StatusOK,
		api.HealthResponse{Status: api.Ok},
	)
}

func (h *Handler) Ready(w http.ResponseWriter, r *http.Request) {
	pingCtx, cancel := context.WithTimeout(r.Context(), h.queryTimeout)
	defer cancel()

	if err := h.pinger.Ping(pingCtx); err != nil {
		h.logger.ErrorContext(
			r.Context(),
			"database readiness check failed",
			"error", err,
		)
		h.writeJSON(
			r.Context(),
			w,
			http.StatusServiceUnavailable,
			api.HealthResponse{Status: api.Unavailable},
		)
		return
	}

	h.writeJSON(
		r.Context(),
		w,
		http.StatusOK,
		api.HealthResponse{Status: api.Ok},
	)
}
