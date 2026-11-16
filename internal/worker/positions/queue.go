package positions

import (
	"context"
	"sync"

	"github.com/google/uuid"
	"go.opentelemetry.io/otel/trace"
)

type Task struct {
	TripID      uuid.UUID
	DriverID    uuid.UUID
	SpanContext trace.SpanContext
}

type Queue struct {
	ch       chan Task
	mu       sync.Mutex
	inFlight map[uuid.UUID]struct{}
	metrics  metrics
	logger   logger
}

func NewQueue(capacity int, metrics metrics, logger logger) *Queue {
	if capacity <= 0 {
		capacity = 100
	}
	return &Queue{
		ch:       make(chan Task, capacity),
		inFlight: make(map[uuid.UUID]struct{}),
		metrics:  metrics,
		logger:   logger,
	}
}

func (q *Queue) TryPush(ctx context.Context, task Task) bool {
	q.mu.Lock()

	if _, exists := q.inFlight[task.TripID]; exists {
		q.mu.Unlock()
		if q.metrics != nil {
			q.metrics.IncStaleChecks(ctx, "duplicate")
		}
		return false
	}

	select {
	case q.ch <- task:
		q.inFlight[task.TripID] = struct{}{}
		if q.metrics != nil {
			q.metrics.SetQueueSize(int64(len(q.ch)))
			q.metrics.IncStaleChecks(ctx, "queued")
		}
		q.mu.Unlock()
		return true
	default:
		q.mu.Unlock()
		if q.metrics != nil {
			q.metrics.IncStaleChecks(ctx, "dropped")
			q.metrics.IncPositionRequests(ctx, "dropped")
		}
		if q.logger != nil {
			q.logger.WarnContext(ctx, "positions queue is full, task dropped", "trip_id", task.TripID.String())
		}
		return false
	}
}

func (q *Queue) Cancel(tripID uuid.UUID) {
	q.mu.Lock()
	delete(q.inFlight, tripID)
	q.mu.Unlock()
}

func (q *Queue) Channel() <-chan Task {
	return q.ch
}

func (q *Queue) IsActive(tripID uuid.UUID) bool {
	q.mu.Lock()
	defer q.mu.Unlock()
	_, exists := q.inFlight[tripID]
	return exists
}

func (q *Queue) Remove(tripID uuid.UUID) {
	q.mu.Lock()
	delete(q.inFlight, tripID)
	if q.metrics != nil {
		q.metrics.SetQueueSize(int64(len(q.ch)))
	}
	q.mu.Unlock()
}

func (q *Queue) Close() {
	close(q.ch)
}
