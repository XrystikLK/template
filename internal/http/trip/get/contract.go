//go:generate go tool mockgen -source ${GOFILE} -destination mocks_test.go -package ${GOPACKAGE}_test
package get

import (
	"context"

	"github.com/XrystikLK/template/internal/model"
	"github.com/google/uuid"
)

type useCase interface {
	Execute(ctx context.Context, id uuid.UUID) (*model.Trip, error)
}

