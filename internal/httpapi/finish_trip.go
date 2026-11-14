package httpapi

import (
	"errors"
	"log"
	"net/http"

	api "github.com/deadOk-core/GoTemplate/internal/generated"
	"github.com/deadOk-core/GoTemplate/internal/trip"
)

func (h *Handler) FinishTrip(w http.ResponseWriter, r *http.Request, tripId api.TripId) {
	finishedTrip, err := h.service.Finish(r.Context(), tripId)

	if err != nil {
		// Поездки с таким UUID нет.
		if errors.Is(err, trip.ErrTripNotFound) {
			writeProblem(
				w,
				r,
				http.StatusNotFound,
				"https://tripgo.example/problems/trip-not-found",
				"Trip not found",
				"Trip was not found",
				"trip_not_found",
			)
			return
		}

		// Поездка уже завершена, поэтому повторное завершение запрещено.
		if errors.Is(err, trip.ErrTripCompleted) {
			writeProblem(
				w,
				r,
				http.StatusConflict,
				"https://tripgo.example/problems/trip-completed",
				"Trip completed",
				"Operation is not allowed for a completed trip",
				"trip_completed",
			)
			return
		}

		// Неизвестную внутреннюю ошибку записываем в лог,
		// но не раскрываем клиенту детали базы данных.
		log.Printf("finish trip: %v", err)

		writeProblem(
			w,
			r,
			http.StatusInternalServerError,
			"https://tripgo.example/problems/internal-error",
			"Internal Server Error",
			"Internal server error",
			"internal_error",
		)
		return
	}

	writeJSON(w, http.StatusOK, toAPITrip(finishedTrip))
}
