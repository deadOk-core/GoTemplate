package trip_test

import (
	"testing"
	"time"

	domaintrip "github.com/deadOk-core/GoTemplate/internal/domain/trip"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestTripFinish(t *testing.T) {
	at := time.Date(2026, time.October, 22, 12, 0, 0, 0, time.UTC)
	previousFinish := at.Add(-time.Hour)

	tests := []struct {
		name           string
		trip           domaintrip.Trip
		wantErr        error
		wantFinishedAt time.Time
	}{
		{
			name: "active trip",
			trip: domaintrip.Trip{
				Status: domaintrip.StatusActive,
			},
			wantFinishedAt: at,
		},
		{
			name: "completed trip",
			trip: domaintrip.Trip{
				Status:     domaintrip.StatusCompleted,
				FinishedAt: &previousFinish,
			},
			wantErr:        domaintrip.ErrTripCompleted,
			wantFinishedAt: previousFinish,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := tt.trip

			err := got.Finish(at)

			if tt.wantErr != nil {
				require.ErrorIs(t, err, tt.wantErr)
			} else {
				require.NoError(t, err)
			}

			assert.Equal(t, domaintrip.StatusCompleted, got.Status)
			require.NotNil(t, got.FinishedAt)
			assert.Equal(t, tt.wantFinishedAt, *got.FinishedAt)
		})
	}
}
