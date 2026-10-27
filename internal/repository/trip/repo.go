package trip

import (
	"context"
	"errors"
	"fmt"
	"time"

	sq "github.com/Masterminds/squirrel"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"

	domaintrip "github.com/deadOk-core/GoTemplate/internal/domain/trip"
	"github.com/deadOk-core/GoTemplate/internal/infra/postgres/transaction"
	"github.com/deadOk-core/GoTemplate/internal/model"
)

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

func (r *Repository) GetByID(ctx context.Context, id uuid.UUID) (model.Trip, error) {
	queryCtx, cancel := context.WithTimeout(ctx, r.queryTimeout)
	defer cancel()

	db := transaction.Executor(queryCtx, r.pool)

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
		return model.Trip{}, fmt.Errorf("build get trip query: %w", err)
	}

	var res model.Trip

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
		return model.Trip{}, domaintrip.ErrTripNotFound
	}

	if err != nil {
		return model.Trip{}, fmt.Errorf("get trip by id: %w", err)
	}

	return res, nil
}

func (r *Repository) Create(ctx context.Context, newTrip model.Trip) (model.Trip, error) {
	queryCtx, cancel := context.WithTimeout(ctx, r.queryTimeout)
	defer cancel()

	db := transaction.Executor(queryCtx, r.pool)

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
		return model.Trip{}, fmt.Errorf("build create trip query: %w", err)
	}

	err = db.QueryRow(queryCtx, query, args...).Scan(
		&newTrip.CreatedAt,
		&newTrip.UpdatedAt,
	)
	if err != nil {
		var pgErr *pgconn.PgError

		if errors.As(err, &pgErr) && pgErr.Code == "23505" && pgErr.ConstraintName == "trips_one_active_trip_per_driver_idx" {
			return model.Trip{}, domaintrip.ErrDriverBusy
		}

		return model.Trip{}, fmt.Errorf("create trip: %w", err)
	}

	return newTrip, nil
}

func (r *Repository) AddStatusHistory(ctx context.Context, history model.StatusHistory) error {
	queryCtx, cancel := context.WithTimeout(ctx, r.queryTimeout)
	defer cancel()

	db := transaction.Executor(queryCtx, r.pool)

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

func (r *Repository) Finish(ctx context.Context, id uuid.UUID) (model.Trip, error) {
	queryCtx, cancel := context.WithTimeout(ctx, r.queryTimeout)
	defer cancel()

	db := transaction.Executor(queryCtx, r.pool)

	query, args, err := sq.Update("trips").
		Set("status", domaintrip.StatusCompleted).Set("finished_at", sq.Expr("now()")).Set("updated_at", sq.Expr("now()")).
		Where(sq.Eq{"id": id, "status": domaintrip.StatusActive}).
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
		return model.Trip{}, fmt.Errorf("build finish trip query: %w", err)
	}

	var finishedTrip model.Trip

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
		return model.Trip{}, fmt.Errorf("finish trip: %w", err)
	}

	existingTrip, err := r.GetByID(queryCtx, id)
	if errors.Is(err, domaintrip.ErrTripNotFound) {
		return model.Trip{}, domaintrip.ErrTripNotFound
	}

	if err != nil {
		return model.Trip{}, fmt.Errorf("check trip after finish: %w", err)
	}

	if existingTrip.Status == domaintrip.StatusCompleted {
		return model.Trip{}, domaintrip.ErrTripCompleted
	}

	return model.Trip{}, fmt.Errorf("finish trip: unexpected trip status %q", existingTrip.Status)
}
