package httpapi

import (
	"errors"
	"log"
	"net/http"

	api "github.com/deadOk-core/GoTemplate/internal/generated"
	"github.com/deadOk-core/GoTemplate/internal/trip"
)

func (h *Handler) GetTrip(w http.ResponseWriter, r *http.Request, tripId api.TripId) {
	// UUID уже разобран сгенерированным роутером из параметра пути tripId.
	foundTrip, err := h.service.GetByID(r.Context(), tripId)

	if err != nil {
		// Отсутствующую поездку преобразуем в 404 trip_not_found.
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

		// Неизвестную внутреннюю ошибку записываем в лог,
		// а клиенту не раскрываем детали базы данных.
		log.Printf("get trip: %v", err)

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

	writeJSON(w, http.StatusOK, toAPITrip(foundTrip))
}
