package http

import (
	"context"
	"encoding/json"
	"net/http"

	api "github.com/deadOk-core/GoTemplate/internal/generated"
)

func (h *Handler) writeJSON(
	ctx context.Context,
	w http.ResponseWriter,
	status int,
	body any,
) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)

	if err := json.NewEncoder(w).Encode(body); err != nil {
		h.logger.ErrorContext(
			ctx,
			"encode JSON response failed",
			"error", err,
		)
	}
}

func (h *Handler) writeProblem(
	w http.ResponseWriter,
	r *http.Request,
	status int32,
	problemType string,
	title string,
	detail string,
	code string,
) {
	instance := r.URL.Path

	problem := api.Problem{
		Code:     code,
		Detail:   &detail,
		Instance: &instance,
		Status:   status,
		Title:    title,
		Type:     problemType,
	}

	w.Header().Set("Content-Type", "application/problem+json")
	w.WriteHeader(int(status))

	if err := json.NewEncoder(w).Encode(problem); err != nil {
		h.logger.ErrorContext(
			r.Context(),
			"encode problem response failed",
			"error", err,
		)
	}
}

// HandleRequestError преобразует ошибки разбора параметров,
// найденные сгенерированным роутером, в формат RFC 9457.
func (h *Handler) HandleRequestError(
	w http.ResponseWriter,
	r *http.Request,
	_ error,
) {
	h.writeProblem(
		w,
		r,
		http.StatusBadRequest,
		"https://tripgo.example/problems/invalid-request",
		"Invalid request",
		"Request validation failed",
		"invalid_request",
	)
}
