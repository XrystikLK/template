package telemetry

import (
	"context"
	"sync/atomic"

	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/metric"
)

type Metrics struct {
	RequestDuration metric.Float64Histogram
	ActiveRequests  metric.Int64UpDownCounter
	TripsCreated    metric.Int64Counter
	TripsCompleted  metric.Int64Counter
	PositionsSaved  metric.Int64Counter

	StaleChecks              metric.Int64Counter
	PositionRequests         metric.Int64Counter
	PositionRequestsDuration metric.Float64Histogram

	queueSize     atomic.Int64
	activeWorkers atomic.Int64
}

func NewMetrics(meter metric.Meter) (*Metrics, error) {
	duration, err := meter.Float64Histogram(
		"http.server.request.duration",
		metric.WithDescription("Duration of HTTP server requests in seconds"),
		metric.WithUnit("s"),
	)
	if err != nil {
		return nil, err
	}

	active, err := meter.Int64UpDownCounter(
		"http.server.active_requests",
		metric.WithDescription("Number of active HTTP server requests"),
		metric.WithUnit("{request}"),
	)
	if err != nil {
		return nil, err
	}

	created, err := meter.Int64Counter(
		"tripgo.trips.created",
		metric.WithDescription("Total number of created trips"),
		metric.WithUnit("{trip}"),
	)
	if err != nil {
		return nil, err
	}

	completed, err := meter.Int64Counter(
		"tripgo.trips.completed",
		metric.WithDescription("Total number of completed trips"),
		metric.WithUnit("{trip}"),
	)
	if err != nil {
		return nil, err
	}

	saved, err := meter.Int64Counter(
		"tripgo.positions.saved",
		metric.WithDescription("Total number of saved positions"),
		metric.WithUnit("{position}"),
	)
	if err != nil {
		return nil, err
	}

	staleChecks, err := meter.Int64Counter(
		"tripgo.stale_checks",
		metric.WithDescription("Total number of stale trip checks"),
		metric.WithUnit("{check}"),
	)
	if err != nil {
		return nil, err
	}

	posRequests, err := meter.Int64Counter(
		"tripgo.position_requests",
		metric.WithDescription("Total number of position requests to push service"),
		metric.WithUnit("{request}"),
	)
	if err != nil {
		return nil, err
	}

	posDuration, err := meter.Float64Histogram(
		"tripgo.position_requests.duration",
		metric.WithDescription("Duration of position requests to push service"),
		metric.WithUnit("s"),
	)
	if err != nil {
		return nil, err
	}

	m := &Metrics{
		RequestDuration:          duration,
		ActiveRequests:           active,
		TripsCreated:             created,
		TripsCompleted:           completed,
		PositionsSaved:           saved,
		StaleChecks:              staleChecks,
		PositionRequests:         posRequests,
		PositionRequestsDuration: posDuration,
	}

	_, err = meter.Int64ObservableGauge(
		"tripgo.position_requests.queue.size",
		metric.WithDescription("Current size of the position requests queue"),
		metric.WithUnit("{task}"),
		metric.WithInt64Callback(func(_ context.Context, obs metric.Int64Observer) error {
			obs.Observe(m.queueSize.Load())
			return nil
		}),
	)
	if err != nil {
		return nil, err
	}

	_, err = meter.Int64ObservableGauge(
		"tripgo.position_requests.workers.active",
		metric.WithDescription("Number of active workers processing position requests"),
		metric.WithUnit("{worker}"),
		metric.WithInt64Callback(func(_ context.Context, obs metric.Int64Observer) error {
			obs.Observe(m.activeWorkers.Load())
			return nil
		}),
	)
	if err != nil {
		return nil, err
	}

	return m, nil
}

func (m *Metrics) IncTripsCreated(ctx context.Context) {
	if m != nil && m.TripsCreated != nil {
		m.TripsCreated.Add(ctx, 1)
	}
}

func (m *Metrics) IncTripsCompleted(ctx context.Context) {
	if m != nil && m.TripsCompleted != nil {
		m.TripsCompleted.Add(ctx, 1)
	}
}

func (m *Metrics) IncPositionsSaved(ctx context.Context) {
	if m != nil && m.PositionsSaved != nil {
		m.PositionsSaved.Add(ctx, 1)
	}
}

func (m *Metrics) IncStaleChecks(ctx context.Context, result string) {
	if m != nil && m.StaleChecks != nil {
		m.StaleChecks.Add(ctx, 1, metric.WithAttributes(attribute.String("result", result)))
	}
}

func (m *Metrics) SetQueueSize(size int64) {
	if m != nil {
		m.queueSize.Store(size)
	}
}

func (m *Metrics) AddActiveWorkers(delta int64) {
	if m != nil {
		m.activeWorkers.Add(delta)
	}
}

func (m *Metrics) IncPositionRequests(ctx context.Context, result string) {
	if m != nil && m.PositionRequests != nil {
		m.PositionRequests.Add(ctx, 1, metric.WithAttributes(attribute.String("result", result)))
	}
}

func (m *Metrics) RecordPositionRequestDuration(ctx context.Context, durationSec float64, result string) {
	if m != nil && m.PositionRequestsDuration != nil {
		m.PositionRequestsDuration.Record(ctx, durationSec, metric.WithAttributes(attribute.String("result", result)))
	}
}

func (m *Metrics) RecordRequestDuration(ctx context.Context, durationSec float64, method, route, statusCode string) {
	if m == nil || m.RequestDuration == nil {
		return
	}
	attrs := metric.WithAttributes(
		attribute.String("http.request.method", method),
		attribute.String("http.route", route),
		attribute.String("http.response.status_code", statusCode),
	)
	m.RequestDuration.Record(ctx, durationSec, attrs)
}

func (m *Metrics) AddActiveRequests(ctx context.Context, delta int64, method, route string) {
	if m == nil || m.ActiveRequests == nil {
		return
	}
	attrs := metric.WithAttributes(
		attribute.String("http.request.method", method),
		attribute.String("http.route", route),
	)
	m.ActiveRequests.Add(ctx, delta, attrs)
}
