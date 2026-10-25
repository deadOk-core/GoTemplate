package domainerror

import (
	"errors"
	"net/http"

	domaintrip "github.com/deadOk-core/GoTemplate/internal/domain/trip"
	api "github.com/deadOk-core/GoTemplate/internal/generated"
)

func Problem(err error) (status int, body api.Problem) {
	// Неизвестная ошибка получает безопасный ответ без внутренних деталей.
	status = http.StatusInternalServerError
	detail := "Internal server error"
	body = api.Problem{
		Type:  "https://tripgo.example/problems/internal-error",
		Title: "Internal Server Error",
		Code:  "internal_error",
	}

	var validationErr *domaintrip.ValidationError

	switch {
	case errors.Is(err, domaintrip.ErrTripNotFound):
		status = http.StatusNotFound
		body.Type = "https://tripgo.example/problems/trip-not-found"
		body.Title = "Trip not found"
		body.Code = "trip_not_found"
		detail = "Trip was not found"

	case errors.Is(err, domaintrip.ErrTripCompleted):
		status = http.StatusConflict
		body.Type = "https://tripgo.example/problems/trip-completed"
		body.Title = "Trip completed"
		body.Code = "trip_completed"
		detail = "Operation is not allowed for a completed trip"

	case errors.Is(err, domaintrip.ErrDriverBusy):
		status = http.StatusConflict
		body.Type = "https://tripgo.example/problems/driver-busy"
		body.Title = "Driver busy"
		body.Code = "driver_busy"
		detail = "Driver already has an active trip"

	case errors.As(err, &validationErr):
		status = http.StatusBadRequest
		body.Type = "https://tripgo.example/problems/invalid-request"
		body.Title = "Invalid request"
		body.Code = "invalid_request"
		detail = "Request validation failed"
	}

	body.Status = int32(status)
	body.Detail = &detail

	return status, body
}
