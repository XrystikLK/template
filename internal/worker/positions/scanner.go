package positions

import (
	"context"
	"sync"
	"time"

	"go.opentelemetry.io/otel/trace"
)

type Scanner struct {
	repo       tripRepository
	queue      *Queue
	ticker     Ticker
	staleAfter time.Duration
	tracer     trace.Tracer
	logger     logger
	stopCh     chan struct{}
	wg         sync.WaitGroup
}

func NewScanner(repo tripRepository, queue *Queue, ticker Ticker, staleAfter time.Duration, tracer trace.Tracer, logger logger) *Scanner {
	return &Scanner{
		repo:       repo,
		queue:      queue,
		ticker:     ticker,
		staleAfter: staleAfter,
		tracer:     tracer,
		logger:     logger,
		stopCh:     make(chan struct{}),
	}
}

func (s *Scanner) Start(ctx context.Context) {
	s.wg.Add(1)
	go func() {
		defer s.wg.Done()
		for {
			select {
			case <-ctx.Done():
				return
			case <-s.stopCh:
				return
			case <-s.ticker.Chan():
				s.scan(ctx)
			}
		}
	}()
}

func (s *Scanner) scan(ctx context.Context) {
	var span trace.Span
	scanCtx := ctx
	if s.tracer != nil {
		scanCtx, span = s.tracer.Start(ctx, "scanner.scan_stale_trips")
		defer span.End()
	}

	trips, err := s.repo.GetStaleTrips(scanCtx, s.staleAfter)
	if err != nil {
		if s.logger != nil {
			s.logger.ErrorContext(scanCtx, "failed to get stale trips", "error", err.Error())
		}
		if span != nil {
			span.RecordError(err)
		}
		return
	}

	var spanCtx trace.SpanContext
	if span != nil {
		spanCtx = span.SpanContext()
	}

	for _, t := range trips {
		task := Task{
			TripID:      t.ID,
			DriverID:    t.DriverID,
			SpanContext: spanCtx,
		}
		_ = s.queue.TryPush(scanCtx, task)
	}
}

func (s *Scanner) Stop() {
	s.ticker.Stop()
	select {
	case <-s.stopCh:
	default:
		close(s.stopCh)
	}
	s.wg.Wait()
}
