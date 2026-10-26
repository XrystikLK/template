package finish

import (
	"context"
	"time"

	"github.com/google/uuid"

	"github.com/XrystikLK/template/internal/model"
)

type UseCase struct {
	repo      tripRepository
	txManager txManager
}

func New(repo tripRepository, txManager txManager) *UseCase {
	return &UseCase{
		repo:      repo,
		txManager: txManager,
	}
}

func (uc *UseCase) Execute(ctx context.Context, id uuid.UUID) (*model.Trip, error) {
	now := time.Now().UTC()
	var finishedTrip *model.Trip

	err := uc.txManager.Do(ctx, func(txCtx context.Context) error {
		t, err := uc.repo.Finish(txCtx, id, now)
		if err != nil {
			return err
		}
		finishedTrip = t

		fromStatus := model.StatusActive
		history := model.StatusHistory{
			TripID:     id,
			FromStatus: &fromStatus,
			ToStatus:   model.StatusCompleted,
			ChangedAt:  now,
		}
		return uc.repo.AddStatusHistory(txCtx, history)
	})
	if err != nil {
		return nil, err
	}

	return finishedTrip, nil
}
