package model

import (
	"time"

	"github.com/google/uuid"
)

type Position struct {
	ID          int64
	TripID      uuid.UUID
	Coordinates Coordinates
	RecordedAt  time.Time
	CreatedAt   time.Time
}

type StaleTrip struct {
	ID             uuid.UUID
	DriverID       uuid.UUID
	LastPositionAt *time.Time
	StartedAt      time.Time
}
