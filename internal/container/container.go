package container

import (
	"context"
	"net/http"
	"time"

	"github.com/XrystikLK/template/internal/config"
	api "github.com/XrystikLK/template/internal/generated"
	httpTransport "github.com/XrystikLK/template/internal/http"
	createHandler "github.com/XrystikLK/template/internal/http/trip/create"
	finishHandler "github.com/XrystikLK/template/internal/http/trip/finish"
	getHandler "github.com/XrystikLK/template/internal/http/trip/get"
	"github.com/XrystikLK/template/internal/infra/postgres"
	"github.com/XrystikLK/template/internal/infra/postgres/transaction"
	repoTrip "github.com/XrystikLK/template/internal/repository/trip"
	createUC "github.com/XrystikLK/template/internal/usecase/trip/create"
	finishUC "github.com/XrystikLK/template/internal/usecase/trip/finish"
	getUC "github.com/XrystikLK/template/internal/usecase/trip/get"
	"github.com/go-chi/chi/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

type Container struct {
	server *http.Server
	pool   *pgxpool.Pool
}

func New(ctx context.Context, cfg *config.Config) (*Container, error) {
	pool, err := postgres.NewPool(ctx, cfg)
	if err != nil {
		return nil, err
	}

	queryTimeout := 3 * time.Second
	if cfg.DatabaseQueryTimeout != "" {
		if d, err := time.ParseDuration(cfg.DatabaseQueryTimeout); err == nil {
			queryTimeout = d
		}
	}

	txManager := transaction.NewManager(pool)
	tripRepo := repoTrip.New(pool, queryTimeout)

	createUseCase := createUC.New(tripRepo, txManager)
	getUseCase := getUC.New(tripRepo)
	finishUseCase := finishUC.New(tripRepo, txManager)

	createH := createHandler.New(createUseCase)
	getH := getHandler.New(getUseCase)
	finishH := finishHandler.New(finishUseCase)

	srv := httpTransport.NewServer(createH, getH, finishH, pool)

	r := chi.NewRouter()
	api.HandlerFromMux(srv, r)

	addr := cfg.HTTPAddr
	if addr == "" {
		addr = ":8080"
	}

	httpServer := &http.Server{
		Addr:              addr,
		Handler:           r,
		ReadTimeout:       5 * time.Second,
		ReadHeaderTimeout: 2 * time.Second,
		WriteTimeout:      5 * time.Second,
		IdleTimeout:       30 * time.Second,
	}

	return &Container{
		server: httpServer,
		pool:   pool,
	}, nil
}

func (c *Container) Start() error {
	return c.server.ListenAndServe()
}

func (c *Container) Shutdown(ctx context.Context) error {
	defer c.pool.Close()
	return c.server.Shutdown(ctx)
}

func (c *Container) Addr() string {
	return c.server.Addr
}

