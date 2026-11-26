package trip

import (
	"context"
	"errors"

	"github.com/Masterminds/squirrel"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"

	domainTrip "github.com/XrystikLK/template/internal/domain/trip"
	"github.com/XrystikLK/template/internal/infra/postgres/transaction"
	"github.com/XrystikLK/template/internal/model"
)

func (r *Repository) ListPositions(ctx context.Context, tripID uuid.UUID) ([]model.Position, error) {
	ctx, cancel := context.WithTimeout(ctx, r.queryTimeout)
	defer cancel()

	var exec executor = transaction.GetExecutor(ctx, r.pool)

	checkQuery, checkArgs, err := r.sq.Select("id").From("trips").Where(squirrel.Eq{"id": tripID}).ToSql()
	if err != nil {
		return nil, err
	}

	var dummy uuid.UUID
	if err := exec.QueryRow(ctx, checkQuery, checkArgs...).Scan(&dummy); err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, domainTrip.ErrTripNotFound
		}
		return nil, err
	}

	query, args, err := r.sq.Select("id", "trip_id", "latitude", "longitude", "recorded_at", "created_at").
		From("trip_positions").
		Where(squirrel.Eq{"trip_id": tripID}).
		OrderBy("recorded_at ASC", "id ASC").
		ToSql()
	if err != nil {
		return nil, err
	}

	rows, err := exec.Query(ctx, query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	positions := make([]model.Position, 0)
	for rows.Next() {
		var p model.Position
		if err := rows.Scan(&p.ID, &p.TripID, &p.Coordinates.Latitude, &p.Coordinates.Longitude, &p.RecordedAt, &p.CreatedAt); err != nil {
			return nil, err
		}
		positions = append(positions, p)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}

	return positions, nil
}
