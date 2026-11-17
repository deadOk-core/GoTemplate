package createposition

import (
	"context"
	"encoding/json"
	"io"

	domaintrip "github.com/deadOk-core/GoTemplate/internal/domain/trip"
	api "github.com/deadOk-core/GoTemplate/internal/generated"
	httptrip "github.com/deadOk-core/GoTemplate/internal/http/trip"
)

type Handler struct {
	useCase positionUseCase
}

func NewHandler(useCase positionUseCase) *Handler {
	return &Handler{useCase: useCase}
}

func (h *Handler) Create(ctx context.Context, tripID api.TripId, body io.Reader) (api.TripPosition, error) {
	params, err := decodePosition(body)
	if err != nil {
		return api.TripPosition{}, err
	}
	position, err := h.useCase.Create(ctx, tripID, params)
	if err != nil {
		return api.TripPosition{}, err
	}
	return httptrip.PositionResponse(position), nil
}

func decodePosition(body io.Reader) (domaintrip.PositionParams, error) {
	invalidRequest := &domaintrip.ValidationError{Field: "body", Message: "invalid request"}
	decoder := json.NewDecoder(body)
	var fields map[string]*json.RawMessage
	if err := decoder.Decode(&fields); err != nil || len(fields) != 2 ||
		fields["coordinates"] == nil || fields["recorded_at"] == nil {
		return domaintrip.PositionParams{}, invalidRequest
	}
	if err := decoder.Decode(new(json.RawMessage)); err != io.EOF {
		return domaintrip.PositionParams{}, invalidRequest
	}
	var coordinates map[string]*json.RawMessage
	if err := json.Unmarshal(*fields["coordinates"], &coordinates); err != nil || len(coordinates) != 2 ||
		coordinates["latitude"] == nil || coordinates["longitude"] == nil {
		return domaintrip.PositionParams{}, invalidRequest
	}
	var request api.PositionData
	if err := json.Unmarshal(*fields["coordinates"], &request.Coordinates); err != nil {
		return domaintrip.PositionParams{}, invalidRequest
	}
	if err := json.Unmarshal(*fields["recorded_at"], &request.RecordedAt); err != nil {
		return domaintrip.PositionParams{}, invalidRequest
	}
	return domaintrip.PositionParams{
		Latitude:   request.Coordinates.Latitude,
		Longitude:  request.Coordinates.Longitude,
		RecordedAt: request.RecordedAt,
	}, nil
}
