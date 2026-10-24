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
	api "github.com/deadOk-core/GoTemplate/internal/generated"
	"github.com/deadOk-core/GoTemplate/internal/infra/postgres"
	"github.com/deadOk-core/GoTemplate/internal/infra/postgres/transaction"
	"github.com/go-chi/chi/v5"

	"github.com/deadOk-core/GoTemplate/internal/httpapi"
	triprepository "github.com/deadOk-core/GoTemplate/internal/repository/trip"
	"github.com/deadOk-core/GoTemplate/internal/trip"
)

func main() {
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	cfg, err := config.Load()
	if err != nil {
		log.Fatalf("load configuration: %v", err)
	}

	pool, err := postgres.NewPool(ctx, cfg.Database)
	if err != nil {
		log.Fatal(err)
	}
	defer pool.Close()

	// Репозиторий выполняет SQL-запросы к PostgreSQL.
	repository := triprepository.NewRepository(pool, cfg.Database.QueryTimeout)

	// Менеджер управляет транзакциями и передаёт их через context.
	txManager := transaction.NewTransactionManager(pool)

	// Сервис выполняет бизнес-операции с поездками.
	service := trip.NewService(repository, txManager)

	// Обработчик связывает HTTP с бизнес-сервисом.
	handler := httpapi.NewHandler(service, pool, cfg.Database.QueryTimeout)

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

	s := http.Server{
		Addr:              cfg.HTTPAddr,
		Handler:           router,
		ReadTimeout:       cfg.ShutdownTimeout,
		ReadHeaderTimeout: cfg.ShutdownTimeout,
		WriteTimeout:      cfg.ShutdownTimeout,
		IdleTimeout:       cfg.ShutdownTimeout,
	}

	go func() {
		log.Printf("HTTP server is listening on %s", cfg.HTTPAddr)

		if err := s.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			log.Fatalf("HTTP server error: %v", err)
		}
	}()

	<-ctx.Done()

	log.Print("shutdown signal received")

	shutdownCtx, cancel := context.WithTimeout(context.Background(), cfg.ShutdownTimeout)

	defer cancel()

	if err := s.Shutdown(shutdownCtx); err != nil {
		log.Printf("HTTP server shutdown error: %v", err)
		return
	}
	log.Print("HTTP server stopped")
}
