package positions

import (
	"context"
	"slices"
	"sync"
	"time"

	"github.com/google/uuid"
	"go.opentelemetry.io/otel/trace"
)

type Task struct {
	TripID         uuid.UUID
	DriverID       uuid.UUID
	LastActivityAt time.Time
	SpanContext    trace.SpanContext
}

type Queue struct {
	pending       []*Task
	capacity      int
	ready         chan struct{}
	done          chan struct{}
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
		pending:  make([]*Task, 0, capacity),
		capacity: capacity,
		ready:    make(chan struct{}, 1),
		done:     make(chan struct{}),
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

	var evicted *Task
	if len(q.pending) == q.capacity {
		newest := q.pending[0]
		if !task.LastActivityAt.Before(newest.LastActivityAt) {
			q.mu.Unlock()
			q.recordDrop(ctx, task.TripID)
			return false
		}
		evicted = newest
		delete(q.inFlight, newest.TripID)
		q.pending = slices.Delete(q.pending, 0, 1)
	}
	index, _ := slices.BinarySearchFunc(q.pending, &task, func(queued, incoming *Task) int {
		return incoming.LastActivityAt.Compare(queued.LastActivityAt)
	})
	q.pending = slices.Insert(q.pending, index, &task)
	q.inFlight[task.TripID] = &task
	q.recordSize()
	q.signalReady()
	q.mu.Unlock()
	if evicted != nil {
		q.recordDrop(ctx, evicted.TripID)
	}
	if q.metrics != nil {
		q.metrics.IncStaleChecks(ctx, "queued")
	}
	return true
}

func (q *Queue) recordDrop(ctx context.Context, tripID uuid.UUID) {
	if q.metrics != nil {
		q.metrics.IncStaleChecks(ctx, "dropped")
		q.metrics.IncPositionRequests(ctx, "dropped")
	}
	if q.logger != nil {
		q.logger.WarnContext(ctx, "positions queue is full, task dropped", "trip_id", tripID.String())
	}
}

func (q *Queue) Cancel(tripID uuid.UUID) {
	q.mu.Lock()
	if index := slices.Index(q.pending, q.inFlight[tripID]); index >= 0 {
		q.pending = slices.Delete(q.pending, index, index+1)
		q.recordSize()
	}
	delete(q.inFlight, tripID)
	if q.scanCanceled != nil {
		q.scanCanceled[tripID] = struct{}{}
	}
	q.mu.Unlock()
}

func (q *Queue) dequeued(task *Task) bool {
	q.mu.Lock()
	defer q.mu.Unlock()
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
		q.metrics.SetQueueSize(int64(len(q.pending)))
	}
}

func (q *Queue) signalReady() {
	select {
	case q.ready <- struct{}{}:
	default:
	}
}

func (q *Queue) take(ctx context.Context) *Task {
	for ctx.Err() == nil {
		q.mu.Lock()
		if len(q.pending) > 0 {
			last := len(q.pending) - 1
			task := q.pending[last]
			q.pending[last] = nil
			q.pending = q.pending[:last]
			q.recordSize()
			if last > 0 {
				q.signalReady()
			}
			q.mu.Unlock()
			return task
		}
		q.mu.Unlock()
		select {
		case <-q.ready:
		case <-q.done:
			return nil
		case <-ctx.Done():
			return nil
		}
	}
	return nil
}

func (q *Queue) Close() {
	q.mu.Lock()
	defer q.mu.Unlock()
	if !q.closed {
		q.closed = true
		close(q.done)
	}
}
