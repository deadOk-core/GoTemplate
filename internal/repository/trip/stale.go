package trip

import (
	"context"
	"fmt"
	"time"

	sq "github.com/Masterminds/squirrel"
	"github.com/jackc/pgx/v5/pgtype"

	"github.com/deadOk-core/GoTemplate/internal/model"
)

func (r *Repository) ListStaleTrips(ctx context.Context, staleAfter time.Duration) ([]model.StaleTrip, error) {
	queryCtx, cancel := context.WithTimeout(ctx, r.queryTimeout)
	defer cancel()

	const activityTime = "COALESCE(last_position_at, started_at)"
	interval := pgtype.Interval{Microseconds: staleAfter.Microseconds(), Valid: true}
	query, args, err := sq.Select("id", "driver_id").From("trips").
		Where("status = 'active'").
		Where(activityTime+" < now() - ?::interval", interval).
		OrderBy(activityTime, "id").PlaceholderFormat(sq.Dollar).ToSql()
	if err != nil {
		return nil, fmt.Errorf("build stale trips query: %w", err)
	}

	rows, err := r.pool.Query(queryCtx, query, args...)
	if err != nil {
		return nil, fmt.Errorf("query stale trips: %w", err)
	}
	defer rows.Close()

	var trips []model.StaleTrip
	for rows.Next() {
		var trip model.StaleTrip
		if err := rows.Scan(&trip.ID, &trip.DriverID); err != nil {
			return nil, fmt.Errorf("scan stale trip: %w", err)
		}
		trips = append(trips, trip)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate stale trips: %w", err)
	}
	return trips, nil
}
