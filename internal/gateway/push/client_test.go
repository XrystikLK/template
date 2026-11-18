package push_test

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/otel/trace/noop"

	"github.com/XrystikLK/template/internal/gateway/push"
)

type pushPayload struct {
	RequestID   string `json:"request_id"`
	RecipientID string `json:"recipient_id"`
	Kind        string `json:"kind"`
	Data        struct {
		TripID string `json:"trip_id"`
	} `json:"data"`
}

func TestNewValidation(t *testing.T) {
	tracer := noop.NewTracerProvider().Tracer("test")

	_, err := push.New("", time.Second, tracer)
	require.Error(t, err)

	_, err = push.New("not-a-valid-url", time.Second, tracer)
	require.Error(t, err)

	client, err := push.New("http://localhost:8090", 0, tracer)
	require.NoError(t, err)
	require.NotNil(t, client)
}

func TestRequestPositionSuccess(t *testing.T) {
	driverID := uuid.New()
	tripID := uuid.New()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		require.Equal(t, http.MethodPost, r.Method)
		require.Equal(t, "/api/v1/push", r.URL.Path)
		require.Equal(t, "application/json", r.Header.Get("Content-Type"))

		var payload pushPayload
		err := json.NewDecoder(r.Body).Decode(&payload)
		require.NoError(t, err)

		require.NotEmpty(t, payload.RequestID)
		require.Equal(t, driverID.String(), payload.RecipientID)
		require.Equal(t, "REQUEST_POSITION", payload.Kind)
		require.Equal(t, tripID.String(), payload.Data.TripID)

		w.WriteHeader(http.StatusAccepted)
		_, _ = w.Write([]byte(`{"request_id":"` + payload.RequestID + `","status":"QUEUED"}`))
	}))
	defer server.Close()

	tracer := noop.NewTracerProvider().Tracer("test")
	client, err := push.New(server.URL, time.Second, tracer)
	require.NoError(t, err)

	err = client.RequestPosition(context.Background(), driverID, tripID)
	require.NoError(t, err)
}

func TestRequestPositionClientError(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusBadRequest)
		_, _ = w.Write([]byte(`{"error":"bad request"}`))
	}))
	defer server.Close()

	tracer := noop.NewTracerProvider().Tracer("test")
	client, err := push.New(server.URL, time.Second, tracer)
	require.NoError(t, err)

	err = client.RequestPosition(context.Background(), uuid.New(), uuid.New())
	require.Error(t, err)
	require.ErrorIs(t, err, push.ErrClient)
}

func TestRequestPositionServerError(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusServiceUnavailable)
		_, _ = w.Write([]byte(`{"error":"unavailable"}`))
	}))
	defer server.Close()

	tracer := noop.NewTracerProvider().Tracer("test")
	client, err := push.New(server.URL, time.Second, tracer)
	require.NoError(t, err)

	err = client.RequestPosition(context.Background(), uuid.New(), uuid.New())
	require.Error(t, err)
	require.ErrorIs(t, err, push.ErrServer)
}

func TestRequestPositionUnexpectedStatus(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"status":"ok"}`))
	}))
	defer server.Close()

	tracer := noop.NewTracerProvider().Tracer("test")
	client, err := push.New(server.URL, time.Second, tracer)
	require.NoError(t, err)

	err = client.RequestPosition(context.Background(), uuid.New(), uuid.New())
	require.Error(t, err)
	require.ErrorIs(t, err, push.ErrUnexpected)
}

func TestRequestPositionTransportTimeout(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		time.Sleep(100 * time.Millisecond)
		w.WriteHeader(http.StatusAccepted)
	}))
	defer server.Close()

	tracer := noop.NewTracerProvider().Tracer("test")
	client, err := push.New(server.URL, 10*time.Millisecond, tracer)
	require.NoError(t, err)

	err = client.RequestPosition(context.Background(), uuid.New(), uuid.New())
	require.Error(t, err)
	require.ErrorIs(t, err, push.ErrTransport)
}

func TestRequestPositionContextCancelled(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusAccepted)
	}))
	defer server.Close()

	tracer := noop.NewTracerProvider().Tracer("test")
	client, err := push.New(server.URL, time.Second, tracer)
	require.NoError(t, err)

	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	err = client.RequestPosition(ctx, uuid.New(), uuid.New())
	require.Error(t, err)
	require.ErrorIs(t, err, push.ErrTransport)
}
