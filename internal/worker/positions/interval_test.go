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
			repo.EXPECT().GetStaleTrips(gomock.Any(), time.Minute).
				DoAndReturn(func(context.Context, time.Duration) ([]model.StaleTrip, error) {
					<-release
					return []model.StaleTrip{trip}, nil
				}),
			repo.EXPECT().GetStaleTrips(gomock.Any(), time.Minute).Return([]model.StaleTrip{trip}, nil),
		)
		push := NewMockpushGateway(ctrl)
		calls := 0
		push.EXPECT().RequestPosition(gomock.Any(), trip.DriverID, trip.ID).
			DoAndReturn(func(context.Context, uuid.UUID, uuid.UUID) error { calls++; return nil })

		svc := positions.NewService(repo, push, ticker, 4, 1, time.Second, time.Minute, time.Minute, nil, nil, nil)
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
		repo.EXPECT().GetStaleTrips(gomock.Any(), time.Minute).Return([]model.StaleTrip{first, second}, nil).Times(3)
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

		svc := positions.NewService(repo, push, ticker, 4, 1, time.Second, time.Minute, time.Minute, nil, nil, nil)
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
