package main

import (
	"context"
	"errors"
	"fmt"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"

	"github.com/deadOk-core/GoTemplate/internal/config"
	"github.com/deadOk-core/GoTemplate/internal/container"
)

func main() {
	if err := run(context.Background()); err != nil {
		log.Print(err)
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

	log.Printf("HTTP server is listening on %s", cfg.HTTPAddr)

	select {
	case err := <-serverErrors:
		app.Close()
		if err != nil && !errors.Is(err, http.ErrServerClosed) {
			return fmt.Errorf("HTTP server error: %w", err)
		}

		return nil

	case <-ctx.Done():
		log.Print("shutdown signal received")
	}

	shutdownCtx, cancel := context.WithTimeout(
		rootCtx,
		cfg.ShutdownTimeout,
	)
	defer cancel()

	shutdownDone := make(chan error, 1)

	go func() {
		err := app.Server.Shutdown(shutdownCtx)
		app.Close()
		shutdownDone <- err
	}()

	select {
	case err := <-shutdownDone:
		if err != nil {
			return fmt.Errorf("application shutdown: %w", err)
		}

	case <-shutdownCtx.Done():
		return fmt.Errorf("application shutdown: %w", shutdownCtx.Err())
	}

	log.Print("HTTP server stopped")

	return nil
}
