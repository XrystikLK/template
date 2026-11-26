//go:generate go tool mockgen -source ${GOFILE} -destination mocks_test.go -package ${GOPACKAGE}_test
package create

import (
	"context"

	"github.com/google/uuid"

	"github.com/XrystikLK/template/internal/model"
)

type (
	positionRepository interface {
		SavePosition(ctx context.Context, tripID uuid.UUID, pos model.Position) (*model.Position, error)
	}

	txManager interface {
		Do(ctx context.Context, fn func(ctx context.Context) error) error
	}

	metrics interface {
		IncPositionsSaved(ctx context.Context)
	}

	cancellationNotifier interface {
		Cancel(tripID uuid.UUID)
	}
)
