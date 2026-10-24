//go:generate go tool mockgen -source ${GOFILE} -destination mocks_test.go -package ${GOPACKAGE}_test
package finish

import (
	"context"
	"time"

	"github.com/XrystikLK/template/internal/model"
	"github.com/google/uuid"
)

type (
	tripRepository interface {
		Finish(ctx context.Context, id uuid.UUID, finishedAt time.Time) (*model.Trip, error)
		AddStatusHistory(ctx context.Context, history model.StatusHistory) error
	}

	txManager interface {
		Do(ctx context.Context, fn func(ctx context.Context) error) error
	}
)

