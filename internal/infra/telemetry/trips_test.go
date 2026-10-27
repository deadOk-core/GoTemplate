package telemetry_test

import (
	"context"
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	sdkmetric "go.opentelemetry.io/otel/sdk/metric"
	"go.opentelemetry.io/otel/sdk/metric/metricdata"
	"go.uber.org/mock/gomock"

	"github.com/deadOk-core/GoTemplate/internal/infra/telemetry"
)

func TestActiveTripsGauge(t *testing.T) {
	reader := sdkmetric.NewManualReader()
	meters := sdkmetric.NewMeterProvider(sdkmetric.WithReader(reader))
	t.Cleanup(func() { require.NoError(t, meters.Shutdown(context.Background())) })
	repository := NewMockactiveTripCounter(gomock.NewController(t))
	require.NoError(t, telemetry.RegisterActiveTrips(repository, meters.Meter("test")))

	for _, count := range []int64{7, 3, 0} {
		repository.EXPECT().CountActive(gomock.Any()).Return(count, nil)
		var metrics metricdata.ResourceMetrics

		require.NoError(t, reader.Collect(t.Context(), &metrics))
		require.Len(t, metrics.ScopeMetrics, 1)
		require.Len(t, metrics.ScopeMetrics[0].Metrics, 1)
		metric := metrics.ScopeMetrics[0].Metrics[0]
		assert.Equal(t, "tripgo.trips.active", metric.Name)
		gauge := metric.Data.(metricdata.Gauge[int64])
		require.Len(t, gauge.DataPoints, 1)
		assert.Equal(t, count, gauge.DataPoints[0].Value)
	}
	storageErr := errors.New("storage unavailable")
	repository.EXPECT().CountActive(gomock.Any()).Return(int64(0), storageErr)
	var metrics metricdata.ResourceMetrics
	assert.ErrorIs(t, reader.Collect(t.Context(), &metrics), storageErr)
}
