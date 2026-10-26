package middleware_test

import (
	"bytes"
	"context"
	"encoding/json"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/go-chi/chi/v5"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"

	"github.com/XrystikLK/template/internal/http/middleware"
)

type mockMetricsRecorder struct {
	recordedDuration bool
	activeRequests   int64
}

func (m *mockMetricsRecorder) RecordRequestDuration(_ context.Context, _ float64, _, _, _ string) {
	m.recordedDuration = true
}

func (m *mockMetricsRecorder) AddActiveRequests(_ context.Context, delta int64, _, _ string) {
	m.activeRequests += delta
}

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

	recorder := &mockMetricsRecorder{}

	r := chi.NewRouter()
	r.Use(middleware.Tracing(tracer))
	r.Use(middleware.Metrics(recorder))
	r.Get("/items/{id}", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})

	req := httptest.NewRequest(http.MethodGet, "/items/100", nil)
	rec := httptest.NewRecorder()

	r.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected status 200, got %d", rec.Code)
	}

	if !recorder.recordedDuration {
		t.Fatalf("expected duration to be recorded")
	}

	if recorder.activeRequests != 0 {
		t.Fatalf("expected activeRequests to be 0 after request, got %d", recorder.activeRequests)
	}
}
