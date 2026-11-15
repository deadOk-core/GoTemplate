package http

import (
	"net/http"

	api "github.com/deadOk-core/GoTemplate/internal/generated"
	"github.com/deadOk-core/GoTemplate/internal/http/domainerror"
)

func (h *Handler) CreateTripPosition(w http.ResponseWriter, r *http.Request, tripID api.TripId) {
	position, err := h.positionCreator.Create(r.Context(), tripID, r.Body)
	h.writePositionResult(w, r, http.StatusCreated, position, err)
}

func (h *Handler) ListTripPositions(w http.ResponseWriter, r *http.Request, tripID api.TripId) {
	positions, err := h.positionLister.List(r.Context(), tripID)
	h.writePositionResult(w, r, http.StatusOK, positions, err)
}

func (h *Handler) writePositionResult(w http.ResponseWriter, r *http.Request, status int, body any, err error) {
	if err == nil {
		h.writeJSON(r.Context(), w, status, body)
		return
	}

	errorStatus, problem := domainerror.Problem(err)
	if errorStatus >= http.StatusInternalServerError {
		h.logger.ErrorContext(r.Context(), "trip positions request failed", "error", err)
	}
	h.writeProblemBody(w, r, problem)
}
