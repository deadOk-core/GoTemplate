package positions_test

import (
	"bytes"
	"context"
	"errors"
	"log/slog"
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/otel/attribute"
	sdkmetric "go.opentelemetry.io/otel/sdk/metric"
	"go.opentelemetry.io/otel/sdk/metric/metricdata"
	"go.uber.org/mock/gomock"

	"github.com/deadOk-core/GoTemplate/internal/model"
	"github.com/deadOk-core/GoTemplate/internal/worker/positions"
)

func TestRequesterRecordsResultsAndLogsFailure(t *testing.T) {
	reader := sdkmetric.NewManualReader()
	provider := sdkmetric.NewMeterProvider(sdkmetric.WithReader(reader))
	t.Cleanup(func() { assert.NoError(t, provider.Shutdown(context.Background())) })
	var logs bytes.Buffer
	logger := slog.New(slog.NewJSONHandler(&logs, nil))
	gateway := NewMockpushGateway(gomock.NewController(t))
	requester, err := positions.NewRequester(gateway, logger, provider.Meter("test"))
	require.NoError(t, err)
	trip := model.StaleTrip{ID: uuid.New(), DriverID: uuid.New()}
	call := gateway.EXPECT().RequestPosition(t.Context(), trip.DriverID, trip.ID).Return(nil)
	gateway.EXPECT().RequestPosition(t.Context(), trip.DriverID, trip.ID).Return(errors.New("unavailable")).After(call)

	requester.Request(t.Context(), trip)
	assert.Empty(t, logs.String())
	requester.Request(t.Context(), trip)
	assert.Contains(t, logs.String(), `"level":"WARN"`)
	assert.Contains(t, logs.String(), trip.ID.String())
	assert.Contains(t, logs.String(), "unavailable")
	assert.NotContains(t, logs.String(), trip.DriverID.String())

	requests := collectMetric(t, reader, positionRequestsMetric)
	require.Equal(t, positionRequestsMetric, requests.Name)
	want := map[string]int64{resultSuccess: 1, resultError: 1}
	assert.Equal(t, want, counterResults(t, requests))

	duration := collectMetric(t, reader, "tripgo.position_requests.duration")
	assert.Equal(t, "s", duration.Unit)
	histogram, ok := duration.Data.(metricdata.Histogram[float64])
	require.True(t, ok)
	results := make(map[string]uint64, len(histogram.DataPoints))
	for _, point := range histogram.DataPoints {
		result, ok := point.Attributes.Value(attribute.Key("result"))
		require.True(t, ok)
		assert.GreaterOrEqual(t, point.Sum, 0.0)
		results[result.AsString()] = point.Count
	}
	assert.Equal(t, map[string]uint64{resultSuccess: 1, resultError: 1}, results)
}
