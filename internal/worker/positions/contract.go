//go:generate go tool mockgen -source ${GOFILE} -destination mocks_test.go -package ${GOPACKAGE}_test
package positions

import (
	"context"
	"time"

	"github.com/google/uuid"

	"github.com/XrystikLK/template/internal/model"
)

type (
	tripRepository interface {
		GetStaleTrips(ctx context.Context, staleAfter time.Duration) ([]model.StaleTrip, error)
	}

	pushGateway interface {
		RequestPosition(ctx context.Context, driverID, tripID uuid.UUID) error
	}

	logger interface {
		InfoContext(ctx context.Context, msg string, args ...any)
		WarnContext(ctx context.Context, msg string, args ...any)
		ErrorContext(ctx context.Context, msg string, args ...any)
	}

	metrics interface {
		IncStaleChecks(ctx context.Context, result string)
		SetQueueSize(size int64)
		AddActiveWorkers(delta int64)
		IncPositionRequests(ctx context.Context, result string)
		RecordPositionRequestDuration(ctx context.Context, durationSec float64, result string)
	}

	Ticker interface {
		Chan() <-chan time.Time
		Stop()
	}
)
