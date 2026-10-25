package finish

import (
	"encoding/json"
	"log"
	"net/http"

	api "github.com/deadOk-core/GoTemplate/internal/generated"
	"github.com/deadOk-core/GoTemplate/internal/http/domainerror"
)

type Handler struct {
	useCase tripUseCase
}

func NewHandler(useCase tripUseCase) *Handler {
	return &Handler{
		useCase: useCase,
	}
}

func (h *Handler) FinishTrip(
	w http.ResponseWriter,
	r *http.Request,
	tripID api.TripId,
) {
	finishedTrip, err := h.useCase.Finish(r.Context(), tripID)

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
			log.Printf("finish trip: %v", err)
		}
	} else {
		body = toAPITrip(finishedTrip)
	}

	w.Header().Set("Content-Type", contentType)
	w.WriteHeader(status)

	if err := json.NewEncoder(w).Encode(body); err != nil {
		log.Printf("encode finish trip response: %v", err)
	}
}
