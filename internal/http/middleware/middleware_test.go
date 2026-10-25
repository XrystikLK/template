package middleware_test

import (
	"bytes"
	"context"
	"encoding/json"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/XrystikLK/template/internal/http/middleware"
	"github.com/XrystikLK/template/internal/infra/telemetry"
	"github.com/go-chi/chi/v5"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/sdk/metric"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
)

func TestRecover(t *testing.T) {
	var buf bytes.Buffer
	logger := slog.New(slog.NewJSONHandler(&buf, nil))

	r := chi.NewRouter()
	r.Use(middleware.Recover(logger))
	r.Get("/panic", func(w http.ResponseWriter, r *http.Request) {
		panic("test panic")
	})

	req := httptest.NewRequest(http.MethodGet, "/panic", nil)
	rec := httptest.NewRecorder()

	r.ServeHTTP(rec, req)

	if rec.Code != http.StatusInternalServerError {
		t.Fatalf("expected status 500, got %d", rec.Code)
	}

	var logEntry map[string]any
	if err := json.Unmarshal(buf.Bytes(), &logEntry); err != nil {
		t.Fatalf("failed to parse log json: %v", err)
	}
	if logEntry["level"] != "ERROR" {
		t.Fatalf("expected log level ERROR, got %v", logEntry["level"])
	}
}

func TestLogging(t *testing.T) {
	var buf bytes.Buffer
	logger := slog.New(slog.NewJSONHandler(&buf, nil))

	r := chi.NewRouter()
	r.Use(middleware.Logging(logger))
	r.Get("/test/{id}", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})

	req := httptest.NewRequest(http.MethodGet, "/test/42", nil)
	rec := httptest.NewRecorder()

	r.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected status 200, got %d", rec.Code)
	}

	var logEntry map[string]any
	if err := json.Unmarshal(buf.Bytes(), &logEntry); err != nil {
		t.Fatalf("failed to parse log json: %v", err)
	}
	if logEntry["route"] != "/test/{id}" {
		t.Fatalf("expected route template /test/{id}, got %v", logEntry["route"])
	}
	if logEntry["method"] != "GET" {
		t.Fatalf("expected method GET, got %v", logEntry["method"])
	}
}

func TestTracingAndMetrics(t *testing.T) {
	tp := sdktrace.NewTracerProvider()
	tracer := tp.Tracer("test")
	otel.SetTracerProvider(tp)

	mp := metric.NewMeterProvider()
	meter := mp.Meter("test")
	metrics, err := telemetry.NewMetrics(meter)
	if err != nil {
		t.Fatalf("failed to create metrics: %v", err)
	}

	r := chi.NewRouter()
	r.Use(middleware.Tracing(tracer))
	r.Use(middleware.Metrics(metrics))
	r.Get("/items/{id}", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})

	req := httptest.NewRequest(http.MethodGet, "/items/100", nil)
	rec := httptest.NewRecorder()

	r.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected status 200, got %d", rec.Code)
	}
}

func TestTelemetryLoggerTraceFields(t *testing.T) {
	tp := sdktrace.NewTracerProvider()
	tracer := tp.Tracer("test")

	ctx, span := tracer.Start(context.Background(), "test-span")
	defer span.End()

	logger := telemetry.NewLogger("info", "test-service")
	logger.InfoContext(ctx, "test event")
}

