package config

import (
	"fmt"
	"os"

	"github.com/joho/godotenv"
)

type Config struct {
	HTTPAddr                string
	DatabaseURL             string
	DatabaseMaxConns        string
	DatabaseMinConns        string
	DatabaseMaxConnLifetime string
	DatabaseConnectTimeout  string
	DatabaseQueryTimeout    string
}


func Load() (*Config, error) {
	_ = godotenv.Load()

	dbURL := os.Getenv("DATABASE_URL")
	if dbURL == "" {
		return nil, fmt.Errorf("DATABASE_URL обязательна, но не задана")
	}

	return &Config{
		HTTPAddr:                os.Getenv("HTTP_ADDR"),
		DatabaseURL:             dbURL,
		DatabaseMaxConns:        os.Getenv("DATABASE_MAX_CONNS"),
		DatabaseMinConns:        os.Getenv("DATABASE_MIN_CONNS"),
		DatabaseMaxConnLifetime: os.Getenv("DATABASE_MAX_CONN_LIFETIME"),
		DatabaseConnectTimeout:  os.Getenv("DATABASE_CONNECT_TIMEOUT"),
		DatabaseQueryTimeout:    os.Getenv("DATABASE_QUERY_TIMEOUT"),
	}, nil
}