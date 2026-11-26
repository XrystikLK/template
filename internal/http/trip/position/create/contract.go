//go:generate go tool mockgen -source ${GOFILE} -destination mocks_test.go -package ${GOPACKAGE}_test
package create

import (
	"context"

	"github.com/XrystikLK/template/internal/model"
	usecase "github.com/XrystikLK/template/internal/usecase/trip/position/create"
)

type useCase interface {
	Execute(ctx context.Context, cmd usecase.Command) (*model.Position, error)
}
