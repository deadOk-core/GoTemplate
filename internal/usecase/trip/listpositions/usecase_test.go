package listpositions_test

import (
	"errors"
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"

	domaintrip "github.com/deadOk-core/GoTemplate/internal/domain/trip"
	"github.com/deadOk-core/GoTemplate/internal/model"
	"github.com/deadOk-core/GoTemplate/internal/usecase/trip/listpositions"
)

func TestListPositions(t *testing.T) {
	queryFailure := errors.New("query failed")
	tests := []struct {
		name    string
		tripErr error
		listErr error
		wantErr error
	}{
		{name: "completed trip route"},
		{name: "missing trip", tripErr: domaintrip.ErrTripNotFound, wantErr: domaintrip.ErrTripNotFound},
		{name: "query failed", listErr: queryFailure, wantErr: queryFailure},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			repo := NewMocktripRepository(gomock.NewController(t))
			useCase := listpositions.NewUseCase(repo)
			id := uuid.New()
			positions := []model.TripPosition{{ID: 42, TripID: id}}
			get := repo.EXPECT().GetByID(t.Context(), id).Return(model.Trip{ID: id, Status: domaintrip.StatusCompleted}, tt.tripErr)
			if tt.tripErr == nil {
				repo.EXPECT().ListPositions(t.Context(), id).Return(positions, tt.listErr).After(get)
			}
			got, err := useCase.List(t.Context(), id)
			if tt.wantErr != nil {
				require.ErrorIs(t, err, tt.wantErr)
				assert.Nil(t, got)
				return
			}
			require.NoError(t, err)
			assert.Equal(t, positions, got)
		})
	}
}
