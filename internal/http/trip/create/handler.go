package create

import (
	"encoding/json"
	"io"
	"log"
	"net/http"

	domaintrip "github.com/deadOk-core/GoTemplate/internal/domain/trip"
	api "github.com/deadOk-core/GoTemplate/internal/generated"
	"github.com/deadOk-core/GoTemplate/internal/http/domainerror"
	"github.com/deadOk-core/GoTemplate/internal/model"
)

type Handler struct {
	useCase tripUseCase
}

func NewHandler(useCase tripUseCase) *Handler {
	return &Handler{
		useCase: useCase,
	}
}

func (h *Handler) CreateTrip(
	w http.ResponseWriter,
	r *http.Request,
	_ api.CreateTripParams,
) {
	payload, err := io.ReadAll(r.Body)
	if err != nil {
		err = &domaintrip.ValidationError{
			Field:   "body",
			Message: "cannot read request",
		}
	}

	// Каждый следующий шаг выполняется только при успехе предыдущего.
	var params domaintrip.CreateParams
	if err == nil {
		params, err = fromRequest(payload)
	}

	var createdTrip model.Trip
	if err == nil {
		createdTrip, err = h.useCase.Create(r.Context(), params)
	}

	status := http.StatusCreated
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
			log.Printf("create trip: %v", err)
		}
	} else {
		w.Header().Set("Location", "/api/v1/trips/"+createdTrip.ID.String())
		body = toAPITrip(createdTrip)
	}

	w.Header().Set("Content-Type", contentType)
	w.WriteHeader(status)

	if err := json.NewEncoder(w).Encode(body); err != nil {
		log.Printf("encode create trip response: %v", err)
	}
}
