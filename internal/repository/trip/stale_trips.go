package trip

import (
	"context"
	"time"

	"github.com/Masterminds/squirrel"

	"github.com/XrystikLK/template/internal/infra/postgres/transaction"
	"github.com/XrystikLK/template/internal/model"
)

func (r *Repository) GetStaleTrips(ctx context.Context, staleAfter time.Duration) ([]model.StaleTrip, error) {
	ctx, cancel := context.WithTimeout(ctx, r.queryTimeout)
	defer cancel()

	var exec executor = transaction.GetExecutor(ctx, r.pool)

	staleMicros := staleAfter.Microseconds()
	query, args, err := r.sq.Select("id", "driver_id", "started_at", "last_position_at").
		From("trips").
		Where("status = 'active'").
		Where(squirrel.Expr("COALESCE(last_position_at, started_at) < now() - (? * interval '1 microsecond')", staleMicros)).
		OrderBy("COALESCE(last_position_at, started_at) ASC").
		ToSql()
	if err != nil {
		return nil, err
	}

	rows, err := exec.Query(ctx, query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	trips := make([]model.StaleTrip, 0)
	for rows.Next() {
		var t model.StaleTrip
		if err := rows.Scan(&t.ID, &t.DriverID, &t.StartedAt, &t.LastPositionAt); err != nil {
			return nil, err
		}
		trips = append(trips, t)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}

	return trips, nil
}
