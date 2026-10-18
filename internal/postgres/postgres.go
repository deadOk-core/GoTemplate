package postgres

import (
	"context"
	"fmt"
	"os"
	"strconv"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
)

func NewPool(ctx context.Context) (*pgxpool.Pool, error) {

	databaseURL := os.Getenv("DATABASE_URL")
	if databaseURL == "" {
		return nil, fmt.Errorf("DATABASE_URL is required")
	}

	maxConns, err := getInt32("DATABASE_MAX_CONNS")
	if err != nil {
		return nil, err
	}

	minConns, err := getInt32("DATABASE_MIN_CONNS")
	if err != nil {
		return nil, err
	}

	maxConnLifetime, err := getDuration("DATABASE_MAX_CONN_LIFETIME")
	if err != nil {
		return nil, err
	}

	connectTimeout, err := getDuration("DATABASE_CONNECT_TIMEOUT")
	if err != nil {
		return nil, err
	}

	cfg, err := pgxpool.ParseConfig(databaseURL)
	if err != nil {
		return nil, err
	}

	cfg.MaxConns = maxConns
	cfg.MinConns = minConns
	cfg.MaxConnLifetime = maxConnLifetime
	cfg.ConnConfig.ConnectTimeout = connectTimeout

	pool, err := pgxpool.NewWithConfig(ctx, cfg)
	if err != nil {
		return nil, err
	}

	pingCtx, cancel := context.WithTimeout(ctx, connectTimeout)
	defer cancel()

	if err := pool.Ping(pingCtx); err != nil {
		pool.Close()
		return nil, fmt.Errorf("ping database: %w", err)
	}
	return pool, nil
}

func getInt32(name string) (int32, error) {
	valueStr := os.Getenv(name)
	if valueStr == "" {
		return 0, fmt.Errorf("%s is required", name)
	}

	value, err := strconv.ParseInt(valueStr, 10, 32)
	if err != nil {
		return 0, fmt.Errorf("%s must be an integer: %w", name, err)
	}

	return int32(value), nil
}

func getDuration(name string) (time.Duration, error) {
	valueStr := os.Getenv(name)
	if valueStr == "" {
		return 0, fmt.Errorf("%s is required", name)
	}

	timeDuration, err := time.ParseDuration(valueStr)
	if err != nil {
		return 0, fmt.Errorf("%s must be a duration: %w", name, err)
	}

	return timeDuration, nil
}
