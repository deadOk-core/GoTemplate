package trip

import (
	"context"
	"fmt"
	"time"

	sq "github.com/Masterminds/squirrel"
	"github.com/google/uuid"

	domaintrip "github.com/deadOk-core/GoTemplate/internal/domain/trip"
	"github.com/deadOk-core/GoTemplate/internal/infra/postgres/transaction"
	"github.com/deadOk-core/GoTemplate/internal/model"
)

func (r *Repository) UpdateLastPosition(ctx context.Context, tripID uuid.UUID, recordedAt time.Time) error {
	queryCtx, cancel := context.WithTimeout(ctx, r.queryTimeout)
	defer cancel()

	query, args, err := sq.Update("trips").
		Set("last_position_at", sq.Expr("GREATEST(last_position_at, ?)", recordedAt)).
		Where(sq.Eq{"id": tripID, tripStatusColumn: domaintrip.StatusActive}).
		PlaceholderFormat(sq.Dollar).ToSql()
	if err != nil {
		return fmt.Errorf("build update last position query: %w", err)
	}

	db := transaction.Executor(queryCtx, r.pool)
	result, err := db.Exec(queryCtx, query, args...)
	if err != nil {
		return fmt.Errorf("update last position: %w", err)
	}
	if result.RowsAffected() != 0 {
		return nil
	}

	trip, err := r.GetByID(queryCtx, tripID)
	if err != nil {
		return fmt.Errorf("check trip after position update: %w", err)
	}
	if trip.Status == domaintrip.StatusCompleted {
		return domaintrip.ErrTripCompleted
	}
	return fmt.Errorf("update last position: unexpected trip status %q", trip.Status)
}

func (r *Repository) InsertPosition(ctx context.Context, position model.TripPosition) (model.TripPosition, error) {
	queryCtx, cancel := context.WithTimeout(ctx, r.queryTimeout)
	defer cancel()

	query, args, err := sq.Insert("trip_positions").
		Columns("trip_id", "latitude", "longitude", "recorded_at").
		Values(position.TripID, position.Latitude, position.Longitude, position.RecordedAt).
		Suffix("RETURNING id, recorded_at").PlaceholderFormat(sq.Dollar).ToSql()
	if err != nil {
		return model.TripPosition{}, fmt.Errorf("build insert position query: %w", err)
	}

	db := transaction.Executor(queryCtx, r.pool)
	if err := db.QueryRow(queryCtx, query, args...).Scan(&position.ID, &position.RecordedAt); err != nil {
		return model.TripPosition{}, fmt.Errorf("insert position: %w", err)
	}
	return position, nil
}

func (r *Repository) ListPositions(ctx context.Context, tripID uuid.UUID) ([]model.TripPosition, error) {
	queryCtx, cancel := context.WithTimeout(ctx, r.queryTimeout)
	defer cancel()

	query, args, err := sq.Select("id", "latitude", "longitude", "recorded_at").From("trip_positions").
		Where(sq.Eq{"trip_id": tripID}).OrderBy("recorded_at", "id").PlaceholderFormat(sq.Dollar).ToSql()
	if err != nil {
		return nil, fmt.Errorf("build list positions query: %w", err)
	}

	rows, err := r.pool.Query(queryCtx, query, args...)
	if err != nil {
		return nil, fmt.Errorf("query trip positions: %w", err)
	}
	defer rows.Close()

	positions := make([]model.TripPosition, 0)
	for rows.Next() {
		position := model.TripPosition{TripID: tripID}
		if err := rows.Scan(&position.ID, &position.Latitude, &position.Longitude, &position.RecordedAt); err != nil {
			return nil, fmt.Errorf("scan trip position: %w", err)
		}
		positions = append(positions, position)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate trip positions: %w", err)
	}
	return positions, nil
}
