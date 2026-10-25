package container

import (
	"context"
	"fmt"
	"log/slog"
	"net/http"
	"os"

	"github.com/go-chi/chi/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/deadOk-core/GoTemplate/internal/config"
	api "github.com/deadOk-core/GoTemplate/internal/generated"
	httpapi "github.com/deadOk-core/GoTemplate/internal/http"
	createhandler "github.com/deadOk-core/GoTemplate/internal/http/trip/create"
	finishhandler "github.com/deadOk-core/GoTemplate/internal/http/trip/finish"
	gethandler "github.com/deadOk-core/GoTemplate/internal/http/trip/get"
	"github.com/deadOk-core/GoTemplate/internal/infra/postgres"
	"github.com/deadOk-core/GoTemplate/internal/infra/postgres/transaction"
	"github.com/deadOk-core/GoTemplate/internal/infra/telemetry"
	triprepository "github.com/deadOk-core/GoTemplate/internal/repository/trip"
	createusecase "github.com/deadOk-core/GoTemplate/internal/usecase/trip/create"
	finishusecase "github.com/deadOk-core/GoTemplate/internal/usecase/trip/finish"
	getusecase "github.com/deadOk-core/GoTemplate/internal/usecase/trip/get"
)

type Container struct {
	Server *http.Server
	pool   *pgxpool.Pool
	Logger *slog.Logger
}

func NewContainer(ctx context.Context, cfg config.Config) (*Container, error) {
	logger := telemetry.NewLogger(
		os.Stdout,
		cfg.LogLevel,
		cfg.ServiceName,
	)

	pool, err := postgres.NewPool(ctx, cfg.Database)
	if err != nil {
		return nil, fmt.Errorf("create postgres pool: %w", err)
	}

	repository := triprepository.NewRepository(pool, cfg.Database.QueryTimeout)

	txManager := transaction.NewTransactionManager(pool)

	createUseCase := createusecase.NewUseCase(repository, txManager, logger)
	getUseCase := getusecase.NewUseCase(repository)
	finishUseCase := finishusecase.NewUseCase(repository, txManager, logger)

	createHandler := createhandler.NewHandler(createUseCase, logger)
	getHandler := gethandler.NewHandler(getUseCase, logger)
	finishHandler := finishhandler.NewHandler(finishUseCase, logger)

	handler := httpapi.NewHandler(
		createHandler,
		getHandler,
		finishHandler,
		pool,
		cfg.Database.QueryTimeout,
		logger,
	)

	router := chi.NewRouter()

	api.HandlerWithOptions(
		handler,
		api.ChiServerOptions{
			BaseRouter:       router,
			ErrorHandlerFunc: handler.HandleRequestError,
		},
	)

	server := &http.Server{
		Addr:              cfg.HTTPAddr,
		Handler:           router,
		ReadTimeout:       cfg.ShutdownTimeout,
		ReadHeaderTimeout: cfg.ShutdownTimeout,
		WriteTimeout:      cfg.ShutdownTimeout,
		IdleTimeout:       cfg.ShutdownTimeout,
		ErrorLog:          slog.NewLogLogger(logger.Handler(), slog.LevelError),
	}

	return &Container{
		Server: server,
		pool:   pool,
		Logger: logger,
	}, nil
}

func (c *Container) Close() {
	c.pool.Close()
}
