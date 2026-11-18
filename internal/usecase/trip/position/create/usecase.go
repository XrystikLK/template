package create

import (
	"context"
	"time"

	"github.com/google/uuid"

	domainPosition "github.com/XrystikLK/template/internal/domain/position"
	"github.com/XrystikLK/template/internal/model"
)

type Command struct {
	TripID      uuid.UUID
	Coordinates model.Coordinates
	RecordedAt  time.Time
}

type UseCase struct {
	repo                 positionRepository
	txManager            txManager
	metrics              metrics
	cancellationNotifier cancellationNotifier
}

func New(repo positionRepository, txManager txManager, metrics metrics, cancellationNotifier cancellationNotifier) *UseCase {
	return &UseCase{
		repo:                 repo,
		txManager:            txManager,
		metrics:              metrics,
		cancellationNotifier: cancellationNotifier,
	}
}

func (uc *UseCase) Execute(ctx context.Context, cmd Command) (*model.Position, error) {
	now := time.Now().UTC()

	domainEntity, err := domainPosition.New(
		cmd.TripID,
		domainPosition.Coordinates{
			Latitude:  cmd.Coordinates.Latitude,
			Longitude: cmd.Coordinates.Longitude,
		},
		cmd.RecordedAt,
		now,
	)
	if err != nil {
		return nil, err
	}

	posModel := model.Position{
		TripID:      domainEntity.TripID,
		Coordinates: cmd.Coordinates,
		RecordedAt:  domainEntity.RecordedAt,
		CreatedAt:   domainEntity.CreatedAt,
	}

	var saved *model.Position
	err = uc.txManager.Do(ctx, func(txCtx context.Context) error {
		var saveErr error
		saved, saveErr = uc.repo.SavePosition(txCtx, cmd.TripID, posModel)
		return saveErr
	})
	if err != nil {
		return nil, err
	}

	if uc.metrics != nil {
		uc.metrics.IncPositionsSaved(ctx)
	}
	if uc.cancellationNotifier != nil {
		uc.cancellationNotifier.Cancel(cmd.TripID)
	}

	return saved, nil
}
