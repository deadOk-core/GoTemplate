package container

import (
	"context"
	"fmt"
	"log/slog"
	"net/http"
	"os"

	"github.com/go-chi/chi/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/metric"

	"github.com/deadOk-core/GoTemplate/internal/config"
	"github.com/deadOk-core/GoTemplate/internal/gateway/push"
	api "github.com/deadOk-core/GoTemplate/internal/generated"
	httpapi "github.com/deadOk-core/GoTemplate/internal/http"
	httpmiddleware "github.com/deadOk-core/GoTemplate/internal/http/middleware"
	createhandler "github.com/deadOk-core/GoTemplate/internal/http/trip/create"
	createpositionhandler "github.com/deadOk-core/GoTemplate/internal/http/trip/createposition"
	finishhandler "github.com/deadOk-core/GoTemplate/internal/http/trip/finish"
	gethandler "github.com/deadOk-core/GoTemplate/internal/http/trip/get"
	listpositionshandler "github.com/deadOk-core/GoTemplate/internal/http/trip/listpositions"
	"github.com/deadOk-core/GoTemplate/internal/infra/postgres"
	"github.com/deadOk-core/GoTemplate/internal/infra/postgres/transaction"
	"github.com/deadOk-core/GoTemplate/internal/infra/telemetry"
	triprepository "github.com/deadOk-core/GoTemplate/internal/repository/trip"
	createusecase "github.com/deadOk-core/GoTemplate/internal/usecase/trip/create"
	createpositionusecase "github.com/deadOk-core/GoTemplate/internal/usecase/trip/createposition"
	finishusecase "github.com/deadOk-core/GoTemplate/internal/usecase/trip/finish"
	getusecase "github.com/deadOk-core/GoTemplate/internal/usecase/trip/get"
	listpositionsusecase "github.com/deadOk-core/GoTemplate/internal/usecase/trip/listpositions"
	"github.com/deadOk-core/GoTemplate/internal/worker/positions"
)

type Container struct {
	Server            *http.Server
	pool              *pgxpool.Pool
	Logger            *slog.Logger
	shutdownTelemetry func(context.Context) error
	pushGateway       *push.Gateway
	stopScanner       context.CancelFunc
	stopWorkers       context.CancelFunc
	positionsDone     <-chan struct{}
}

func NewContainer(ctx context.Context, cfg config.Config) (*Container, error) {
	logger := telemetry.NewLogger(
		os.Stdout,
		cfg.LogLevel,
		cfg.ServiceName,
	)

	shutdownTelemetry, err := telemetry.Setup(ctx, cfg, logger)
	if err != nil {
		return nil, fmt.Errorf("setup telemetry: %w", err)
	}
	var pool *pgxpool.Pool
	initialized := false
	defer func() {
		if !initialized {
			cleanupCtx, cancel := context.WithTimeout(context.WithoutCancel(ctx), cfg.ShutdownTimeout)
			defer cancel()
			if err := shutdownTelemetry(cleanupCtx); err != nil {
				logger.WarnContext(cleanupCtx, "cleanup telemetry", "error", err)
			}
			if pool != nil {
				pool.Close()
			}
		}
	}()
	pool, err = postgres.NewPool(ctx, cfg.Database)
	if err != nil {
		return nil, fmt.Errorf("create postgres pool: %w", err)
	}
	meter := otel.Meter("trip-service/trips")
	created, err := meter.Int64Counter("tripgo.trips.created", metric.WithUnit("{trip}"))
	if err != nil {
		return nil, fmt.Errorf("create trips created metric: %w", err)
	}
	completed, err := meter.Int64Counter("tripgo.trips.completed", metric.WithUnit("{trip}"))
	if err != nil {
		return nil, fmt.Errorf("create trips completed metric: %w", err)
	}
	created.Add(ctx, 0)
	completed.Add(ctx, 0)
	repository := triprepository.NewRepository(pool, cfg.Database.QueryTimeout)
	if err := telemetry.RegisterActiveTrips(repository, meter); err != nil {
		return nil, err
	}

	pushGateway := push.NewGateway(cfg.Push.HTTPURL, cfg.Push.RequestTimeout)
	positionsMeter := otel.Meter("trip-service/positions")
	savedPositions, err := positionsMeter.Int64Counter("tripgo.positions.saved", metric.WithUnit("{position}"))
	if err != nil {
		return nil, fmt.Errorf("create positions saved metric: %w", err)
	}
	savedPositions.Add(ctx, 0)
	requester, err := positions.NewRequester(pushGateway, logger, positionsMeter)
	if err != nil {
		return nil, err
	}
	positionPool, err := positions.NewPool(repository, requester, logger, positionsMeter,
		otel.Tracer("trip-service/positions"), cfg.Positions, cfg.Push.RequestTimeout)
	if err != nil {
		return nil, err
	}

	txManager := transaction.NewTransactionManager(pool)

	createUseCase := createusecase.NewUseCase(repository, txManager, logger, created)
	getUseCase := getusecase.NewUseCase(repository)
	finishUseCase := finishusecase.NewUseCase(repository, txManager, logger, completed)
	createPositionUseCase := createpositionusecase.NewUseCase(repository, txManager, positionPool, savedPositions)
	listPositionsUseCase := listpositionsusecase.NewUseCase(repository)

	createHandler := createhandler.NewHandler(createUseCase, logger)
	getHandler := gethandler.NewHandler(getUseCase, logger)
	finishHandler := finishhandler.NewHandler(finishUseCase, logger)
	createPositionHandler := createpositionhandler.NewHandler(createPositionUseCase)
	listPositionsHandler := listpositionshandler.NewHandler(listPositionsUseCase)

	handler := httpapi.NewHandler(
		createHandler,
		getHandler,
		finishHandler,
		createPositionHandler,
		listPositionsHandler,
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
	middleware, err := httpmiddleware.New(router, logger, otel.GetMeterProvider(), otel.GetTracerProvider())
	if err != nil {
		return nil, err
	}

	server := &http.Server{
		Addr:              cfg.HTTPAddr,
		Handler:           middleware.Handler(),
		ReadTimeout:       cfg.ShutdownTimeout,
		ReadHeaderTimeout: cfg.ShutdownTimeout,
		WriteTimeout:      cfg.ShutdownTimeout,
		IdleTimeout:       cfg.ShutdownTimeout,
		ErrorLog:          slog.NewLogLogger(logger.Handler(), slog.LevelError),
	}

	pollingCtx := context.WithoutCancel(ctx)
	scanCtx, stopScanner := context.WithCancel(pollingCtx)
	workerCtx, stopWorkers := context.WithCancel(pollingCtx)
	positionsDone := make(chan struct{})
	go func() {
		defer close(positionsDone)
		positionPool.Run(scanCtx, workerCtx)
	}()

	initialized = true
	return &Container{
		Server:            server,
		pool:              pool,
		Logger:            logger,
		shutdownTelemetry: shutdownTelemetry,
		pushGateway:       pushGateway,
		stopScanner:       stopScanner,
		stopWorkers:       stopWorkers,
		positionsDone:     positionsDone,
	}, nil
}

func (c *Container) Close(ctx context.Context) error {
	c.stopScanner()
	defer c.stopWorkers()
	select {
	case <-c.positionsDone:
	case <-ctx.Done():
		c.stopWorkers()
		<-c.positionsDone
	}
	c.pushGateway.CloseIdleConnections()

	// The final metric collection still needs the pool for the active trips gauge.
	defer c.pool.Close()
	if err := c.shutdownTelemetry(ctx); err != nil {
		return fmt.Errorf("shutdown telemetry: %w", err)
	}
	return ctx.Err()
}
