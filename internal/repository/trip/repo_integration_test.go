//go:build integration

package trip_test

import (
	"context"
	"errors"
	"os"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/joho/godotenv"

	domainTrip "github.com/XrystikLK/template/internal/domain/trip"
	"github.com/XrystikLK/template/internal/infra/postgres/transaction"
	"github.com/XrystikLK/template/internal/model"
	repoTrip "github.com/XrystikLK/template/internal/repository/trip"
)

func setupTestDB(t *testing.T) (*pgxpool.Pool, *repoTrip.Repository, *transaction.Manager) {
	t.Helper()
	_ = godotenv.Load("../../../.env")

	dbURL := os.Getenv("DATABASE_URL")
	if dbURL == "" {
		//nolint:gosec // default test database connection string for local integration tests
		dbURL = "postgres://tripgo:tripgo@localhost:22032/tripgo?sslmode=disable"
	}

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	pool, err := pgxpool.New(ctx, dbURL)
	if err != nil {
		t.Fatalf("failed to connect to test db: %v", err)
	}

	if err := pool.Ping(ctx); err != nil {
		t.Fatalf("failed to ping test db: %v", err)
	}

	t.Cleanup(func() {
		pool.Close()
	})

	repo := repoTrip.New(pool, 3*time.Second)
	txManager := transaction.NewManager(pool)
	return pool, repo, txManager
}

func cleanupTrip(t *testing.T, pool *pgxpool.Pool, tripID uuid.UUID) {
	t.Helper()
	t.Cleanup(func() {
		ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
		defer cancel()
		_, _ = pool.Exec(ctx, "DELETE FROM trip_positions WHERE trip_id = $1", tripID)
		_, _ = pool.Exec(ctx, "DELETE FROM trip_status_history WHERE trip_id = $1", tripID)
		_, _ = pool.Exec(ctx, "DELETE FROM trips WHERE id = $1", tripID)
	})
}

func TestIntegrationSaveAndGetTrip(t *testing.T) {
	pool, repo, _ := setupTestDB(t)

	tripID := uuid.New()
	cleanupTrip(t, pool, tripID)

	now := time.Now().UTC().Truncate(time.Microsecond)
	tr := model.Trip{
		ID:         tripID,
		UserID:     uuid.New(),
		DriverID:   uuid.New(),
		StartPoint: model.Coordinates{Latitude: 55.75, Longitude: 37.61},
		EndPoint:   model.Coordinates{Latitude: 55.80, Longitude: 37.70},
		Price:      450,
		Status:     model.StatusActive,
		StartedAt:  now,
		CreatedAt:  now,
		UpdatedAt:  now,
	}

	ctx := context.Background()
	if err := repo.Save(ctx, tr); err != nil {
		t.Fatalf("failed to save trip: %v", err)
	}

	fetched, err := repo.GetByID(ctx, tripID)
	if err != nil {
		t.Fatalf("failed to fetch trip: %v", err)
	}

	if fetched.ID != tr.ID || fetched.Price != tr.Price || fetched.Status != tr.Status {
		t.Fatalf("fetched trip mismatch: got %+v, want %+v", fetched, tr)
	}
}

func TestIntegrationSaveDuplicateDriver(t *testing.T) {
	pool, repo, _ := setupTestDB(t)

	driverID := uuid.New()
	trip1ID := uuid.New()
	trip2ID := uuid.New()

	cleanupTrip(t, pool, trip1ID)
	cleanupTrip(t, pool, trip2ID)

	now := time.Now().UTC()
	tr1 := model.Trip{
		ID:         trip1ID,
		UserID:     uuid.New(),
		DriverID:   driverID,
		StartPoint: model.Coordinates{Latitude: 55.75, Longitude: 37.61},
		EndPoint:   model.Coordinates{Latitude: 55.80, Longitude: 37.70},
		Price:      300,
		Status:     model.StatusActive,
		StartedAt:  now,
		CreatedAt:  now,
		UpdatedAt:  now,
	}

	ctx := context.Background()
	if err := repo.Save(ctx, tr1); err != nil {
		t.Fatalf("failed to save first trip: %v", err)
	}

	tr2 := tr1
	tr2.ID = trip2ID
	tr2.UserID = uuid.New()

	err := repo.Save(ctx, tr2)
	if !errors.Is(err, domainTrip.ErrDriverBusy) {
		t.Fatalf("expected ErrDriverBusy, got: %v", err)
	}
}

func TestIntegrationFinishTrip(t *testing.T) {
	pool, repo, _ := setupTestDB(t)

	tripID := uuid.New()
	cleanupTrip(t, pool, tripID)

	now := time.Now().UTC().Truncate(time.Microsecond)
	tr := model.Trip{
		ID:         tripID,
		UserID:     uuid.New(),
		DriverID:   uuid.New(),
		StartPoint: model.Coordinates{Latitude: 55.75, Longitude: 37.61},
		EndPoint:   model.Coordinates{Latitude: 55.80, Longitude: 37.70},
		Price:      700,
		Status:     model.StatusActive,
		StartedAt:  now,
		CreatedAt:  now,
		UpdatedAt:  now,
	}

	ctx := context.Background()
	if err := repo.Save(ctx, tr); err != nil {
		t.Fatalf("failed to save trip: %v", err)
	}

	finishedAt := now.Add(15 * time.Minute)
	finished, err := repo.Finish(ctx, tripID, finishedAt)
	if err != nil {
		t.Fatalf("failed to finish trip: %v", err)
	}
	if finished.Status != model.StatusCompleted {
		t.Fatalf("expected completed status, got: %v", finished.Status)
	}
	if finished.FinishedAt == nil || !finished.FinishedAt.Equal(finishedAt) {
		t.Fatalf("expected finishedAt %v, got %v", finishedAt, finished.FinishedAt)
	}

	_, err = repo.Finish(ctx, tripID, finishedAt)
	if !errors.Is(err, domainTrip.ErrTripCompleted) {
		t.Fatalf("expected ErrTripCompleted on repeated finish, got: %v", err)
	}
}

func TestIntegrationGetAndFinishNotFound(t *testing.T) {
	_, repo, _ := setupTestDB(t)
	nonExistentID := uuid.New()
	ctx := context.Background()

	_, err := repo.GetByID(ctx, nonExistentID)
	if !errors.Is(err, domainTrip.ErrTripNotFound) {
		t.Fatalf("expected ErrTripNotFound on get, got: %v", err)
	}

	_, err = repo.Finish(ctx, nonExistentID, time.Now().UTC())
	if !errors.Is(err, domainTrip.ErrTripNotFound) {
		t.Fatalf("expected ErrTripNotFound on finish, got: %v", err)
	}
}

func TestIntegrationTransactionRollback(t *testing.T) {
	pool, repo, txManager := setupTestDB(t)

	tripID := uuid.New()
	cleanupTrip(t, pool, tripID)

	now := time.Now().UTC()
	tr := model.Trip{
		ID:         tripID,
		UserID:     uuid.New(),
		DriverID:   uuid.New(),
		StartPoint: model.Coordinates{Latitude: 55.75, Longitude: 37.61},
		EndPoint:   model.Coordinates{Latitude: 55.80, Longitude: 37.70},
		Price:      1000,
		Status:     model.StatusActive,
		StartedAt:  now,
		CreatedAt:  now,
		UpdatedAt:  now,
	}

	expectedErr := errors.New("simulated error during transaction")

	ctx := context.Background()
	err := txManager.Do(ctx, func(txCtx context.Context) error {
		if err := repo.Save(txCtx, tr); err != nil {
			return err
		}

		history := model.StatusHistory{
			TripID:     tripID,
			FromStatus: nil,
			ToStatus:   model.StatusActive,
			ChangedAt:  now,
		}
		if err := repo.AddStatusHistory(txCtx, history); err != nil {
			return err
		}

		return expectedErr
	})

	if !errors.Is(err, expectedErr) {
		t.Fatalf("expected expectedErr, got: %v", err)
	}

	_, err = repo.GetByID(ctx, tripID)
	if !errors.Is(err, domainTrip.ErrTripNotFound) {
		t.Fatalf("expected trip to not exist after rollback, but got error: %v", err)
	}

	var count int
	row := pool.QueryRow(ctx, "SELECT COUNT(*) FROM trip_status_history WHERE trip_id = $1", tripID)
	if scanErr := row.Scan(&count); scanErr != nil {
		t.Fatalf("failed to query status history count: %v", scanErr)
	}
	if count != 0 {
		t.Fatalf("expected 0 status history rows after rollback, got %d", count)
	}
}

func TestIntegrationSaveAndListPositions(t *testing.T) {
	pool, repo, _ := setupTestDB(t)

	tripID := uuid.New()
	cleanupTrip(t, pool, tripID)

	now := time.Now().UTC().Truncate(time.Microsecond)
	tr := model.Trip{
		ID:         tripID,
		UserID:     uuid.New(),
		DriverID:   uuid.New(),
		StartPoint: model.Coordinates{Latitude: 55.75, Longitude: 37.61},
		EndPoint:   model.Coordinates{Latitude: 55.80, Longitude: 37.70},
		Price:      500,
		Status:     model.StatusActive,
		StartedAt:  now,
		CreatedAt:  now,
		UpdatedAt:  now,
	}

	ctx := context.Background()
	if err := repo.Save(ctx, tr); err != nil {
		t.Fatalf("failed to save trip: %v", err)
	}

	pos1 := model.Position{
		Coordinates: model.Coordinates{Latitude: 55.76, Longitude: 37.62},
		RecordedAt:  now.Add(10 * time.Second),
	}
	saved1, err := repo.SavePosition(ctx, tripID, pos1)
	if err != nil {
		t.Fatalf("failed to save pos1: %v", err)
	}
	if saved1.ID == 0 {
		t.Fatalf("expected generated ID for pos1, got 0")
	}

	pos2 := model.Position{
		Coordinates: model.Coordinates{Latitude: 55.77, Longitude: 37.63},
		RecordedAt:  now.Add(20 * time.Second),
	}
	saved2, err := repo.SavePosition(ctx, tripID, pos2)
	if err != nil {
		t.Fatalf("failed to save pos2: %v", err)
	}

	list, err := repo.ListPositions(ctx, tripID)
	if err != nil {
		t.Fatalf("failed to list positions: %v", err)
	}
	if len(list) != 2 {
		t.Fatalf("expected 2 positions, got %d", len(list))
	}
	if list[0].ID != saved1.ID || list[1].ID != saved2.ID {
		t.Fatalf("positions not sorted correctly: %+v", list)
	}

	fetchedTrip, err := repo.GetByID(ctx, tripID)
	if err != nil {
		t.Fatalf("failed to get trip: %v", err)
	}
	if fetchedTrip.LastPositionAt == nil || !fetchedTrip.LastPositionAt.Equal(pos2.RecordedAt) {
		t.Fatalf("expected last_position_at to be %v, got %v", pos2.RecordedAt, fetchedTrip.LastPositionAt)
	}
}

func TestIntegrationSavePositionOlderDoesNotRevertLastPositionAt(t *testing.T) {
	pool, repo, _ := setupTestDB(t)

	tripID := uuid.New()
	cleanupTrip(t, pool, tripID)

	now := time.Now().UTC().Truncate(time.Microsecond)
	tr := model.Trip{
		ID:         tripID,
		UserID:     uuid.New(),
		DriverID:   uuid.New(),
		StartPoint: model.Coordinates{Latitude: 55.75, Longitude: 37.61},
		EndPoint:   model.Coordinates{Latitude: 55.80, Longitude: 37.70},
		Price:      500,
		Status:     model.StatusActive,
		StartedAt:  now,
		CreatedAt:  now,
		UpdatedAt:  now,
	}

	ctx := context.Background()
	if err := repo.Save(ctx, tr); err != nil {
		t.Fatalf("failed to save trip: %v", err)
	}

	newerPos := model.Position{
		Coordinates: model.Coordinates{Latitude: 55.76, Longitude: 37.62},
		RecordedAt:  now.Add(20 * time.Second),
	}
	if _, err := repo.SavePosition(ctx, tripID, newerPos); err != nil {
		t.Fatalf("failed to save newer pos: %v", err)
	}

	olderPos := model.Position{
		Coordinates: model.Coordinates{Latitude: 55.755, Longitude: 37.615},
		RecordedAt:  now.Add(10 * time.Second),
	}
	if _, err := repo.SavePosition(ctx, tripID, olderPos); err != nil {
		t.Fatalf("failed to save older pos: %v", err)
	}

	fetchedTrip, err := repo.GetByID(ctx, tripID)
	if err != nil {
		t.Fatalf("failed to get trip: %v", err)
	}
	if fetchedTrip.LastPositionAt == nil || !fetchedTrip.LastPositionAt.Equal(newerPos.RecordedAt) {
		t.Fatalf("last_position_at was reverted! got %v, want %v", fetchedTrip.LastPositionAt, newerPos.RecordedAt)
	}
}

func TestIntegrationSavePositionTripStatusValidation(t *testing.T) {
	pool, repo, _ := setupTestDB(t)

	nonExistentID := uuid.New()
	ctx := context.Background()

	pos := model.Position{
		Coordinates: model.Coordinates{Latitude: 55.75, Longitude: 37.61},
		RecordedAt:  time.Now().UTC(),
	}

	_, err := repo.SavePosition(ctx, nonExistentID, pos)
	if !errors.Is(err, domainTrip.ErrTripNotFound) {
		t.Fatalf("expected ErrTripNotFound for non-existent trip, got: %v", err)
	}

	_, err = repo.ListPositions(ctx, nonExistentID)
	if !errors.Is(err, domainTrip.ErrTripNotFound) {
		t.Fatalf("expected ErrTripNotFound for non-existent trip in list, got: %v", err)
	}

	tripID := uuid.New()
	cleanupTrip(t, pool, tripID)

	now := time.Now().UTC().Truncate(time.Microsecond)
	tr := model.Trip{
		ID:         tripID,
		UserID:     uuid.New(),
		DriverID:   uuid.New(),
		StartPoint: model.Coordinates{Latitude: 55.75, Longitude: 37.61},
		EndPoint:   model.Coordinates{Latitude: 55.80, Longitude: 37.70},
		Price:      500,
		Status:     model.StatusActive,
		StartedAt:  now,
		CreatedAt:  now,
		UpdatedAt:  now,
	}
	if err := repo.Save(ctx, tr); err != nil {
		t.Fatalf("failed to save trip: %v", err)
	}

	finishTime := now.Add(5 * time.Minute)
	if _, err := repo.Finish(ctx, tripID, finishTime); err != nil {
		t.Fatalf("failed to finish trip: %v", err)
	}

	_, err = repo.SavePosition(ctx, tripID, pos)
	if !errors.Is(err, domainTrip.ErrTripCompleted) {
		t.Fatalf("expected ErrTripCompleted for completed trip, got: %v", err)
	}
}

func TestIntegrationSavePositionTransactionRollback(t *testing.T) {
	pool, repo, txManager := setupTestDB(t)

	tripID := uuid.New()
	cleanupTrip(t, pool, tripID)

	now := time.Now().UTC().Truncate(time.Microsecond)
	tr := model.Trip{
		ID:         tripID,
		UserID:     uuid.New(),
		DriverID:   uuid.New(),
		StartPoint: model.Coordinates{Latitude: 55.75, Longitude: 37.61},
		EndPoint:   model.Coordinates{Latitude: 55.80, Longitude: 37.70},
		Price:      500,
		Status:     model.StatusActive,
		StartedAt:  now,
		CreatedAt:  now,
		UpdatedAt:  now,
	}

	ctx := context.Background()
	if err := repo.Save(ctx, tr); err != nil {
		t.Fatalf("failed to save trip: %v", err)
	}

	expectedErr := errors.New("simulated error during position save")
	err := txManager.Do(ctx, func(txCtx context.Context) error {
		pos := model.Position{
			Coordinates: model.Coordinates{Latitude: 55.76, Longitude: 37.62},
			RecordedAt:  now.Add(10 * time.Second),
		}
		if _, err := repo.SavePosition(txCtx, tripID, pos); err != nil {
			return err
		}
		return expectedErr
	})

	if !errors.Is(err, expectedErr) {
		t.Fatalf("expected expectedErr, got %v", err)
	}

	positions, err := repo.ListPositions(ctx, tripID)
	if err != nil {
		t.Fatalf("failed to list positions: %v", err)
	}
	if len(positions) != 0 {
		t.Fatalf("expected 0 positions after rollback, got %d", len(positions))
	}

	fetchedTrip, err := repo.GetByID(ctx, tripID)
	if err != nil {
		t.Fatalf("failed to get trip: %v", err)
	}
	if fetchedTrip.LastPositionAt != nil {
		t.Fatalf("expected last_position_at to remain nil, got %v", fetchedTrip.LastPositionAt)
	}
}

func TestIntegrationGetStaleTrips(t *testing.T) {
	pool, repo, _ := setupTestDB(t)

	staleTripID := uuid.New()
	cleanupTrip(t, pool, staleTripID)

	freshTripID := uuid.New()
	cleanupTrip(t, pool, freshTripID)

	completedTripID := uuid.New()
	cleanupTrip(t, pool, completedTripID)

	now := time.Now().UTC().Truncate(time.Microsecond)
	ctx := context.Background()

	staleTrip := model.Trip{
		ID:         staleTripID,
		UserID:     uuid.New(),
		DriverID:   uuid.New(),
		StartPoint: model.Coordinates{Latitude: 55.75, Longitude: 37.61},
		EndPoint:   model.Coordinates{Latitude: 55.80, Longitude: 37.70},
		Price:      500,
		Status:     model.StatusActive,
		StartedAt:  now.Add(-2 * time.Minute),
		CreatedAt:  now.Add(-2 * time.Minute),
		UpdatedAt:  now.Add(-2 * time.Minute),
	}
	if err := repo.Save(ctx, staleTrip); err != nil {
		t.Fatalf("failed to save stale trip: %v", err)
	}

	tiedTrip := staleTrip
	tiedTrip.ID = uuid.New()
	tiedTrip.DriverID = uuid.New()
	cleanupTrip(t, pool, tiedTrip.ID)
	if err := repo.Save(ctx, tiedTrip); err != nil {
		t.Fatalf("failed to save trip with matching activity time: %v", err)
	}

	freshTrip := model.Trip{
		ID:         freshTripID,
		UserID:     uuid.New(),
		DriverID:   uuid.New(),
		StartPoint: model.Coordinates{Latitude: 55.75, Longitude: 37.61},
		EndPoint:   model.Coordinates{Latitude: 55.80, Longitude: 37.70},
		Price:      500,
		Status:     model.StatusActive,
		StartedAt:  now.Add(-2 * time.Minute),
		CreatedAt:  now.Add(-2 * time.Minute),
		UpdatedAt:  now.Add(-2 * time.Minute),
	}
	if err := repo.Save(ctx, freshTrip); err != nil {
		t.Fatalf("failed to save fresh trip: %v", err)
	}
	freshPos := model.Position{
		Coordinates: model.Coordinates{Latitude: 55.76, Longitude: 37.62},
		RecordedAt:  now.Add(-5 * time.Second),
	}
	if _, err := repo.SavePosition(ctx, freshTripID, freshPos); err != nil {
		t.Fatalf("failed to save fresh pos: %v", err)
	}

	completedTrip := model.Trip{
		ID:         completedTripID,
		UserID:     uuid.New(),
		DriverID:   uuid.New(),
		StartPoint: model.Coordinates{Latitude: 55.75, Longitude: 37.61},
		EndPoint:   model.Coordinates{Latitude: 55.80, Longitude: 37.70},
		Price:      500,
		Status:     model.StatusActive,
		StartedAt:  now.Add(-2 * time.Minute),
		CreatedAt:  now.Add(-2 * time.Minute),
		UpdatedAt:  now.Add(-2 * time.Minute),
	}
	if err := repo.Save(ctx, completedTrip); err != nil {
		t.Fatalf("failed to save completed trip: %v", err)
	}
	if _, err := repo.Finish(ctx, completedTripID, now.Add(-1*time.Minute)); err != nil {
		t.Fatalf("failed to finish trip: %v", err)
	}

	staleTrips, err := repo.GetStaleTrips(ctx, time.Minute, model.StaleTripPage{ScanAt: now, Limit: 100})
	if err != nil {
		t.Fatalf("failed to get stale trips: %v", err)
	}

	foundStale := false
	for _, st := range staleTrips {
		if st.ID == staleTripID {
			foundStale = true
		}
		if st.ID == freshTripID {
			t.Fatalf("fresh trip should not be in stale trips list")
		}
		if st.ID == completedTripID {
			t.Fatalf("completed trip should not be in stale trips list")
		}
	}
	if !foundStale {
		t.Fatalf("expected staleTripID %v in stale trips, got %v", staleTripID, staleTrips)
	}

	firstID, secondID := staleTripID, tiedTrip.ID
	if firstID.String() > secondID.String() {
		firstID, secondID = secondID, firstID
	}
	next, err := repo.GetStaleTrips(ctx, time.Minute, model.StaleTripPage{
		ScanAt: now,
		After:  &model.StaleTrip{ID: firstID, StartedAt: staleTrip.StartedAt},
		Limit:  1,
	})
	if err != nil {
		t.Fatalf("failed to read next stale trip page: %v", err)
	}
	if len(next) != 1 || next[0].ID != secondID {
		t.Fatalf("expected next page to contain tied trip %v, got %v", secondID, next)
	}
}
