package config_test

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/deadOk-core/GoTemplate/internal/config"
)

func TestLoadPositionsOverrides(t *testing.T) {
	setValidEnv(t)
	t.Setenv(positionCheckIntervalEnv, "45s")
	t.Setenv(positionStaleAfterEnv, "90s")
	t.Setenv(positionWorkersEnv, "6")
	t.Setenv(positionQueueSizeEnv, "150")
	t.Setenv(pushHTTPURLEnv, "https://push.example.test/base/")
	t.Setenv(pushTimeoutEnv, "750ms")

	got, err := config.Load()
	require.NoError(t, err)
	assert.Equal(t, config.PositionsConfig{
		CheckInterval: 45 * time.Second,
		StaleAfter:    90 * time.Second,
		Workers:       6,
		QueueSize:     150,
	}, got.Positions)
	assert.Equal(t, "https://push.example.test/base/", got.Push.HTTPURL.String())
	assert.Equal(t, 750*time.Millisecond, got.Push.RequestTimeout)
}

func TestLoadInvalidPositionsConfig(t *testing.T) {
	tests := []struct {
		name  string
		env   string
		value string
	}{
		{name: "ticker period zero", env: positionCheckIntervalEnv, value: "0s"},
		{name: "stale age negative", env: positionStaleAfterEnv, value: "-1s"},
		{name: "workers not numeric", env: positionWorkersEnv, value: "four"},
		{name: "workers overflow", env: positionWorkersEnv, value: "999999999999999999999"},
		{name: "queue empty capacity", env: positionQueueSizeEnv, value: "0"},
		{name: "timeout malformed", env: pushTimeoutEnv, value: "one second"},
		{name: "timeout zero", env: pushTimeoutEnv, value: "0s"},
		{name: "missing push URL", env: pushHTTPURLEnv},
		{name: "missing push scheme", env: pushHTTPURLEnv, value: "localhost:8090"},
		{name: "missing push host", env: pushHTTPURLEnv, value: "https:///api/v1/push"},
		{name: "query on push base URL", env: pushHTTPURLEnv, value: "https://localhost:8090?debug=1"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			setValidEnv(t)
			t.Setenv(tt.env, tt.value)
			_, err := config.Load()
			require.ErrorContains(t, err, tt.env)
		})
	}
}
