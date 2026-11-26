-- +goose NO TRANSACTION
-- +goose Up
DROP INDEX CONCURRENTLY IF EXISTS trips_stale_scan_concurrent_idx;

-- +goose Down
DROP INDEX CONCURRENTLY IF EXISTS trips_stale_scan_concurrent_idx;
CREATE INDEX CONCURRENTLY trips_stale_scan_concurrent_idx
    ON trips (COALESCE(last_position_at, started_at))
    WHERE status = 'active';
