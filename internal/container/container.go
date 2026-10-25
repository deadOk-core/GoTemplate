package container

import (
	"context"
	"fmt"
	"net/http"

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
	triprepository "github.com/deadOk-core/GoTemplate/internal/repository/trip"
	createusecase "github.com/deadOk-core/GoTemplate/internal/usecase/trip/create"
	finishusecase "github.com/deadOk-core/GoTemplate/internal/usecase/trip/finish"
	getusecase "github.com/deadOk-core/GoTemplate/internal/usecase/trip/get"
)

type Container struct {
	Server *http.Server
	pool   *pgxpool.Pool
}

func NewContainer(ctx context.Context, cfg config.Config) (*Container, error) {
	pool, err := postgres.NewPool(ctx, cfg.Database)
	if err != nil {
		return nil, fmt.Errorf("create postgres pool: %w", err)
	}

	repository := triprepository.NewRepository(pool, cfg.Database.QueryTimeout)

	txManager := transaction.NewTransactionManager(pool)

	createUseCase := createusecase.NewUseCase(repository, txManager)
	getUseCase := getusecase.NewUseCase(repository)
	finishUseCase := finishusecase.NewUseCase(repository, txManager)

	createHandler := createhandler.NewHandler(createUseCase)
	getHandler := gethandler.NewHandler(getUseCase)
	finishHandler := finishhandler.NewHandler(finishUseCase)

	handler := httpapi.NewHandler(
		createHandler,
		getHandler,
		finishHandler,
		pool,
		cfg.Database.QueryTimeout,
	)

	router := chi.NewRouter()

	api.HandlerWithOptions(
		handler,
		api.ChiServerOptions{
			BaseRouter:       router,
			ErrorHandlerFunc: httpapi.HandleRequestError,
		},
	)

	server := &http.Server{
		Addr:              cfg.HTTPAddr,
		Handler:           router,
		ReadTimeout:       cfg.ShutdownTimeout,
		ReadHeaderTimeout: cfg.ShutdownTimeout,
		WriteTimeout:      cfg.ShutdownTimeout,
		IdleTimeout:       cfg.ShutdownTimeout,
	}

	return &Container{
		Server: server,
		pool:   pool,
	}, nil
}

func (c *Container) Close() {
	c.pool.Close()
}
