package positions

import (
	"context"
	"time"

	"github.com/google/uuid"
	"go.opentelemetry.io/otel/trace"
)

type Service struct {
	queue   *Queue
	scanner *Scanner
	pool    *Pool
}

func NewService(
	repo tripRepository,
	gateway pushGateway,
	ticker Ticker,
	queueSize int,
	workersCount int,
	checkInterval time.Duration,
	staleAfter time.Duration,
	reqTimeout time.Duration,
	tracer trace.Tracer,
	logger logger,
	metrics metrics,
) *Service {
	if ticker == nil {
		ticker = NewRealTicker(checkInterval)
	}

	q := NewQueue(queueSize, metrics, logger)
	scanner := NewScanner(repo, q, ticker, staleAfter, tracer, logger)
	pool := NewPool(q, gateway, workersCount, reqTimeout, tracer, logger, metrics)

	return &Service{
		queue:   q,
		scanner: scanner,
		pool:    pool,
	}
}

func (s *Service) Start(ctx context.Context) {
	s.pool.Start(ctx)
	s.scanner.Start(ctx)
}

func (s *Service) Cancel(tripID uuid.UUID) {
	s.queue.Cancel(tripID)
}

func (s *Service) StopScanner() {
	s.scanner.Stop()
}

func (s *Service) StopWorkers(ctx context.Context) {
	s.queue.Close()
	done := make(chan struct{})
	go func() {
		s.pool.Wait()
		close(done)
	}()

	select {
	case <-done:
	case <-ctx.Done():
	}
}
