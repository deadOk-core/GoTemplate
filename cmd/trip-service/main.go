package main

import (
	"context"
	"errors"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"

	"github.com/deadOk-core/GoTemplate/internal/config"
	"github.com/deadOk-core/GoTemplate/internal/container"
)

func main() {
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	cfg, err := config.Load()
	if err != nil {
		log.Fatalf("load configuration: %v", err)
	}

	app, err := container.NewContainer(ctx, cfg)
	if err != nil {
		log.Fatalf("create application: %v", err)
	}
	defer app.Close()

	serverErrors := make(chan error, 1)

	go func() {
		serverErrors <- app.Server.ListenAndServe()
	}()

	log.Printf("HTTP server is listening on %s", cfg.HTTPAddr)

	select {
	case err := <-serverErrors:
		if err != nil && !errors.Is(err, http.ErrServerClosed) {
			app.Close()
			log.Fatalf("HTTP server error: %v", err)
		}

	case <-ctx.Done():
		log.Print("shutdown signal received")
	}

	shutdownCtx, cancel := context.WithTimeout(context.Background(), cfg.ShutdownTimeout)
	defer cancel()

	if err := app.Server.Shutdown(shutdownCtx); err != nil {
		log.Printf("HTTP server shutdown error: %v", err)
		os.Exit(1)
	}

	log.Print("HTTP server stopped")
}
