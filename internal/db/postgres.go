package db

import (
	"context"
	"time"

	"github.com/XrystikLK/template/internal/config"
	"github.com/jackc/pgx/v5/pgxpool"
)

func InitPool(ctx context.Context, cfg *config.Config) (*pgxpool.Pool, error) {
	config, err := pgxpool.ParseConfig(cfg.DatabaseURL)
	if err != nil {
		return nil, err
	}
	config.MaxConns = 10
	config.MinConns = 2
	config.MaxConnLifetime = 30 * time.Minute
	config.ConnConfig.ConnectTimeout = 5 * time.Second
	
	pool, err := pgxpool.NewWithConfig(ctx, config)
	if err != nil {
		return nil, err
	}
	if err := pool.Ping(ctx); err != nil {
		pool.Close()
		return nil, err
	}
	
	return pool, nil
}