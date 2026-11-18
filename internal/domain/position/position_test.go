package position_test

import (
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/require"

	"github.com/XrystikLK/template/internal/domain/position"
	domainTrip "github.com/XrystikLK/template/internal/domain/trip"
)

func TestNewPosition(t *testing.T) {
	const fieldCoordinates = "coordinates"

	now := time.Date(2026, 8, 24, 12, 0, 0, 0, time.UTC)
	validTripID := uuid.New()
	validCoords := position.Coordinates{Latitude: 59.935, Longitude: 30.335}
	validRecordedAt := now.Add(-5 * time.Second)

	tests := []struct {
		name       string
		tripID     uuid.UUID
		coords     position.Coordinates
		recordedAt time.Time
		now        time.Time
		wantField  string
		wantErr    bool
	}{
		{
			name:       "valid position",
			tripID:     validTripID,
			coords:     validCoords,
			recordedAt: validRecordedAt,
			now:        now,
			wantErr:    false,
		},
		{
			name:       "empty trip id",
			tripID:     uuid.Nil,
			coords:     validCoords,
			recordedAt: validRecordedAt,
			now:        now,
			wantField:  "trip_id",
			wantErr:    true,
		},
		{
			name:       "latitude less than -90",
			tripID:     validTripID,
			coords:     position.Coordinates{Latitude: -90.0001, Longitude: 30.335},
			recordedAt: validRecordedAt,
			now:        now,
			wantField:  fieldCoordinates,
			wantErr:    true,
		},
		{
			name:       "latitude greater than 90",
			tripID:     validTripID,
			coords:     position.Coordinates{Latitude: 90.0001, Longitude: 30.335},
			recordedAt: validRecordedAt,
			now:        now,
			wantField:  fieldCoordinates,
			wantErr:    true,
		},
		{
			name:       "longitude less than -180",
			tripID:     validTripID,
			coords:     position.Coordinates{Latitude: 59.935, Longitude: -180.0001},
			recordedAt: validRecordedAt,
			now:        now,
			wantField:  fieldCoordinates,
			wantErr:    true,
		},
		{
			name:       "longitude greater than 180",
			tripID:     validTripID,
			coords:     position.Coordinates{Latitude: 59.935, Longitude: 180.0001},
			recordedAt: validRecordedAt,
			now:        now,
			wantField:  fieldCoordinates,
			wantErr:    true,
		},
		{
			name:       "zero recorded at",
			tripID:     validTripID,
			coords:     validCoords,
			recordedAt: time.Time{},
			now:        now,
			wantField:  "recorded_at",
			wantErr:    true,
		},
		{
			name:       "recorded at in future more than 1 minute",
			tripID:     validTripID,
			coords:     validCoords,
			recordedAt: now.Add(61 * time.Second),
			now:        now,
			wantField:  "recorded_at",
			wantErr:    true,
		},
		{
			name:       "recorded at in future within 1 minute",
			tripID:     validTripID,
			coords:     validCoords,
			recordedAt: now.Add(50 * time.Second),
			now:        now,
			wantErr:    false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			pos, err := position.New(tt.tripID, tt.coords, tt.recordedAt, tt.now)
			if tt.wantErr {
				require.Error(t, err)
				var valErr *domainTrip.ValidationError
				require.ErrorAs(t, err, &valErr)
				require.Equal(t, tt.wantField, valErr.Field)
				require.Nil(t, pos)
			} else {
				require.NoError(t, err)
				require.NotNil(t, pos)
				require.Equal(t, tt.tripID, pos.TripID)
				require.Equal(t, tt.coords.Latitude, pos.Coordinates.Latitude)
				require.Equal(t, tt.coords.Longitude, pos.Coordinates.Longitude)
				require.Equal(t, tt.recordedAt, pos.RecordedAt)
				require.Equal(t, tt.now, pos.CreatedAt)
			}
		})
	}
}
