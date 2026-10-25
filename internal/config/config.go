package config

import (
	"fmt"
	"log/slog"
	"os"
	"strconv"
	"time"
)

type Config struct {
	HTTPAddr        string
	ServiceName     string
	LogLevel        slog.Level
	ShutdownTimeout time.Duration
	Database        DatabaseConfig
}

type DatabaseConfig struct {
	URL             string
	MaxConns        int32
	MinConns        int32
	MaxConnLifetime time.Duration
	ConnectTimeout  time.Duration
	QueryTimeout    time.Duration
}

func Load() (Config, error) {
	var cfg Config
	var err error

	cfg.HTTPAddr, err = required("HTTP_ADDR")
	if err != nil {
		return Config{}, err
	}

	cfg.ServiceName, err = required("OTEL_SERVICE_NAME")
	if err != nil {
		return Config{}, err
	}

	logLevel, err := required("LOG_LEVEL")
	if err != nil {
		return Config{}, err
	}

	err = cfg.LogLevel.UnmarshalText([]byte(logLevel))
	if err != nil {
		return Config{}, fmt.Errorf("ENV LOG_LEVEL is invalid: %w", err)
	}

	cfg.ShutdownTimeout, err = requiredDuration("SHUTDOWN_TIMEOUT")
	if err != nil {
		return Config{}, err
	}

	cfg.Database.URL, err = required("DATABASE_URL")
	if err != nil {
		return Config{}, err
	}

	cfg.Database.MaxConns, err = requiredInt32("DATABASE_MAX_CONNS")
	if err != nil {
		return Config{}, err
	}

	cfg.Database.MinConns, err = requiredInt32("DATABASE_MIN_CONNS")
	if err != nil {
		return Config{}, err
	}

	cfg.Database.MaxConnLifetime, err = requiredDuration(
		"DATABASE_MAX_CONN_LIFETIME",
	)
	if err != nil {
		return Config{}, err
	}

	cfg.Database.ConnectTimeout, err = requiredDuration(
		"DATABASE_CONNECT_TIMEOUT",
	)
	if err != nil {
		return Config{}, err
	}

	cfg.Database.QueryTimeout, err = requiredDuration(
		"DATABASE_QUERY_TIMEOUT",
	)
	if err != nil {
		return Config{}, err
	}

	if cfg.Database.MaxConns <= 0 {
		return Config{}, fmt.Errorf(
			"ENV DATABASE_MAX_CONNS must be greater than zero",
		)
	}

	if cfg.Database.MinConns < 0 {
		return Config{}, fmt.Errorf(
			"ENV DATABASE_MIN_CONNS must not be negative",
		)
	}

	if cfg.Database.MinConns > cfg.Database.MaxConns {
		return Config{}, fmt.Errorf(
			"ENV DATABASE_MIN_CONNS must not exceed DATABASE_MAX_CONNS",
		)
	}

	return cfg, nil
}

func required(name string) (string, error) {
	value := os.Getenv(name)
	if value == "" {
		return "", fmt.Errorf("ENV %s is required", name)
	}
	return value, nil
}

func requiredDuration(name string) (time.Duration, error) {
	value, err := required(name)
	if err != nil {
		return 0, err
	}

	duration, err := time.ParseDuration(value)
	if err != nil {
		return 0, fmt.Errorf("ENV %s must be a duration: %w", name, err)
	}

	if duration <= 0 {
		return 0, fmt.Errorf("ENV %s must be greater than zero", name)
	}

	return duration, nil
}

func requiredInt32(name string) (int32, error) {
	value, err := required(name)
	if err != nil {
		return 0, err
	}

	number, err := strconv.ParseInt(value, 10, 32)
	if err != nil {
		return 0, fmt.Errorf("ENV %s must be an int32: %w", name, err)
	}

	return int32(number), nil
}
