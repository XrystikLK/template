package push

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/google/uuid"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/codes"
	"go.opentelemetry.io/otel/trace"
)

var (
	ErrClient     = errors.New("push service client error")
	ErrServer     = errors.New("push service server error")
	ErrUnexpected = errors.New("push service unexpected status")
	ErrTransport  = errors.New("push service transport error")
)

type Client struct {
	baseURL    string
	httpClient *http.Client
	tracer     trace.Tracer
}

func New(baseURL string, timeout time.Duration, tracer trace.Tracer) (*Client, error) {
	if strings.TrimSpace(baseURL) == "" {
		return nil, errors.New("push service baseURL cannot be empty")
	}
	parsedURL, err := url.Parse(baseURL)
	if err != nil || parsedURL.Scheme == "" || parsedURL.Host == "" {
		return nil, fmt.Errorf("invalid push service baseURL %q: %w", baseURL, err)
	}

	if timeout <= 0 {
		timeout = 1 * time.Second
	}

	httpClient := &http.Client{
		Timeout: timeout,
	}

	return &Client{
		baseURL:    strings.TrimRight(baseURL, "/"),
		httpClient: httpClient,
		tracer:     tracer,
	}, nil
}

type pushData struct {
	TripID string `json:"trip_id"`
}

type pushRequest struct {
	RequestID   string   `json:"request_id"`
	RecipientID string   `json:"recipient_id"`
	Kind        string   `json:"kind"`
	Data        pushData `json:"data"`
}

func (c *Client) RequestPosition(ctx context.Context, driverID, tripID uuid.UUID) error {
	var span trace.Span
	if c.tracer != nil {
		ctx, span = c.tracer.Start(ctx, "push.request_position",
			trace.WithAttributes(
				attribute.String("driver_id", driverID.String()),
				attribute.String("trip_id", tripID.String()),
			),
		)
		defer span.End()
	}

	reqBody := pushRequest{
		RequestID:   uuid.New().String(),
		RecipientID: driverID.String(),
		Kind:        "REQUEST_POSITION",
		Data: pushData{
			TripID: tripID.String(),
		},
	}

	bodyBytes, err := json.Marshal(reqBody)
	if err != nil {
		if span != nil {
			span.RecordError(err)
			span.SetStatus(codes.Error, err.Error())
		}
		return fmt.Errorf("%w: failed to marshal request: %v", ErrClient, err)
	}

	endpoint := c.baseURL + "/api/v1/push"
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, endpoint, bytes.NewReader(bodyBytes))
	if err != nil {
		if span != nil {
			span.RecordError(err)
			span.SetStatus(codes.Error, err.Error())
		}
		return fmt.Errorf("%w: failed to create request: %v", ErrTransport, err)
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := c.httpClient.Do(req)
	if err != nil {
		if span != nil {
			span.RecordError(err)
			span.SetStatus(codes.Error, err.Error())
		}
		return fmt.Errorf("%w: %v", ErrTransport, err)
	}
	defer func() {
		_, _ = io.Copy(io.Discard, resp.Body)
		_ = resp.Body.Close()
	}()

	if span != nil {
		span.SetAttributes(attribute.Int("http.status_code", resp.StatusCode))
	}

	if resp.StatusCode == http.StatusAccepted {
		return nil
	}

	bodySample, _ := io.ReadAll(io.LimitReader(resp.Body, 1024))
	statusErr := fmt.Errorf("status %d: %s", resp.StatusCode, string(bodySample))

	switch {
	case resp.StatusCode >= 400 && resp.StatusCode < 500:
		err = fmt.Errorf("%w: %v", ErrClient, statusErr)
	case resp.StatusCode >= 500:
		err = fmt.Errorf("%w: %v", ErrServer, statusErr)
	default:
		err = fmt.Errorf("%w: %v", ErrUnexpected, statusErr)
	}

	if span != nil {
		span.RecordError(err)
		span.SetStatus(codes.Error, err.Error())
	}
	return err
}
