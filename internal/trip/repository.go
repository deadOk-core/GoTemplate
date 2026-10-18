package trip

import (
	"context"
	"errors"
	"fmt"
	"time"

	sq "github.com/Masterminds/squirrel"
	postgresdb "github.com/deadOk-core/GoTemplate/internal/postgres"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"
)

var ErrTripNotFound = errors.New("trip not found")

type Repository struct {
	pool         *pgxpool.Pool
	queryTimeout time.Duration
}

func NewRepository(pool *pgxpool.Pool, queryTimeout time.Duration) *Repository {
	return &Repository{
		pool:         pool,
		queryTimeout: queryTimeout,
	}
}

func (r *Repository) GetByID(ctx context.Context, id uuid.UUID) (Trip, error) {
	queryCtx, cancel := context.WithTimeout(ctx, r.queryTimeout)
	defer cancel()

	db := postgresdb.Executor(queryCtx, r.pool)

	query, args, err := sq.Select(
		"id",
		"user_id",
		"driver_id",
		"start_latitude",
		"start_longitude",
		"end_latitude",
		"end_longitude",
		"price",
		"status",
		"started_at",
		"finished_at",
		"created_at",
		"updated_at",
	).From("trips").Where(sq.Eq{"id": id}).PlaceholderFormat(sq.Dollar).ToSql()

	if err != nil {
		return Trip{}, fmt.Errorf("build get trip query: %w", err)
	}

	var res Trip

	err = db.QueryRow(queryCtx, query, args...).Scan(
		&res.ID,
		&res.UserID,
		&res.DriverID,
		&res.StartLatitude,
		&res.StartLongitude,
		&res.EndLatitude,
		&res.EndLongitude,
		&res.Price,
		&res.Status,
		&res.StartedAt,
		&res.FinishedAt,
		&res.CreatedAt,
		&res.UpdatedAt,
	)

	if errors.Is(err, pgx.ErrNoRows) {
		return Trip{}, ErrTripNotFound
	}

	if err != nil {
		return Trip{}, fmt.Errorf("get trip by id: %w", err)
	}

	return res, nil
}

var ErrDriverBusy = errors.New("driver already has an active trip")

func (r *Repository) Create(ctx context.Context, newTrip Trip) (Trip, error) {
	queryCtx, cancel := context.WithTimeout(ctx, r.queryTimeout)
	defer cancel()

	db := postgresdb.Executor(queryCtx, r.pool)

	query, args, err := sq.Insert("trips").Columns(
		"id",
		"user_id",
		"driver_id",
		"start_latitude",
		"start_longitude",
		"end_latitude",
		"end_longitude",
		"price",
		"status",
		"started_at",
		"finished_at",
	).Values(
		newTrip.ID,
		newTrip.UserID,
		newTrip.DriverID,
		newTrip.StartLatitude,
		newTrip.StartLongitude,
		newTrip.EndLatitude,
		newTrip.EndLongitude,
		newTrip.Price,
		newTrip.Status,
		newTrip.StartedAt,
		nil,
	).Suffix("RETURNING created_at, updated_at").PlaceholderFormat(sq.Dollar).ToSql()

	if err != nil {
		return Trip{}, fmt.Errorf("build create trip query: %w", err)
	}

	err = db.QueryRow(queryCtx, query, args...).Scan(
		&newTrip.CreatedAt,
		&newTrip.UpdatedAt,
	)

	if err != nil {
		var pgErr *pgconn.PgError

		if errors.As(err, &pgErr) && pgErr.Code == "23505" && pgErr.ConstraintName == "trips_one_active_trip_per_driver_idx" {
			return Trip{}, ErrDriverBusy
		}

		return Trip{}, fmt.Errorf("create trip: %w", err)
	}

	return newTrip, nil
}

func (r *Repository) AddStatusHistory(ctx context.Context, history StatusHistory) error {
	queryCtx, cancel := context.WithTimeout(ctx, r.queryTimeout)
	defer cancel()

	db := postgresdb.Executor(queryCtx, r.pool)

	query, args, err := sq.Insert("trip_status_history").Columns(
		"trip_id",
		"from_status",
		"to_status",
		"reason",
	).Values(
		history.TripID,
		history.FromStatus,
		history.ToStatus,
		history.Reason,
	).PlaceholderFormat(sq.Dollar).ToSql()

	if err != nil {
		return fmt.Errorf("build add status history query: %w", err)
	}

	if _, err := db.Exec(queryCtx, query, args...); err != nil {
		return fmt.Errorf("add status history: %w", err)
	}

	return nil
}

var ErrTripCompleted = errors.New("trip already completed")

func (r *Repository) Finish(ctx context.Context, id uuid.UUID) (Trip, error) {
	queryCtx, cancel := context.WithTimeout(ctx, r.queryTimeout)
	defer cancel()

	db := postgresdb.Executor(queryCtx, r.pool)

	query, args, err := sq.Update("trips").
		Set("status", StatusCompleted).Set("finished_at", sq.Expr("now()")).Set("updated_at", sq.Expr("now()")).
		Where(sq.Eq{"id": id, "status": StatusActive}).
		Suffix(`
		RETURNING
			id,
			user_id,
			driver_id,
			start_latitude,
			start_longitude,
			end_latitude,
			end_longitude,
			price,
			status,
			started_at,
			finished_at,
			created_at,
			updated_at
		`).PlaceholderFormat(sq.Dollar).ToSql()

	if err != nil {
		return Trip{}, fmt.Errorf("build finish trip query: %w", err)
	}

	var finishedTrip Trip

	err = db.QueryRow(queryCtx, query, args...).Scan(
		&finishedTrip.ID,
		&finishedTrip.UserID,
		&finishedTrip.DriverID,
		&finishedTrip.StartLatitude,
		&finishedTrip.StartLongitude,
		&finishedTrip.EndLatitude,
		&finishedTrip.EndLongitude,
		&finishedTrip.Price,
		&finishedTrip.Status,
		&finishedTrip.StartedAt,
		&finishedTrip.FinishedAt,
		&finishedTrip.CreatedAt,
		&finishedTrip.UpdatedAt,
	)
	if err == nil {
		return finishedTrip, nil
	}

	if !errors.Is(err, pgx.ErrNoRows) {
		return Trip{}, fmt.Errorf("finish trip: %w", err)
	}

	existingTrip, err := r.GetByID(queryCtx, id)
	if errors.Is(err, ErrTripNotFound) {
		return Trip{}, ErrTripNotFound
	}

	if err != nil {
		return Trip{}, fmt.Errorf("check trip after finish: %w", err)
	}

	if existingTrip.Status == StatusCompleted {
		return Trip{}, ErrTripCompleted
	}

	return Trip{}, fmt.Errorf("finish trip: unexpected trip status %q", existingTrip.Status)
}
