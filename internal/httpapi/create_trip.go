package httpapi

import (
	"encoding/json"
	"errors"
	"log"
	"net/http"

	"github.com/google/uuid"

	api "github.com/deadOk-core/GoTemplate/internal/generated"
	"github.com/deadOk-core/GoTemplate/internal/trip"
)

func (h *Handler) CreateTrip(w http.ResponseWriter, r *http.Request, _ api.CreateTripParams) {
	// Декодируем JSON-тело запроса в тип, сгенерированный из OpenAPI.
	// Неизвестные поля запрещены контрактом, поэтому включаем их отклонение.
	var request api.CreateTripJSONRequestBody

	decoder := json.NewDecoder(r.Body)
	decoder.DisallowUnknownFields()

	if err := decoder.Decode(&request); err != nil {
		writeProblem(
			w,
			r,
			http.StatusBadRequest,
			"https://tripgo.example/problems/invalid-request",
			"Invalid request",
			"Request validation failed",
			"invalid_request",
		)
		return
	}

	// Проверяем бизнес-правила до вызова сервисного слоя:
	// UUID не должны быть пустыми, координаты должны попадать
	// в допустимые диапазоны, цена не может быть отрицательной.
	if request.DriverId == uuid.Nil || request.UserId == uuid.Nil ||
		!isCoordinatesValid(request.EndPoint) || !isCoordinatesValid(request.StartPoint) ||
		request.Price < 0 {

		writeProblem(
			w,
			r,
			http.StatusBadRequest,
			"https://tripgo.example/problems/invalid-request",
			"Invalid request",
			"Request validation failed",
			"invalid_request",
		)
		return
	}

	// Преобразуем HTTP-модель из OpenAPI во внутренние параметры сервиса.
	// Служебные поля ID, status и started_at сервис заполнит самостоятельно.
	createdTrip, err := h.service.Create(r.Context(), trip.CreateParams{
		UserID:   request.UserId,
		DriverID: request.DriverId,

		StartLatitude:  request.StartPoint.Latitude,
		StartLongitude: request.StartPoint.Longitude,
		EndLatitude:    request.EndPoint.Latitude,
		EndLongitude:   request.EndPoint.Longitude,

		Price: request.Price,
	})

	if err != nil {
		// Доменную ошибку занятого водителя преобразуем
		// в предусмотренный контрактом ответ 409 driver_busy.

		if errors.Is(err, trip.ErrDriverBusy) {
			writeProblem(
				w,
				r,
				http.StatusConflict,
				"https://tripgo.example/problems/driver-busy",
				"Driver busy",
				"Driver already has an active trip",
				"driver_busy",
			)
			return
		}

		// Детали неизвестной внутренней ошибки сохраняем в логе.
		// Клиенту возвращаем безопасное сообщение без SQL и данных PostgreSQL.
		log.Printf("create trip: %v", err)

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
	// Контракт требует адрес созданного ресурса в заголовке Location.
	// Заголовок устанавливаем до writeJSON, потому что writeJSON отправит статус.
	w.Header().Set("Location", "/api/v1/trips/"+createdTrip.ID.String())

	writeJSON(w, http.StatusCreated, toAPITrip(createdTrip))
}

func isCoordinatesValid(coordinates api.Coordinates) bool {
	return coordinates.Latitude >= -90 &&
		coordinates.Latitude <= 90 &&
		coordinates.Longitude >= -180 &&
		coordinates.Longitude <= 180
}
