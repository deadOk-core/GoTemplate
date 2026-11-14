package get

import (
	"encoding/json"
	"log/slog"
	"net/http"

	api "github.com/deadOk-core/GoTemplate/internal/generated"
	"github.com/deadOk-core/GoTemplate/internal/http/domainerror"
)

type Handler struct {
	useCase tripUseCase
	logger  *slog.Logger
}

func NewHandler(
	useCase tripUseCase,
	logger *slog.Logger,
) *Handler {
	return &Handler{
		useCase: useCase,
		logger:  logger,
	}
}

func (h *Handler) GetTrip(
	w http.ResponseWriter,
	r *http.Request,
	tripID api.TripId,
) {
	foundTrip, err := h.useCase.GetByID(r.Context(), tripID)

	status := http.StatusOK
	contentType := "application/json"
	var body any

	if err != nil {
		errorStatus, problem := domainerror.Problem(err)
		status = errorStatus
		contentType = "application/problem+json"

		instance := r.URL.Path
		problem.Instance = &instance
		body = problem

		if status >= http.StatusInternalServerError {
			h.logger.ErrorContext(
				r.Context(),
				"get trip failed",
				"error", err,
			)
		}
	} else {
		body = toAPITrip(foundTrip)
	}

	w.Header().Set("Content-Type", contentType)
	w.WriteHeader(status)

	if err := json.NewEncoder(w).Encode(body); err != nil {
		h.logger.ErrorContext(
			r.Context(),
			"encode get trip response failed",
			"error", err,
		)
	}
}
