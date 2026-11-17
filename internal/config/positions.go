package config

import (
	"fmt"
	"net/url"
	"os"
	"strconv"
	"time"
)

type PositionsConfig struct {
	CheckInterval time.Duration
	StaleAfter    time.Duration
	Workers       int
	QueueSize     int
}

type PushConfig struct {
	HTTPURL        *url.URL
	RequestTimeout time.Duration
}

func loadPositionsConfig() (PositionsConfig, error) {
	var cfg PositionsConfig
	var err error
	cfg.CheckInterval, err = durationOrDefault("POSITION_CHECK_INTERVAL", 30*time.Second)
	if err != nil {
		return PositionsConfig{}, err
	}
	cfg.StaleAfter, err = durationOrDefault("POSITION_STALE_AFTER", time.Minute)
	if err != nil {
		return PositionsConfig{}, err
	}
	cfg.Workers, err = positiveIntOrDefault("POSITION_WORKERS", 4)
	if err != nil {
		return PositionsConfig{}, err
	}
	cfg.QueueSize, err = positiveIntOrDefault("POSITION_QUEUE_SIZE", 100)
	if err != nil {
		return PositionsConfig{}, err
	}
	return cfg, nil
}

func loadPushConfig() (PushConfig, error) {
	address, err := required("PUSH_HTTP_URL")
	if err != nil {
		return PushConfig{}, err
	}
	endpoint, err := url.Parse(address)
	if err != nil || endpoint.Hostname() == "" || (endpoint.Scheme != "http" && endpoint.Scheme != "https") {
		return PushConfig{}, fmt.Errorf("ENV PUSH_HTTP_URL must be an absolute HTTP(S) URL")
	}
	if endpoint.User != nil || endpoint.RawQuery != "" || endpoint.ForceQuery || endpoint.Fragment != "" {
		return PushConfig{}, fmt.Errorf("ENV PUSH_HTTP_URL must be a base URL without user info, query or fragment")
	}
	timeout, err := durationOrDefault("PUSH_REQUEST_TIMEOUT", time.Second)
	if err != nil {
		return PushConfig{}, err
	}
	return PushConfig{HTTPURL: endpoint, RequestTimeout: timeout}, nil
}

func durationOrDefault(name string, fallback time.Duration) (time.Duration, error) {
	value := os.Getenv(name)
	if value == "" {
		return fallback, nil
	}
	return positiveDuration(name, value)
}

func positiveIntOrDefault(name string, fallback int) (int, error) {
	value := os.Getenv(name)
	if value == "" {
		return fallback, nil
	}
	number, err := strconv.Atoi(value)
	if err != nil || number <= 0 {
		return 0, fmt.Errorf("ENV %s must be a positive integer", name)
	}
	return number, nil
}
