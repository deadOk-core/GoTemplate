package main

import (
	"context"
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

	"github.com/deadOk-core/GoTemplate/internal/httpapi"
	"github.com/deadOk-core/GoTemplate/internal/trip"
)

func main() {
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	pool, err := postgres.NewPool(ctx)
	if err != nil {
		log.Fatal(err)
	}
	defer pool.Close()

	logLevel := os.Getenv("LOG_LEVEL")
	if logLevel == "" {
		log.Fatal("LOG_LEVEL is required")
	}

	queryTimeoutString := os.Getenv("DATABASE_QUERY_TIMEOUT")
	if queryTimeoutString == "" {
		log.Fatal("DATABASE_QUERY_TIMEOUT is required")
	}

	queryTimeout, err := time.ParseDuration(queryTimeoutString)
	if err != nil {
		log.Fatalf("DATABASE_QUERY_TIMEOUT must be a duration: %v", err)
	}

	shutdownTimeoutString := os.Getenv("SHUTDOWN_TIMEOUT")
	if shutdownTimeoutString == "" {
		log.Fatal("SHUTDOWN_TIMEOUT is required")
	}

	shutdownTimeout, err := time.ParseDuration(shutdownTimeoutString)
	if err != nil {
		log.Fatalf("SHUTDOWN_TIMEOUT must be a duration: %v", err)
	}

	// Репозиторий выполняет SQL-запросы к PostgreSQL.
	repository := trip.NewRepository(pool, queryTimeout)

	// Менеджер управляет транзакциями и передаёт их через context.
	txManager := postgres.NewTransactionManager(pool)

	// Сервис выполняет бизнес-операции с поездками.
	service := trip.NewService(repository, txManager)

	// Обработчик связывает HTTP с бизнес-сервисом.
	handler := httpapi.NewHandler(service, pool, queryTimeout)

	// Роутер принимает запросы и выбирает нужный обработчик.
	router := chi.NewRouter()

	// Регистрируем маршруты, сгенерированные из OpenAPI.
	api.HandlerWithOptions(
		handler,
		api.ChiServerOptions{
			BaseRouter:       router,
			ErrorHandlerFunc: httpapi.HandleRequestError,
		},
	)

	httpAddr := os.Getenv("HTTP_ADDR")
	if httpAddr == "" {
		log.Fatal("HTTP_ADDR is required")
	}

	s := http.Server{
		Addr:              httpAddr,
		Handler:           router,
		ReadTimeout:       shutdownTimeout,
		ReadHeaderTimeout: shutdownTimeout,
		WriteTimeout:      shutdownTimeout,
		IdleTimeout:       shutdownTimeout,
	}

	go func() {
		log.Printf("HTTP server is listening on %s", httpAddr)

		if err := s.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			log.Printf("HTTP server error: %v", err)
		}
	}()

	<-ctx.Done()

	log.Print("shutdown signal received")

	shutdownCtx, cancel := context.WithTimeout(context.Background(), shutdownTimeout)

	defer cancel()

	if err := s.Shutdown(shutdownCtx); err != nil {
		log.Printf("HTTP server shutdown error: %v", err)
		return
	}
	log.Print("HTTP server stopped")
}
