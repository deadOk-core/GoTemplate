package config_test

import (
	"log/slog"
	"testing"
	"time"

	"github.com/deadOk-core/GoTemplate/internal/config"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func setValidEnv(t *testing.T) {
	t.Helper()

	values := map[string]string{
		"HTTP_ADDR":                  ":8080",
		"LOG_LEVEL":                  "info",
		"SHUTDOWN_TIMEOUT":           "10s",
		"DATABASE_URL":               "postgres://test:test@localhost/test",
		"DATABASE_MAX_CONNS":         "10",
		"DATABASE_MIN_CONNS":         "2",
		"DATABASE_MAX_CONN_LIFETIME": "30m",
		"DATABASE_CONNECT_TIMEOUT":   "5s",
		"DATABASE_QUERY_TIMEOUT":     "3s",
	}

	for name, value := range values {
		t.Setenv(name, value)
	}
}

func TestLoad(t *testing.T) {
	setValidEnv(t)

	got, err := config.Load()
	if err != nil {
		t.Fatalf("Load returned an error: %v", err)
	}

	want := config.Config{
		HTTPAddr:        ":8080",
		LogLevel:        slog.LevelInfo,
		ShutdownTimeout: 10 * time.Second,
		Database: config.DatabaseConfig{
			URL:             "postgres://test:test@localhost/test",
			MaxConns:        10,
			MinConns:        2,
			MaxConnLifetime: 30 * time.Minute,
			ConnectTimeout:  5 * time.Second,
			QueryTimeout:    3 * time.Second,
		},
	}

	assert.Equal(t, want, got)
}

func TestLoadInvalid(t *testing.T) {
	tests := []struct {
		name  string
		env   string
		value string
	}{
		{
			name:  "empty HTTP address",
			env:   "HTTP_ADDR",
			value: "",
		},
		{
			name:  "empty database URL",
			env:   "DATABASE_URL",
			value: "",
		},
		{
			name:  "invalid log level",
			env:   "LOG_LEVEL",
			value: "banana",
		},
		{
			name:  "invalid duration",
			env:   "SHUTDOWN_TIMEOUT",
			value: "ten seconds",
		},
		{
			name:  "zero timeout",
			env:   "DATABASE_QUERY_TIMEOUT",
			value: "0s",
		},
		{
			name:  "negative timeout",
			env:   "DATABASE_CONNECT_TIMEOUT",
			value: "-1s",
		},
		{
			name:  "invalid connection count",
			env:   "DATABASE_MAX_CONNS",
			value: "ten",
		},
		{
			name:  "connection count overflow",
			env:   "DATABASE_MAX_CONNS",
			value: "2147483648",
		},
		{
			name:  "zero maximum connections",
			env:   "DATABASE_MAX_CONNS",
			value: "0",
		},
		{
			name:  "negative minimum connections",
			env:   "DATABASE_MIN_CONNS",
			value: "-1",
		},
		{
			name:  "minimum exceeds maximum",
			env:   "DATABASE_MIN_CONNS",
			value: "11",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			setValidEnv(t)
			t.Setenv(tt.env, tt.value)

			got, err := config.Load()

			require.Error(t, err)
			assert.Contains(t, err.Error(), tt.env)
			assert.Equal(t, config.Config{}, got)
		})
	}
}

func TestLoadZeroMinConns(t *testing.T) {
	setValidEnv(t)
	t.Setenv("DATABASE_MIN_CONNS", "0")

	got, err := config.Load()

	require.NoError(t, err)
	assert.Equal(t, int32(0), got.Database.MinConns)
}
