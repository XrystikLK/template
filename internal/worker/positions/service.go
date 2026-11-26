package positions

import (
	"context"
	"sync"
	"time"

	"github.com/google/uuid"
	"go.opentelemetry.io/otel/trace"
)

type Service struct {
	queue         *Queue
	scanner       *Scanner
	pool          *Pool
	startOnce     sync.Once
	workersDone   chan struct{}
	cancelWorkers context.CancelFunc
}

func NewService(
	repo tripRepository,
	gateway pushGateway,
	ticker Ticker,
	queueSize int,
	workersCount int,
	checkInterval time.Duration,
	staleAfter time.Duration,
	batchSize int,
	reqTimeout time.Duration,
	tracer trace.Tracer,
	logger logger,
	metrics metrics,
) *Service {
	if ticker == nil {
		ticker = NewRealTicker(checkInterval)
	}

	q := NewQueue(queueSize, metrics, logger)
	q.checkInterval = checkInterval
	q.nextRequest = make(map[uuid.UUID]time.Time)
	scanner := NewScanner(repo, q, ticker, staleAfter, batchSize, tracer, logger)
	pool := NewPool(q, gateway, workersCount, reqTimeout, tracer, logger, metrics)

	return &Service{
		queue:   q,
		scanner: scanner,
		pool:    pool,
	}
}

func (s *Service) Start(ctx context.Context) {
	s.startOnce.Do(func() {
		workerCtx, cancel := context.WithCancel(ctx)
		s.cancelWorkers = cancel
		s.workersDone = make(chan struct{})
		s.pool.Start(workerCtx)
		s.scanner.Start(ctx)
		go func() {
			s.pool.Wait()
			close(s.workersDone)
		}()
	})
}

func (s *Service) Cancel(tripID uuid.UUID) {
	s.queue.Cancel(tripID)
}

func (s *Service) Shutdown(ctx context.Context) {
	s.startOnce.Do(func() {})
	if s.cancelWorkers != nil {
		defer s.cancelWorkers()
	}
	s.scanner.Stop(ctx)
	if ctx.Err() != nil && s.cancelWorkers != nil {
		s.cancelWorkers()
	}
	s.queue.Close()
	if s.workersDone == nil {
		return
	}
	select {
	case <-s.workersDone:
	case <-ctx.Done():
	}
}
