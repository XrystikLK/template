package postgres

import (
	"context"
	"strconv"
	"time"

	"github.com/XrystikLK/template/internal/config"
	"github.com/jackc/pgx/v5/pgxpool"
)

func NewPool(ctx context.Context, cfg *config.Config) (*pgxpool.Pool, error) {
	poolCfg, err := pgxpool.ParseConfig(cfg.DatabaseURL)
	if err != nil {
		return nil, err
	}

	if maxConns, err := strconv.Atoi(cfg.DatabaseMaxConns); err == nil && maxConns > 0 {
		poolCfg.MaxConns = int32(maxConns)
	}
	if minConns, err := strconv.Atoi(cfg.DatabaseMinConns); err == nil && minConns > 0 {
		poolCfg.MinConns = int32(minConns)
	}
	if maxLifetime, err := time.ParseDuration(cfg.DatabaseMaxConnLifetime); err == nil {
		poolCfg.MaxConnLifetime = maxLifetime
	}
	if connTimeout, err := time.ParseDuration(cfg.DatabaseConnectTimeout); err == nil {
		poolCfg.ConnConfig.ConnectTimeout = connTimeout
	}

	pool, err := pgxpool.NewWithConfig(ctx, poolCfg)
	if err != nil {
		return nil, err
	}

	if err := pool.Ping(ctx); err != nil {
		pool.Close()
		return nil, err
	}

	return pool, nil
}

