package http

import (
	"log/slog"
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
	logger       *slog.Logger
}

func NewHandler(
	creator tripCreator,
	getter tripGetter,
	finisher tripFinisher,
	pinger databasePinger,
	queryTimeout time.Duration,
	logger *slog.Logger,
) *Handler {
	return &Handler{
		creator:      creator,
		getter:       getter,
		finisher:     finisher,
		pinger:       pinger,
		queryTimeout: queryTimeout,
		logger:       logger,
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
