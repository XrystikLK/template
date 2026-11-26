package positions

import (
	"context"
	"sync"
	"time"

	"github.com/google/uuid"
	"go.opentelemetry.io/otel/trace"
)

type Task struct {
	TripID      uuid.UUID
	DriverID    uuid.UUID
	SpanContext trace.SpanContext
}

type Queue struct {
	ch            chan *Task
	mu            sync.Mutex
	inFlight      map[uuid.UUID]*Task
	nextRequest   map[uuid.UUID]time.Time
	scanCanceled  map[uuid.UUID]struct{}
	checkInterval time.Duration
	closed        bool
	metrics       metrics
	logger        logger
}

func NewQueue(capacity int, metrics metrics, logger logger) *Queue {
	if capacity <= 0 {
		capacity = 100
	}
	return &Queue{
		ch:       make(chan *Task, capacity),
		inFlight: make(map[uuid.UUID]*Task),
		metrics:  metrics,
		logger:   logger,
	}
}

func (q *Queue) TryPush(ctx context.Context, task Task) bool {
	return q.tryPush(ctx, task, false)
}

func (q *Queue) tryPush(ctx context.Context, task Task, fromScan bool) bool {
	q.mu.Lock()

	if q.closed {
		q.mu.Unlock()
		return false
	}

	_, canceled := q.scanCanceled[task.TripID]
	if (fromScan && canceled) || q.inFlight[task.TripID] != nil || time.Now().Before(q.nextRequest[task.TripID]) {
		q.mu.Unlock()
		if q.metrics != nil {
			q.metrics.IncStaleChecks(ctx, "duplicate")
		}
		return false
	}

	select {
	case q.ch <- &task:
		q.inFlight[task.TripID] = &task
		q.recordSize()
		q.mu.Unlock()
		if q.metrics != nil {
			q.metrics.IncStaleChecks(ctx, "queued")
		}
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
	if q.scanCanceled != nil {
		q.scanCanceled[tripID] = struct{}{}
	}
	q.mu.Unlock()
}

func (q *Queue) dequeued(task *Task) bool {
	q.mu.Lock()
	defer q.mu.Unlock()
	q.recordSize()
	if q.inFlight[task.TripID] != task {
		return false
	}
	if q.checkInterval > 0 {
		q.nextRequest[task.TripID] = time.Now().Add(q.checkInterval)
	}
	return true
}

func (q *Queue) beginScan(now time.Time) {
	q.mu.Lock()
	defer q.mu.Unlock()
	q.scanCanceled = make(map[uuid.UUID]struct{})
	for id, deadline := range q.nextRequest {
		if !now.Before(deadline) {
			delete(q.nextRequest, id)
		}
	}
}

func (q *Queue) endScan() {
	q.mu.Lock()
	q.scanCanceled = nil
	q.mu.Unlock()
}

func (q *Queue) remove(task *Task) {
	q.mu.Lock()
	if q.inFlight[task.TripID] == task {
		delete(q.inFlight, task.TripID)
	}
	q.mu.Unlock()
}

func (q *Queue) recordSize() {
	if q.metrics != nil {
		q.metrics.SetQueueSize(int64(len(q.ch)))
	}
}

func (q *Queue) Close() {
	q.mu.Lock()
	defer q.mu.Unlock()
	if !q.closed {
		q.closed = true
		close(q.ch)
	}
}
