package finish_test

import (
	"context"
	"errors"
	"testing"
	"time"

	domainTrip "github.com/XrystikLK/template/internal/domain/trip"
	"github.com/XrystikLK/template/internal/model"
	"github.com/XrystikLK/template/internal/usecase/trip/finish"
	"github.com/google/uuid"
	"go.uber.org/mock/gomock"
)

func TestFinishTripUseCase(t *testing.T) {
	tripID := uuid.New()
	now := time.Now().UTC()

	t.Run("success", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		defer ctrl.Finish()

		repo := NewMocktripRepository(ctrl)
		tx := NewMocktxManager(ctrl)

		expectedFinished := &model.Trip{
			ID:         tripID,
			Status:     model.StatusCompleted,
			FinishedAt: &now,
		}

		tx.EXPECT().
			Do(gomock.Any(), gomock.Any()).
			DoAndReturn(func(ctx context.Context, fn func(context.Context) error) error {
				return fn(ctx)
			})

		repo.EXPECT().
			Finish(gomock.Any(), tripID, gomock.Any()).
			Return(expectedFinished, nil)

		repo.EXPECT().
			AddStatusHistory(gomock.Any(), gomock.AssignableToTypeOf(model.StatusHistory{})).
			Return(nil)

		uc := finish.New(repo, tx)
		res, err := uc.Execute(context.Background(), tripID)

		if err != nil {
			t.Fatalf("expected nil error, got: %v", err)
		}
		if res == nil || res.Status != model.StatusCompleted {
			t.Fatalf("expected completed status, got: %v", res)
		}
	})

	t.Run("trip already completed", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		defer ctrl.Finish()

		repo := NewMocktripRepository(ctrl)
		tx := NewMocktxManager(ctrl)

		tx.EXPECT().
			Do(gomock.Any(), gomock.Any()).
			DoAndReturn(func(ctx context.Context, fn func(context.Context) error) error {
				return fn(ctx)
			})

		repo.EXPECT().
			Finish(gomock.Any(), tripID, gomock.Any()).
			Return(nil, domainTrip.ErrTripCompleted)

		uc := finish.New(repo, tx)
		_, err := uc.Execute(context.Background(), tripID)

		if !errors.Is(err, domainTrip.ErrTripCompleted) {
			t.Fatalf("expected ErrTripCompleted, got: %v", err)
		}
	})

	t.Run("trip not found", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		defer ctrl.Finish()

		repo := NewMocktripRepository(ctrl)
		tx := NewMocktxManager(ctrl)

		tx.EXPECT().
			Do(gomock.Any(), gomock.Any()).
			DoAndReturn(func(ctx context.Context, fn func(context.Context) error) error {
				return fn(ctx)
			})

		repo.EXPECT().
			Finish(gomock.Any(), tripID, gomock.Any()).
			Return(nil, domainTrip.ErrTripNotFound)

		uc := finish.New(repo, tx)
		_, err := uc.Execute(context.Background(), tripID)

		if !errors.Is(err, domainTrip.ErrTripNotFound) {
			t.Fatalf("expected ErrTripNotFound, got: %v", err)
		}
	})

	t.Run("repository unexpected error", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		defer ctrl.Finish()

		repo := NewMocktripRepository(ctrl)
		tx := NewMocktxManager(ctrl)
		dbErr := errors.New("write timeout")

		tx.EXPECT().
			Do(gomock.Any(), gomock.Any()).
			DoAndReturn(func(ctx context.Context, fn func(context.Context) error) error {
				return fn(ctx)
			})

		repo.EXPECT().
			Finish(gomock.Any(), tripID, gomock.Any()).
			Return(nil, dbErr)

		uc := finish.New(repo, tx)
		_, err := uc.Execute(context.Background(), tripID)

		if !errors.Is(err, dbErr) {
			t.Fatalf("expected dbErr, got: %v", err)
		}
	})
}

