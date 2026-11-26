package positions_test

import (
	"context"
	"testing"
	"testing/synctest"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"

	"github.com/XrystikLK/template/internal/model"
	"github.com/XrystikLK/template/internal/worker/positions"
)

func TestScanReadsAll250TripsInThreeBatches(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		ctrl := gomock.NewController(t)
		ctx := t.Context()
		trips := make([]model.StaleTrip, 250)
		scanAt := time.Now()
		for i := range trips {
			trips[i] = model.StaleTrip{ID: uuid.New(), DriverID: uuid.New(), StartedAt: scanAt.Add(time.Duration(i-300) * time.Second)}
		}
		page := model.StaleTripPage{ScanAt: scanAt, Limit: 100}
		repo := NewMocktripRepository(ctrl)
		gomock.InOrder(
			repo.EXPECT().GetStaleTrips(gomock.Any(), time.Minute, page).Return(trips[:100], nil),
			repo.EXPECT().GetStaleTrips(gomock.Any(), time.Minute, model.StaleTripPage{
				ScanAt: scanAt, Limit: 100, After: &trips[99],
			}).Return(trips[100:200], nil),
			repo.EXPECT().GetStaleTrips(gomock.Any(), time.Minute, model.StaleTripPage{
				ScanAt: scanAt, Limit: 100, After: &trips[199],
			}).Return(trips[200:], nil),
		)
		metrics := NewMockmetrics(ctrl)
		metrics.EXPECT().SetQueueSize(gomock.Any()).AnyTimes()
		metrics.EXPECT().IncStaleChecks(gomock.Any(), "queued").Times(len(trips))
		metrics.EXPECT().RecordStaleRowsRead(gomock.Any(), int64(len(trips)))
		queue := positions.NewQueue(len(trips), metrics, nil)
		ticks := make(chan time.Time)
		ticker := NewMockTicker(ctrl)
		ticker.EXPECT().Chan().Return((<-chan time.Time)(ticks)).AnyTimes()
		ticker.EXPECT().Stop()
		scanner := positions.NewScanner(repo, queue, ticker, time.Minute, 100, nil, nil)
		scanner.Start(ctx)
		ticks <- scanAt
		synctest.Wait()
		scanner.Stop(ctx)
		queue.Close()

		push := NewMockpushGateway(ctrl)
		for _, trip := range trips {
			push.EXPECT().RequestPosition(gomock.Any(), trip.DriverID, trip.ID)
		}
		pool := positions.NewPool(queue, push, 1, time.Second, nil, nil, nil)
		pool.Start(ctx)
		pool.Wait()
	})
}

func TestPriorityQueueEvictsNewestAndProcessesOldestFirst(t *testing.T) {
	ctrl := gomock.NewController(t)
	ctx := t.Context()
	queue := positions.NewQueue(3, nil, nil)
	now := time.Now()
	oldest := positions.Task{TripID: uuid.New(), LastActivityAt: now.Add(-3 * time.Minute)}
	older := positions.Task{TripID: uuid.New(), LastActivityAt: now.Add(-2 * time.Minute)}
	recent := positions.Task{TripID: uuid.New(), LastActivityAt: now.Add(-time.Minute)}
	newest := positions.Task{TripID: uuid.New(), LastActivityAt: now}

	require.True(t, queue.TryPush(ctx, recent))
	require.True(t, queue.TryPush(ctx, older))
	require.True(t, queue.TryPush(ctx, newest))
	require.True(t, queue.TryPush(ctx, oldest))
	require.False(t, queue.TryPush(ctx, older))
	require.False(t, queue.TryPush(ctx, newest))
	queue.Cancel(recent.TripID)
	require.True(t, queue.TryPush(ctx, newest))
	queue.Close()

	push := NewMockpushGateway(ctrl)
	gomock.InOrder(
		push.EXPECT().RequestPosition(gomock.Any(), oldest.DriverID, oldest.TripID),
		push.EXPECT().RequestPosition(gomock.Any(), older.DriverID, older.TripID),
		push.EXPECT().RequestPosition(gomock.Any(), newest.DriverID, newest.TripID),
	)
	pool := positions.NewPool(queue, push, 1, time.Second, nil, nil, nil)
	pool.Start(ctx)
	pool.Wait()
}

func TestCoordinateSavedDuringScanInvalidatesSnapshot(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		ctrl := gomock.NewController(t)
		ticks := make(chan time.Time)
		ticker := NewMockTicker(ctrl)
		ticker.EXPECT().Chan().Return((<-chan time.Time)(ticks)).AnyTimes()
		ticker.EXPECT().Stop()
		trip := model.StaleTrip{ID: uuid.New(), DriverID: uuid.New()}
		release := make(chan struct{})
		repo := NewMocktripRepository(ctrl)
		gomock.InOrder(
			repo.EXPECT().GetStaleTrips(gomock.Any(), time.Minute, gomock.Any()).
				DoAndReturn(func(context.Context, time.Duration, model.StaleTripPage) ([]model.StaleTrip, error) {
					<-release
					return []model.StaleTrip{trip}, nil
				}),
			repo.EXPECT().GetStaleTrips(gomock.Any(), time.Minute, gomock.Any()).Return([]model.StaleTrip{trip}, nil),
		)
		push := NewMockpushGateway(ctrl)
		calls := 0
		push.EXPECT().RequestPosition(gomock.Any(), trip.DriverID, trip.ID).
			DoAndReturn(func(context.Context, uuid.UUID, uuid.UUID) error { calls++; return nil })

		svc := positions.NewService(repo, push, ticker, 4, 1, time.Second, time.Minute, 100, time.Minute, nil, nil, nil)
		svc.Start(t.Context())
		defer svc.Shutdown(t.Context())
		ticks <- time.Now()
		synctest.Wait()
		svc.Cancel(trip.ID)
		close(release)
		synctest.Wait()
		require.Zero(t, calls)
		ticks <- time.Now()
		synctest.Wait()
		require.Equal(t, 1, calls)
	})
}

func TestPoolHonorsConfiguredParallelism(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		ctrl := gomock.NewController(t)
		gateway := NewMockpushGateway(ctrl)
		queue := positions.NewQueue(4, nil, nil)
		pool := positions.NewPool(queue, gateway, 3, time.Minute, nil, nil, nil)
		started := make(chan struct{}, 4)
		release := make(chan struct{})
		gateway.EXPECT().RequestPosition(gomock.Any(), gomock.Any(), gomock.Any()).
			DoAndReturn(func(context.Context, uuid.UUID, uuid.UUID) error {
				started <- struct{}{}
				<-release
				return nil
			}).Times(4)

		for range 4 {
			require.True(t, queue.TryPush(t.Context(), positions.Task{TripID: uuid.New()}))
		}
		queue.Close()
		pool.Start(t.Context())
		defer pool.Wait()
		defer close(release)
		synctest.Wait()
		require.Len(t, started, 3)
		release <- struct{}{}
		synctest.Wait()
		require.Len(t, started, 4)
	})
}

func TestDelayedWorkerRespectsCheckInterval(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		ctrl := gomock.NewController(t)
		ticks := make(chan time.Time)
		ticker := NewMockTicker(ctrl)
		ticker.EXPECT().Chan().Return((<-chan time.Time)(ticks)).AnyTimes()
		ticker.EXPECT().Stop()
		first := model.StaleTrip{ID: uuid.New(), DriverID: uuid.New()}
		second := model.StaleTrip{ID: uuid.New(), DriverID: uuid.New()}
		repo := NewMocktripRepository(ctrl)
		repo.EXPECT().GetStaleTrips(gomock.Any(), time.Minute, gomock.Any()).Return([]model.StaleTrip{first, second}, nil).Times(3)
		push := NewMockpushGateway(ctrl)
		release := make(chan struct{})
		push.EXPECT().RequestPosition(gomock.Any(), first.DriverID, first.ID).
			DoAndReturn(func(context.Context, uuid.UUID, uuid.UUID) error { <-release; return nil })
		push.EXPECT().RequestPosition(gomock.Any(), first.DriverID, first.ID).Times(2)
		var calls []time.Time
		push.EXPECT().RequestPosition(gomock.Any(), second.DriverID, second.ID).
			DoAndReturn(func(context.Context, uuid.UUID, uuid.UUID) error {
				calls = append(calls, time.Now())
				return nil
			}).Times(2)

		svc := positions.NewService(repo, push, ticker, 4, 1, time.Second, time.Minute, 100, time.Minute, nil, nil, nil)
		svc.Start(t.Context())
		defer svc.Shutdown(t.Context())
		ticks <- time.Now()
		synctest.Wait()
		<-time.After(800 * time.Millisecond)
		close(release)
		synctest.Wait()
		<-time.After(200 * time.Millisecond)
		ticks <- time.Now()
		synctest.Wait()
		require.Len(t, calls, 1)
		<-time.After(time.Second)
		ticks <- time.Now()
		synctest.Wait()
		require.Len(t, calls, 2)
		require.GreaterOrEqual(t, calls[1].Sub(calls[0]), time.Second)
	})
}
