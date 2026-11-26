package list

import (
	"context"

	"github.com/google/uuid"

	domainTrip "github.com/XrystikLK/template/internal/domain/trip"
	"github.com/XrystikLK/template/internal/model"
)

type UseCase struct {
	repo positionRepository
}

func New(repo positionRepository) *UseCase {
	return &UseCase{repo: repo}
}

func (uc *UseCase) Execute(ctx context.Context, tripID uuid.UUID) ([]model.Position, error) {
	if tripID == uuid.Nil {
		return nil, &domainTrip.ValidationError{Field: "trip_id", Message: "cannot be empty"}
	}
	return uc.repo.ListPositions(ctx, tripID)
}
