package position

import (
	"time"

	"github.com/google/uuid"

	domainTrip "github.com/XrystikLK/template/internal/domain/trip"
)

type Coordinates struct {
	Latitude  float64
	Longitude float64
}

type Position struct {
	ID          int64
	TripID      uuid.UUID
	Coordinates Coordinates
	RecordedAt  time.Time
	CreatedAt   time.Time
}

func New(tripID uuid.UUID, coords Coordinates, recordedAt, now time.Time) (*Position, error) {
	if tripID == uuid.Nil {
		return nil, &domainTrip.ValidationError{Field: "trip_id", Message: "cannot be empty"}
	}
	if coords.Latitude < -90 || coords.Latitude > 90 || coords.Longitude < -180 || coords.Longitude > 180 {
		return nil, &domainTrip.ValidationError{Field: "coordinates", Message: "coordinates out of valid range"}
	}
	if recordedAt.IsZero() {
		return nil, &domainTrip.ValidationError{Field: "recorded_at", Message: "cannot be empty"}
	}
	if recordedAt.After(now.Add(1 * time.Minute)) {
		return nil, &domainTrip.ValidationError{Field: "recorded_at", Message: "recorded_at cannot be more than 1 minute in the future"}
	}

	return &Position{
		TripID:      tripID,
		Coordinates: coords,
		RecordedAt:  recordedAt,
		CreatedAt:   now,
	}, nil
}
