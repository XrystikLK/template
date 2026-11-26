package positions

import (
	"context"
	"time"

	"go.opentelemetry.io/otel/trace"

	"github.com/XrystikLK/template/internal/model"
)

type Scanner struct {
	repo       tripRepository
	queue      *Queue
	ticker     Ticker
	staleAfter time.Duration
	batchSize  int
	tracer     trace.Tracer
	logger     logger
	cancel     context.CancelFunc
	done       chan struct{}
}

func NewScanner(repo tripRepository, queue *Queue, ticker Ticker, staleAfter time.Duration, batchSize int, tracer trace.Tracer, logger logger) *Scanner {
	if batchSize <= 0 {
		batchSize = 100
	}
	return &Scanner{
		repo:       repo,
		queue:      queue,
		ticker:     ticker,
		staleAfter: staleAfter,
		batchSize:  batchSize,
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
	scanAt := time.Now()
	s.queue.beginScan(scanAt)
	defer s.queue.endScan()
	var span trace.Span
	scanCtx := ctx
	if s.tracer != nil {
		scanCtx, span = s.tracer.Start(ctx, "scanner.scan_stale_trips")
		defer span.End()
	}

	var spanCtx trace.SpanContext
	if span != nil {
		spanCtx = span.SpanContext()
	}

	var rowsRead int64
	if s.queue.metrics != nil {
		defer func() { s.queue.metrics.RecordStaleRowsRead(scanCtx, rowsRead) }()
	}
	page := model.StaleTripPage{ScanAt: scanAt, Limit: s.batchSize}
	for scanCtx.Err() == nil {
		trips, err := s.repo.GetStaleTrips(scanCtx, s.staleAfter, page)
		rowsRead += int64(len(trips))
		if scanCtx.Err() != nil {
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
		for _, t := range trips {
			if scanCtx.Err() != nil {
				return
			}
			_ = s.queue.tryPush(scanCtx, Task{
				TripID: t.ID, DriverID: t.DriverID, LastActivityAt: t.LastActivityAt(), SpanContext: spanCtx,
			}, true)
		}
		if len(trips) < page.Limit {
			return
		}
		page.After = &trips[len(trips)-1]
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
