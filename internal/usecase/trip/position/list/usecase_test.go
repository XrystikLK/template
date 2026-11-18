package list_test

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"

	domainTrip "github.com/XrystikLK/template/internal/domain/trip"
	"github.com/XrystikLK/template/internal/model"
	"github.com/XrystikLK/template/internal/usecase/trip/position/list"
)

func TestListPositionsSuccess(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockRepo := NewMockpositionRepository(ctrl)
	tripID := uuid.New()

	expectedPositions := []model.Position{
		{
			ID:          1,
			TripID:      tripID,
			Coordinates: model.Coordinates{Latitude: 59.935, Longitude: 30.335},
			RecordedAt:  time.Now().UTC().Add(-10 * time.Second),
			CreatedAt:   time.Now().UTC(),
		},
	}

	mockRepo.EXPECT().
		ListPositions(gomock.Any(), tripID).
		Return(expectedPositions, nil)

	uc := list.New(mockRepo)
	res, err := uc.Execute(context.Background(), tripID)

	require.NoError(t, err)
	require.Equal(t, expectedPositions, res)
}

func TestListPositionsEmptyTripID(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockRepo := NewMockpositionRepository(ctrl)
	uc := list.New(mockRepo)

	res, err := uc.Execute(context.Background(), uuid.Nil)
	require.Error(t, err)
	var valErr *domainTrip.ValidationError
	require.ErrorAs(t, err, &valErr)
	require.Equal(t, "trip_id", valErr.Field)
	require.Nil(t, res)
}

func TestListPositionsRepoError(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockRepo := NewMockpositionRepository(ctrl)
	tripID := uuid.New()
	expectedErr := errors.New("db error")

	mockRepo.EXPECT().
		ListPositions(gomock.Any(), tripID).
		Return(nil, expectedErr)

	uc := list.New(mockRepo)
	res, err := uc.Execute(context.Background(), tripID)

	require.ErrorIs(t, err, expectedErr)
	require.Nil(t, res)
}
