package main

import (
	"context"
	"errors"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/XrystikLK/template/internal/config"
	"github.com/XrystikLK/template/internal/db"
	api "github.com/XrystikLK/template/internal/generated"
	"github.com/XrystikLK/template/internal/handler"
	"github.com/XrystikLK/template/internal/repository"
	"github.com/go-chi/chi/v5"
)

func main() {
	cfg, err := config.Load()
	if err != nil {
		log.Fatal(err)
	}

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	pool, err := db.InitPool(ctx, cfg)
	if err != nil {
		log.Fatal(err)
	}
	defer pool.Close()

	queryTimeout := 3 * time.Second
	if cfg.DatabaseQueryTimeout != "" {
		if d, err := time.ParseDuration(cfg.DatabaseQueryTimeout); err == nil {
			queryTimeout = d
		}
	}

	txManager := db.NewTxManager(pool)
	tripRepo := repository.NewTripRepository(pool, queryTimeout)
	tripHandler := handler.NewTripHandler(tripRepo, txManager, pool)

	r := chi.NewRouter()
	api.HandlerFromMux(tripHandler, r)

	addr := cfg.HTTPAddr
	if addr == "" {
		addr = ":8080"
	}

	server := &http.Server{
		Addr:              addr,
		Handler:           r,
		ReadTimeout:       5 * time.Second,
		ReadHeaderTimeout: 2 * time.Second,
		WriteTimeout:      5 * time.Second,
		IdleTimeout:       30 * time.Second,
	}

	log.Printf("server starting on %s", addr)

	go func() {
		if err := server.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			log.Fatal(err)
		}
	}()

	<-ctx.Done()
	log.Printf("shutting down gracefully..")

	shutdownTimeout := 10 * time.Second
	if cfg.ShutdownTimeout != "" {
		if d, err := time.ParseDuration(cfg.ShutdownTimeout); err == nil {
			shutdownTimeout = d
		}
	}

	shutdownCtx, cancel := context.WithTimeout(context.Background(), shutdownTimeout)
	defer cancel()

	if err := server.Shutdown(shutdownCtx); err != nil {
		log.Printf("server shutdown error: %v", err)
	}
}
