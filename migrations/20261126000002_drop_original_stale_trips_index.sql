-- +goose NO TRANSACTION
-- +goose Up
DROP INDEX CONCURRENTLY IF EXISTS trips_stale_scan_idx;

-- +goose Down
DROP INDEX CONCURRENTLY IF EXISTS trips_stale_scan_idx;
CREATE INDEX CONCURRENTLY trips_stale_scan_idx
    ON trips (COALESCE(last_position_at, started_at) ASC)
    WHERE status = 'active';
