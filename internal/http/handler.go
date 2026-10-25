package http

import (
	"net/http"
	"time"

	api "github.com/deadOk-core/GoTemplate/internal/generated"
)

type Handler struct {
	creator      tripCreator
	getter       tripGetter
	finisher     tripFinisher
	pinger       databasePinger
	queryTimeout time.Duration
}

func NewHandler(
	creator tripCreator,
	getter tripGetter,
	finisher tripFinisher,
	pinger databasePinger,
	queryTimeout time.Duration,
) *Handler {
	return &Handler{
		creator:      creator,
		getter:       getter,
		finisher:     finisher,
		pinger:       pinger,
		queryTimeout: queryTimeout,
	}
}

func (h *Handler) CreateTrip(
	w http.ResponseWriter,
	r *http.Request,
	params api.CreateTripParams,
) {
	h.creator.CreateTrip(w, r, params)
}

func (h *Handler) GetTrip(
	w http.ResponseWriter,
	r *http.Request,
	tripID api.TripId,
) {
	h.getter.GetTrip(w, r, tripID)
}

func (h *Handler) FinishTrip(
	w http.ResponseWriter,
	r *http.Request,
	tripID api.TripId,
) {
	h.finisher.FinishTrip(w, r, tripID)
}
