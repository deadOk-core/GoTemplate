package main

import (
	"context"
	"encoding/json"
	"errors"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	api "github.com/deadOk-core/GoTemplate/internal/generated"
	postgres "github.com/deadOk-core/GoTemplate/internal/postgres"
	"github.com/go-chi/chi/v5"
)

func main() {
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	dsn := "postgres://tripgo:tripgo@localhost:21032/tripgo?sslmode=disable"
	pool, err := postgres.NewPool(ctx, dsn)
	if err != nil {
		log.Fatal(err)
	}
	defer pool.Close()

	r := chi.NewRouter()

	r.HandleFunc("GET /health", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")

		response := api.HealthResponse{Status: api.Ok}

		if err := json.NewEncoder(w).Encode(response); err != nil {
			http.Error(w, "failed to encode response", http.StatusInternalServerError)
			return
		}
	})

	r.HandleFunc("GET /ready", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")

		pingCtx, cancel := context.WithTimeout(r.Context(), 5*time.Second)
		defer cancel()

		response := api.HealthResponse{Status: api.Ok}
		statusCode := http.StatusOK

		if err := pool.Ping(pingCtx); err != nil {
			response.Status = api.Unavailable
			statusCode = http.StatusServiceUnavailable
		}

		w.WriteHeader(statusCode)

		if err := json.NewEncoder(w).Encode(response); err != nil {
			log.Printf("failed to encode ready response: %v", err)
		}
	})

	port, ok := os.LookupEnv("HTTP_ADDR")
	if !ok || port == "" {
		return
	}

	s := http.Server{
		Addr:              port,
		Handler:           r,
		ReadTimeout:       10 * time.Second,
		ReadHeaderTimeout: 10 * time.Second,
		WriteTimeout:      10 * time.Second,
		IdleTimeout:       10 * time.Second,
	}

	go func() {
		log.Printf("HTTP server is listening on %s", port)

		if err := s.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			log.Printf("HTTP server error: %v", err)
		}
	}()

	<-ctx.Done()

	log.Print("shutdown signal received")

	shutdownCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)

	defer cancel()

	if err := s.Shutdown(shutdownCtx); err != nil {
		log.Printf("HTTP server shutdown error: %v", err)
		return
	}
	log.Print("HTTP server stopped")
}
