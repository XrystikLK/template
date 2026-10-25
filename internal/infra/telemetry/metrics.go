package telemetry

import (
	"go.opentelemetry.io/otel/metric"
)

type Metrics struct {
	RequestDuration metric.Float64Histogram
	ActiveRequests  metric.Int64UpDownCounter
	TripsCreated    metric.Int64Counter
	TripsCompleted  metric.Int64Counter
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

	return &Metrics{
		RequestDuration: duration,
		ActiveRequests:  active,
		TripsCreated:    created,
		TripsCompleted:  completed,
	}, nil
}

