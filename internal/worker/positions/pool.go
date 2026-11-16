package positions

import (
	"context"
	"sync"
	"time"

	"go.opentelemetry.io/otel/codes"
	"go.opentelemetry.io/otel/trace"
)

type Pool struct {
	queue        *Queue
	gateway      pushGateway
	tracer       trace.Tracer
	logger       logger
	metrics      metrics
	workersCount int
	reqTimeout   time.Duration
	wg           sync.WaitGroup
}

func NewPool(queue *Queue, gateway pushGateway, workersCount int, reqTimeout time.Duration, tracer trace.Tracer, logger logger, metrics metrics) *Pool {
	if workersCount <= 0 {
		workersCount = 4
	}
	if reqTimeout <= 0 {
		reqTimeout = 1 * time.Second
	}
	return &Pool{
		queue:        queue,
		gateway:      gateway,
		tracer:       tracer,
		logger:       logger,
		metrics:      metrics,
		workersCount: workersCount,
		reqTimeout:   reqTimeout,
	}
}

func (p *Pool) Start(ctx context.Context) {
	for i := 0; i < p.workersCount; i++ {
		p.wg.Add(1)
		go func() {
			defer p.wg.Done()
			for task := range p.queue.Channel() {
				if !p.queue.IsActive(task.TripID) {
					continue
				}

				p.process(ctx, task)
				p.queue.Remove(task.TripID)
			}
		}()
	}
}

func (p *Pool) process(ctx context.Context, task Task) {
	if p.metrics != nil {
		p.metrics.AddActiveWorkers(1)
		defer p.metrics.AddActiveWorkers(-1)
	}

	var opts []trace.SpanStartOption
	if task.SpanContext.IsValid() {
		opts = append(opts, trace.WithLinks(trace.Link{SpanContext: task.SpanContext}))
	}

	var span trace.Span
	procCtx := context.Background()
	if p.tracer != nil {
		procCtx, span = p.tracer.Start(procCtx, "worker.process_position_request", opts...)
		defer span.End()
	}

	reqCtx, cancel := context.WithTimeout(procCtx, p.reqTimeout)
	defer cancel()

	start := time.Now()
	err := p.gateway.RequestPosition(reqCtx, task.DriverID, task.TripID)
	duration := time.Since(start).Seconds()

	if err != nil {
		if span != nil {
			span.RecordError(err)
			span.SetStatus(codes.Error, err.Error())
		}
		if p.logger != nil {
			p.logger.ErrorContext(procCtx, "failed to request position from push service",
				"trip_id", task.TripID.String(),
				"driver_id", task.DriverID.String(),
				"error", err.Error(),
			)
		}
		if p.metrics != nil {
			p.metrics.IncPositionRequests(ctx, "error")
			p.metrics.RecordPositionRequestDuration(ctx, duration, "error")
		}
		return
	}

	if span != nil {
		span.SetStatus(codes.Ok, "success")
	}
	if p.metrics != nil {
		p.metrics.IncPositionRequests(ctx, "success")
		p.metrics.RecordPositionRequestDuration(ctx, duration, "success")
	}
}

func (p *Pool) Wait() {
	p.wg.Wait()
}
