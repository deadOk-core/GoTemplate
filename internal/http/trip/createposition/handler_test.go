package createposition_test

import (
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"

	domaintrip "github.com/deadOk-core/GoTemplate/internal/domain/trip"
	api "github.com/deadOk-core/GoTemplate/internal/generated"
	"github.com/deadOk-core/GoTemplate/internal/http/trip/createposition"
	"github.com/deadOk-core/GoTemplate/internal/model"
)

func TestCreatePositionRequest(t *testing.T) {
	const validBody = `{"coordinates":{"latitude":0,"longitude":0},"recorded_at":"2026-08-24T12:00:00Z"}`
	tests := []struct {
		name  string
		body  string
		valid bool
	}{
		{name: "zero coordinates", body: validBody, valid: true},
		{name: "missing latitude", body: strings.Replace(validBody, `"latitude":0,`, "", 1)},
		{name: "null longitude", body: strings.Replace(validBody, `"longitude":0`, `"longitude":null`, 1)},
		{name: "null coordinates", body: strings.Replace(validBody, `{"latitude":0,"longitude":0}`, "null", 1)},
		{name: "missing time", body: `{"coordinates":{"latitude":0,"longitude":0}}`},
		{name: "null time", body: strings.Replace(validBody, `"2026-08-24T12:00:00Z"`, "null", 1)},
		{name: "invalid time", body: strings.Replace(validBody, `"2026-08-24T12:00:00Z"`, `"yesterday"`, 1)},
		{name: "unexpected field", body: strings.Replace(validBody, `"coordinates":`, `"id":1,"coordinates":`, 1)},
		{name: "wrong field case", body: strings.Replace(validBody, `"latitude"`, `"Latitude"`, 1)},
		{name: "trailing JSON", body: validBody + ` {}`},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			useCase := NewMockpositionUseCase(gomock.NewController(t))
			handler := createposition.NewHandler(useCase)
			id := uuid.New()
			recordedAt := time.Date(2026, time.August, 24, 12, 0, 0, 0, time.UTC)
			params := domaintrip.PositionParams{RecordedAt: recordedAt}
			if tt.valid {
				useCase.EXPECT().Create(t.Context(), id, params).Return(model.TripPosition{ID: 42, TripID: id, PositionParams: params}, nil)
			}

			position, err := handler.Create(t.Context(), id, strings.NewReader(tt.body))
			if !tt.valid {
				var validationErr *domaintrip.ValidationError
				require.ErrorAs(t, err, &validationErr)
				return
			}
			require.NoError(t, err)
			assert.Equal(t, api.TripPosition{Id: 42, TripId: id, RecordedAt: recordedAt}, position)
		})
	}
}
