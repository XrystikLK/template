package config

import (
	"fmt"
	"os"

	"github.com/joho/godotenv"
)

type Config struct {
	HTTPAddr                 string
	LogLevel                 string
	ShutdownTimeout          string
	DatabaseURL              string
	DatabaseMaxConns         string
	DatabaseMinConns         string
	DatabaseMaxConnLifetime  string
	DatabaseConnectTimeout   string
	DatabaseQueryTimeout     string
	OtelServiceName          string
	OtelExporterOtlpEndpoint string
	OtelExporterOtlpProtocol string
	OtelResourceAttributes   string
	OtelTracesSampler        string
}

func Load() (*Config, error) {
	_ = godotenv.Load()

	dbURL := os.Getenv("DATABASE_URL")
	if dbURL == "" {
		return nil, fmt.Errorf("DATABASE_URL обязательна, но не задана")
	}

	logLevel := os.Getenv("LOG_LEVEL")
	if logLevel == "" {
		logLevel = "info"
	}

	otelService := os.Getenv("OTEL_SERVICE_NAME")
	if otelService == "" {
		otelService = "trip-service"
	}

	return &Config{
		HTTPAddr:                 os.Getenv("HTTP_ADDR"),
		LogLevel:                 logLevel,
		ShutdownTimeout:          os.Getenv("SHUTDOWN_TIMEOUT"),
		DatabaseURL:              dbURL,
		DatabaseMaxConns:         os.Getenv("DATABASE_MAX_CONNS"),
		DatabaseMinConns:         os.Getenv("DATABASE_MIN_CONNS"),
		DatabaseMaxConnLifetime:  os.Getenv("DATABASE_MAX_CONN_LIFETIME"),
		DatabaseConnectTimeout:   os.Getenv("DATABASE_CONNECT_TIMEOUT"),
		DatabaseQueryTimeout:     os.Getenv("DATABASE_QUERY_TIMEOUT"),
		OtelServiceName:          otelService,
		OtelExporterOtlpEndpoint: os.Getenv("OTEL_EXPORTER_OTLP_ENDPOINT"),
		OtelExporterOtlpProtocol: os.Getenv("OTEL_EXPORTER_OTLP_PROTOCOL"),
		OtelResourceAttributes:   os.Getenv("OTEL_RESOURCE_ATTRIBUTES"),
		OtelTracesSampler:        os.Getenv("OTEL_TRACES_SAMPLER"),
	}, nil
}
