package listpositions_test

import (
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"

	domaintrip "github.com/deadOk-core/GoTemplate/internal/domain/trip"
	api "github.com/deadOk-core/GoTemplate/internal/generated"
	"github.com/deadOk-core/GoTemplate/internal/http/trip/listpositions"
	"github.com/deadOk-core/GoTemplate/internal/model"
)

func TestListPositionsResponse(t *testing.T) {
	useCase := NewMockpositionUseCase(gomock.NewController(t))
	handler := listpositions.NewHandler(useCase)
	id := uuid.New()
	point := domaintrip.PositionParams{Latitude: 59.93, Longitude: 30.33, RecordedAt: time.Now()}
	useCase.EXPECT().List(t.Context(), id).Return([]model.TripPosition{{ID: 42, TripID: id, PositionParams: point}}, nil)

	response, err := handler.List(t.Context(), id)
	require.NoError(t, err)
	assert.Equal(t, []api.TripPosition{{
		Id:          42,
		TripId:      id,
		Coordinates: api.Coordinates{Latitude: point.Latitude, Longitude: point.Longitude},
		RecordedAt:  point.RecordedAt,
	}}, response.Positions)
}

func TestListPositionsResponseEmptyAndMissing(t *testing.T) {
	useCase := NewMockpositionUseCase(gomock.NewController(t))
	handler := listpositions.NewHandler(useCase)
	emptyID := uuid.New()
	missingID := uuid.New()
	useCase.EXPECT().List(t.Context(), emptyID).Return(nil, nil)
	useCase.EXPECT().List(t.Context(), missingID).Return(nil, domaintrip.ErrTripNotFound)

	response, err := handler.List(t.Context(), emptyID)
	require.NoError(t, err)
	assert.Equal(t, []api.TripPosition{}, response.Positions)
	_, err = handler.List(t.Context(), missingID)
	require.ErrorIs(t, err, domaintrip.ErrTripNotFound)
}
