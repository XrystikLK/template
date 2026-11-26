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
	mockMetrics := NewMockmetrics(ctrl)
	q := positions.NewQueue(10, mockMetrics, nil)
	pool := positions.NewPool(q, mockGateway, 1, time.Second, nil, nil, nil)

	tripID := uuid.New()
	task := positions.Task{
		TripID:   tripID,
		DriverID: uuid.New(),
	}

	gomock.InOrder(
		mockMetrics.EXPECT().SetQueueSize(int64(1)),
		mockMetrics.EXPECT().IncStaleChecks(gomock.Any(), "queued"),
		mockMetrics.EXPECT().SetQueueSize(int64(2)),
		mockMetrics.EXPECT().IncStaleChecks(gomock.Any(), "queued"),
		mockMetrics.EXPECT().SetQueueSize(int64(1)),
		mockMetrics.EXPECT().SetQueueSize(int64(0)),
	)

	pushed := q.TryPush(context.Background(), task)
	require.True(t, pushed)

	q.Cancel(tripID)

	replacement := positions.Task{TripID: tripID, DriverID: uuid.New()}
	require.True(t, q.TryPush(context.Background(), replacement))
	mockGateway.EXPECT().RequestPosition(gomock.Any(), replacement.DriverID, tripID)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	pool.Start(ctx)
	q.Close()
	pool.Wait()
}

func TestCompletedTaskDoesNotRemoveReplacement(t *testing.T) {
	ctrl := gomock.NewController(t)
	gateway := NewMockpushGateway(ctrl)
	queue := positions.NewQueue(2, nil, nil)
	pool := positions.NewPool(queue, gateway, 1, time.Second, nil, nil, nil)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	task := positions.Task{TripID: uuid.New(), DriverID: uuid.New()}
	replacement := positions.Task{TripID: task.TripID, DriverID: uuid.New()}
	started := make(chan struct{})
	release := make(chan struct{})
	gomock.InOrder(
		gateway.EXPECT().RequestPosition(gomock.Any(), task.DriverID, task.TripID).
			DoAndReturn(func(ctx context.Context, _, _ uuid.UUID) error {
				close(started)
				select {
				case <-release:
					return nil
				case <-ctx.Done():
					return ctx.Err()
				}
			}),
		gateway.EXPECT().RequestPosition(gomock.Any(), replacement.DriverID, replacement.TripID),
	)

	require.True(t, queue.TryPush(ctx, task))
	pool.Start(ctx)
	<-started
	queue.Cancel(task.TripID)
	require.True(t, queue.TryPush(ctx, replacement))
	queue.Close()
	close(release)
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
	ticker := NewMockTicker(ctrl)
	ticks := make(chan time.Time)
	ticker.EXPECT().Chan().Return((<-chan time.Time)(ticks)).AnyTimes()
	ticker.EXPECT().Stop()

	svc := positions.NewService(
		mockRepo,
		mockGateway,
		ticker,
		10,
		1,
		30*time.Second,
		60*time.Second,
		time.Minute,
		nil,
		nil,
		nil,
	)

	first := model.StaleTrip{ID: uuid.New(), DriverID: uuid.New()}
	second := model.StaleTrip{ID: uuid.New(), DriverID: uuid.New()}
	requestStarted := make(chan struct{})
	requestStopped := make(chan error, 1)
	scanStarted := make(chan struct{})
	scanStopped := make(chan error, 1)
	gomock.InOrder(
		mockRepo.EXPECT().GetStaleTrips(gomock.Any(), gomock.Any()).
			Return([]model.StaleTrip{first, second}, nil),
		mockRepo.EXPECT().GetStaleTrips(gomock.Any(), gomock.Any()).
			DoAndReturn(func(ctx context.Context, _ time.Duration) ([]model.StaleTrip, error) {
				close(scanStarted)
				<-ctx.Done()
				scanStopped <- ctx.Err()
				return nil, ctx.Err()
			}),
	)
	mockGateway.EXPECT().RequestPosition(gomock.Any(), first.DriverID, first.ID).
		DoAndReturn(func(ctx context.Context, _, _ uuid.UUID) error {
			close(requestStarted)
			<-ctx.Done()
			requestStopped <- ctx.Err()
			return ctx.Err()
		})

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	svc.Start(ctx)
	ticks <- time.Now()
	<-requestStarted
	ticks <- time.Now()
	<-scanStarted

	shutdownCtx, shutdownCancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
	defer shutdownCancel()

	svc.Shutdown(shutdownCtx)
	require.ErrorIs(t, shutdownCtx.Err(), context.DeadlineExceeded)
	require.ErrorIs(t, <-scanStopped, context.Canceled)
	select {
	case err := <-requestStopped:
		require.ErrorIs(t, err, context.Canceled)
	case <-time.After(time.Second):
		t.Fatal("push request was not canceled at shutdown deadline")
	}
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
	ticker := NewMockTicker(ctrl)
	ticks := make(chan time.Time)
	ticker.EXPECT().Chan().Return((<-chan time.Time)(ticks)).AnyTimes()
	ticker.EXPECT().Stop()
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

	ticks <- time.Now()
	<-startFirst

	select {
	case ticks <- time.Now():
		t.Error("scanner accepted a tick during an active scan")
	default:
	}

	close(finishFirst)
	scanner.Stop(context.Background())
	require.Equal(t, int32(1), callCount.Load())
}
