-- +goose NO TRANSACTION
-- +goose Up
DROP INDEX CONCURRENTLY IF EXISTS trips_stale_page_idx;
CREATE INDEX CONCURRENTLY trips_stale_page_idx
    ON trips (COALESCE(last_position_at, started_at), id)
    WHERE status = 'active';

-- +goose Down
DROP INDEX CONCURRENTLY IF EXISTS trips_stale_page_idx;
