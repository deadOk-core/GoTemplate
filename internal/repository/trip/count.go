package trip

import (
	"context"
	"fmt"

	sq "github.com/Masterminds/squirrel"

	domaintrip "github.com/deadOk-core/GoTemplate/internal/domain/trip"
)

func (r *Repository) CountActive(ctx context.Context) (int64, error) {
	queryCtx, cancel := context.WithTimeout(ctx, r.queryTimeout)
	defer cancel()
	query, args, err := sq.Select("count(*)").From("trips").
		Where(sq.Eq{"status": domaintrip.StatusActive}).PlaceholderFormat(sq.Dollar).ToSql()
	if err != nil {
		return 0, fmt.Errorf("build active trips count query: %w", err)
	}
	var count int64
	if err := r.pool.QueryRow(queryCtx, query, args...).Scan(&count); err != nil {
		return 0, fmt.Errorf("count active trips: %w", err)
	}
	return count, nil
}
