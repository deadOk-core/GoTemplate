package listpositions

import (
	"context"

	api "github.com/deadOk-core/GoTemplate/internal/generated"
	httptrip "github.com/deadOk-core/GoTemplate/internal/http/trip"
)

type Handler struct {
	useCase positionUseCase
}

func NewHandler(useCase positionUseCase) *Handler {
	return &Handler{useCase: useCase}
}

func (h *Handler) List(ctx context.Context, tripID api.TripId) (api.TripPositionsResponse, error) {
	positions, err := h.useCase.List(ctx, tripID)
	if err != nil {
		return api.TripPositionsResponse{}, err
	}
	response := api.TripPositionsResponse{Positions: make([]api.TripPosition, 0, len(positions))}
	for _, position := range positions {
		response.Positions = append(response.Positions, httptrip.PositionResponse(position))
	}
	return response, nil
}
