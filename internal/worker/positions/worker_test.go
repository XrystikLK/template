package positions_test

import (
	"context"
	"errors"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
	"go.uber.org/goleak"
	"go.uber.org/mock/gomock"

	"github.com/XrystikLK/template/internal/model"
	"github.com/XrystikLK/template/internal/worker/positions"
)

func TestMain(m *testing.M) {
	goleak.VerifyTestMain(m)
}

type fakeTicker struct {
	ch chan time.Time
}

func newFakeTicker() *fakeTicker {
	return &fakeTicker{ch: make(chan time.Time)}
}

func (f *fakeTicker) Chan() <-chan time.Time {
	return f.ch
}

func (f *fakeTicker) Stop() {}

func (f *fakeTicker) Tick() {
	select {
	case f.ch <- time.Now():
	default:
	}
}

func TestDeduplicationInQueue(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockMetrics := NewMockmetrics(ctrl)
	q := positions.NewQueue(10, mockMetrics, nil)

	tripID := uuid.New()
	task := positions.Task{
		TripID:   tripID,
		DriverID: uuid.New(),
	}

	mockMetrics.EXPECT().SetQueueSize(int64(1))
	mockMetrics.EXPECT().IncStaleChecks(gomock.Any(), "queued")

	pushed := q.TryPush(context.Background(), task)
	require.True(t, pushed)

	mockMetrics.EXPECT().IncStaleChecks(gomock.Any(), "duplicate")

	pushedAgain := q.TryPush(context.Background(), task)
	require.False(t, pushedAgain)
}

func TestCancelTaskByIncomingPosition(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockGateway := NewMockpushGateway(ctrl)
	q := positions.NewQueue(10, nil, nil)
	pool := positions.NewPool(q, mockGateway, 1, time.Second, nil, nil, nil)

	tripID := uuid.New()
	task := positions.Task{
		TripID:   tripID,
		DriverID: uuid.New(),
	}

	pushed := q.TryPush(context.Background(), task)
	require.True(t, pushed)

	q.Cancel(tripID)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	pool.Start(ctx)
	q.Close()
	pool.Wait()
}

func TestQueueOverflowGivesDroppedWithoutBlocking(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockMetrics := NewMockmetrics(ctrl)
	mockLogger := NewMocklogger(ctrl)
	q := positions.NewQueue(1, mockMetrics, mockLogger)

	task1 := positions.Task{TripID: uuid.New(), DriverID: uuid.New()}
	task2 := positions.Task{TripID: uuid.New(), DriverID: uuid.New()}

	mockMetrics.EXPECT().SetQueueSize(int64(1))
	mockMetrics.EXPECT().IncStaleChecks(gomock.Any(), "queued")

	pushed1 := q.TryPush(context.Background(), task1)
	require.True(t, pushed1)

	mockMetrics.EXPECT().IncStaleChecks(gomock.Any(), "dropped")
	mockMetrics.EXPECT().IncPositionRequests(gomock.Any(), "dropped")
	mockLogger.EXPECT().WarnContext(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any())

	pushed2 := q.TryPush(context.Background(), task2)
	require.False(t, pushed2)
}

func TestPoolProcessesNTasksWithWorkers(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockGateway := NewMockpushGateway(ctrl)
	q := positions.NewQueue(10, nil, nil)
	pool := positions.NewPool(q, mockGateway, 3, time.Second, nil, nil, nil)

	const n = 6
	var processed atomic.Int32
	mockGateway.EXPECT().
		RequestPosition(gomock.Any(), gomock.Any(), gomock.Any()).
		Times(n).
		DoAndReturn(func(_ context.Context, _, _ uuid.UUID) error {
			processed.Add(1)
			return nil
		})

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	pool.Start(ctx)

	for i := 0; i < n; i++ {
		pushed := q.TryPush(ctx, positions.Task{
			TripID:   uuid.New(),
			DriverID: uuid.New(),
		})
		require.True(t, pushed)
	}

	q.Close()
	pool.Wait()

	require.Equal(t, int32(n), processed.Load())
}

func TestGracefulShutdownNoGoroutineLeaks(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockRepo := NewMocktripRepository(ctrl)
	mockGateway := NewMockpushGateway(ctrl)
	ticker := newFakeTicker()

	svc := positions.NewService(
		mockRepo,
		mockGateway,
		ticker,
		10,
		2,
		30*time.Second,
		60*time.Second,
		time.Second,
		nil,
		nil,
		nil,
	)

	ctx, cancel := context.WithCancel(context.Background())
	svc.Start(ctx)

	svc.StopScanner()
	cancel()

	shutdownCtx, shutdownCancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer shutdownCancel()

	svc.StopWorkers(shutdownCtx)
}

func TestPushServiceErrorDoesNotCrashWorker(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockGateway := NewMockpushGateway(ctrl)
	q := positions.NewQueue(10, nil, nil)
	pool := positions.NewPool(q, mockGateway, 1, time.Second, nil, nil, nil)

	trip1 := uuid.New()
	trip2 := uuid.New()

	var order []uuid.UUID
	var mu sync.Mutex

	mockGateway.EXPECT().
		RequestPosition(gomock.Any(), gomock.Any(), trip1).
		DoAndReturn(func(_ context.Context, _, _ uuid.UUID) error {
			mu.Lock()
			order = append(order, trip1)
			mu.Unlock()
			return errors.New("push service temporary failure")
		})

	mockGateway.EXPECT().
		RequestPosition(gomock.Any(), gomock.Any(), trip2).
		DoAndReturn(func(_ context.Context, _, _ uuid.UUID) error {
			mu.Lock()
			order = append(order, trip2)
			mu.Unlock()
			return nil
		})

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	pool.Start(ctx)

	pushed := q.TryPush(ctx, positions.Task{TripID: trip1, DriverID: uuid.New()})
	require.True(t, pushed)

	pushed = q.TryPush(ctx, positions.Task{TripID: trip2, DriverID: uuid.New()})
	require.True(t, pushed)

	q.Close()
	pool.Wait()

	mu.Lock()
	defer mu.Unlock()
	require.Equal(t, []uuid.UUID{trip1, trip2}, order)
}

func TestScannerNonOverlappingTicks(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockRepo := NewMocktripRepository(ctrl)
	ticker := newFakeTicker()
	q := positions.NewQueue(10, nil, nil)

	startFirst := make(chan struct{})
	finishFirst := make(chan struct{})
	var callCount atomic.Int32

	mockRepo.EXPECT().
		GetStaleTrips(gomock.Any(), gomock.Any()).
		DoAndReturn(func(_ context.Context, _ time.Duration) ([]model.StaleTrip, error) {
			callCount.Add(1)
			close(startFirst)
			<-finishFirst
			return nil, nil
		})

	scanner := positions.NewScanner(mockRepo, q, ticker, 60*time.Second, nil, nil)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	scanner.Start(ctx)

	go func() {
		ticker.ch <- time.Now()
	}()
	<-startFirst

	ticker.Tick()
	ticker.Tick()

	close(finishFirst)
	scanner.Stop()
	require.Equal(t, int32(1), callCount.Load())
}
