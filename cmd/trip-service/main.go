package main

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"syscall"

	"github.com/deadOk-core/GoTemplate/internal/config"
	"github.com/deadOk-core/GoTemplate/internal/container"
)

func main() {
	if err := run(context.Background()); err != nil {
		logger := slog.New(
			slog.NewJSONHandler(os.Stdout, nil),
		)

		logger.Error(
			"application failed",
			"service", "trip-service",
			"trace_id", "",
			"span_id", "",
			"error", err,
		)

		os.Exit(1)
	}
}

func run(rootCtx context.Context) error {
	ctx, stop := signal.NotifyContext(
		rootCtx,
		os.Interrupt,
		syscall.SIGTERM,
	)
	defer stop()

	cfg, err := config.Load()
	if err != nil {
		return fmt.Errorf("load configuration: %w", err)
	}

	app, err := container.NewContainer(ctx, cfg)
	if err != nil {
		return fmt.Errorf("create application: %w", err)
	}

	serverErrors := make(chan error, 1)

	go func() {
		serverErrors <- app.Server.ListenAndServe()
	}()

	app.Logger.InfoContext(
		ctx,
		"HTTP server started",
		"address", cfg.HTTPAddr,
	)

	var serveErr error

	select {
	case serveErr = <-serverErrors:
		if errors.Is(serveErr, http.ErrServerClosed) {
			serveErr = nil
		}
		if serveErr != nil {
			serveErr = fmt.Errorf("HTTP server error: %w", serveErr)
		}

	case <-ctx.Done():
		app.Logger.InfoContext(rootCtx, "shutdown signal received")
	}

	shutdownCtx, cancel := context.WithTimeout(
		rootCtx,
		cfg.ShutdownTimeout,
	)
	defer cancel()

	shutdownDone := make(chan error, 1)

	go func() {
		httpErr := app.Server.Shutdown(shutdownCtx)
		closeErr := app.Close(shutdownCtx)
		shutdownDone <- errors.Join(httpErr, closeErr)
	}()

	select {
	case err := <-shutdownDone:
		if err != nil {
			return errors.Join(serveErr, fmt.Errorf("application shutdown: %w", err))
		}

	case <-shutdownCtx.Done():
		return errors.Join(serveErr, fmt.Errorf("application shutdown: %w", shutdownCtx.Err()))
	}

	if serveErr != nil {
		return serveErr
	}

	app.Logger.InfoContext(rootCtx, "HTTP server stopped")

	return nil
}
