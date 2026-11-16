package trip

import (
	"context"
	"errors"
	"time"

	"github.com/Masterminds/squirrel"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"

	domainTrip "github.com/XrystikLK/template/internal/domain/trip"
	"github.com/XrystikLK/template/internal/infra/postgres/transaction"
	"github.com/XrystikLK/template/internal/model"
)

func (r *Repository) SavePosition(ctx context.Context, tripID uuid.UUID, pos model.Position) (*model.Position, error) {
	ctx, cancel := context.WithTimeout(ctx, r.queryTimeout)
	defer cancel()

	var exec executor = transaction.GetExecutor(ctx, r.pool)

	updateQuery, updateArgs, err := r.sq.Update("trips").
		Set("last_position_at", squirrel.Expr("COALESCE(GREATEST(last_position_at, ?), ?)", pos.RecordedAt, pos.RecordedAt)).
		Set("updated_at", time.Now().UTC()).
		Where(squirrel.Eq{"id": tripID, statusColumn: string(model.StatusActive)}).
		ToSql()
	if err != nil {
		return nil, err
	}

	tag, err := exec.Exec(ctx, updateQuery, updateArgs...)
	if err != nil {
		return nil, err
	}

	if tag.RowsAffected() == 0 {
		checkQuery, checkArgs, checkErr := r.sq.Select(statusColumn).From("trips").Where(squirrel.Eq{"id": tripID}).ToSql()
		if checkErr != nil {
			return nil, checkErr
		}

		var currentStatus string
		checkErr = exec.QueryRow(ctx, checkQuery, checkArgs...).Scan(&currentStatus)
		if errors.Is(checkErr, pgx.ErrNoRows) {
			return nil, domainTrip.ErrTripNotFound
		}
		if currentStatus == string(model.StatusCompleted) {
			return nil, domainTrip.ErrTripCompleted
		}
		if checkErr != nil {
			return nil, checkErr
		}
	}

	createdAt := time.Now().UTC()
	insertQuery, insertArgs, err := r.sq.Insert("trip_positions").
		Columns("trip_id", "latitude", "longitude", "recorded_at", "created_at").
		Values(tripID, pos.Coordinates.Latitude, pos.Coordinates.Longitude, pos.RecordedAt, createdAt).
		Suffix("RETURNING id, created_at").
		ToSql()
	if err != nil {
		return nil, err
	}

	var (
		id          int64
		dbCreatedAt time.Time
	)
	err = exec.QueryRow(ctx, insertQuery, insertArgs...).Scan(&id, &dbCreatedAt)
	if err != nil {
		return nil, err
	}

	result := pos
	result.ID = id
	result.TripID = tripID
	result.CreatedAt = dbCreatedAt
	return &result, nil
}
