package positions

import (
	"context"
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
	cancel     context.CancelFunc
	done       chan struct{}
}

func NewScanner(repo tripRepository, queue *Queue, ticker Ticker, staleAfter time.Duration, tracer trace.Tracer, logger logger) *Scanner {
	return &Scanner{
		repo:       repo,
		queue:      queue,
		ticker:     ticker,
		staleAfter: staleAfter,
		tracer:     tracer,
		logger:     logger,
	}
}

func (s *Scanner) Start(ctx context.Context) {
	ctx, s.cancel = context.WithCancel(ctx)
	s.done = make(chan struct{})
	go func() {
		defer close(s.done)
		for {
			select {
			case <-ctx.Done():
				return
			case <-s.ticker.Chan():
				if ctx.Err() == nil {
					s.scan(ctx)
				}
			}
		}
	}()
}

func (s *Scanner) scan(ctx context.Context) {
	s.queue.beginScan(time.Now())
	defer s.queue.endScan()
	var span trace.Span
	scanCtx := ctx
	if s.tracer != nil {
		scanCtx, span = s.tracer.Start(ctx, "scanner.scan_stale_trips")
		defer span.End()
	}

	trips, err := s.repo.GetStaleTrips(scanCtx, s.staleAfter)
	if ctx.Err() != nil {
		return
	}
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
		if scanCtx.Err() != nil {
			return
		}
		task := Task{
			TripID:      t.ID,
			DriverID:    t.DriverID,
			SpanContext: spanCtx,
		}
		_ = s.queue.tryPush(scanCtx, task, true)
	}
}

func (s *Scanner) Stop(ctx context.Context) {
	s.ticker.Stop()
	if s.cancel == nil {
		return
	}
	s.cancel()
	select {
	case <-s.done:
	case <-ctx.Done():
	}
}
