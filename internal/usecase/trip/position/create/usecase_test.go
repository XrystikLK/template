package create_test

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
	"github.com/XrystikLK/template/internal/usecase/trip/position/create"
)

func TestCreatePositionSuccess(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockRepo := NewMockpositionRepository(ctrl)
	mockTx := NewMocktxManager(ctrl)
	mockMetrics := NewMockmetrics(ctrl)
	mockNotifier := NewMockcancellationNotifier(ctrl)

	tripID := uuid.New()
	recordedAt := time.Now().UTC().Add(-5 * time.Second)
	cmd := create.Command{
		TripID:      tripID,
		Coordinates: model.Coordinates{Latitude: 59.935, Longitude: 30.335},
		RecordedAt:  recordedAt,
	}

	expectedResult := &model.Position{
		ID:          42,
		TripID:      tripID,
		Coordinates: cmd.Coordinates,
		RecordedAt:  recordedAt,
		CreatedAt:   time.Now().UTC(),
	}

	mockTx.EXPECT().
		Do(gomock.Any(), gomock.Any()).
		DoAndReturn(func(ctx context.Context, fn func(context.Context) error) error {
			return fn(ctx)
		})

	mockRepo.EXPECT().
		SavePosition(gomock.Any(), tripID, gomock.Any()).
		Return(expectedResult, nil)

	mockMetrics.EXPECT().IncPositionsSaved(gomock.Any())
	mockNotifier.EXPECT().Cancel(tripID)

	uc := create.New(mockRepo, mockTx, mockMetrics, mockNotifier)
	res, err := uc.Execute(context.Background(), cmd)

	require.NoError(t, err)
	require.Equal(t, expectedResult, res)
}

func TestCreatePositionSuccessNilOptionalDependencies(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockRepo := NewMockpositionRepository(ctrl)
	mockTx := NewMocktxManager(ctrl)

	tripID := uuid.New()
	recordedAt := time.Now().UTC().Add(-5 * time.Second)
	cmd := create.Command{
		TripID:      tripID,
		Coordinates: model.Coordinates{Latitude: 59.935, Longitude: 30.335},
		RecordedAt:  recordedAt,
	}

	expectedResult := &model.Position{
		ID:          42,
		TripID:      tripID,
		Coordinates: cmd.Coordinates,
		RecordedAt:  recordedAt,
		CreatedAt:   time.Now().UTC(),
	}

	mockTx.EXPECT().
		Do(gomock.Any(), gomock.Any()).
		DoAndReturn(func(ctx context.Context, fn func(context.Context) error) error {
			return fn(ctx)
		})

	mockRepo.EXPECT().
		SavePosition(gomock.Any(), tripID, gomock.Any()).
		Return(expectedResult, nil)

	uc := create.New(mockRepo, mockTx, nil, nil)
	res, err := uc.Execute(context.Background(), cmd)

	require.NoError(t, err)
	require.Equal(t, expectedResult, res)
}

func TestCreatePositionValidationError(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockRepo := NewMockpositionRepository(ctrl)
	mockTx := NewMocktxManager(ctrl)

	cmd := create.Command{
		TripID:      uuid.Nil,
		Coordinates: model.Coordinates{Latitude: 59.935, Longitude: 30.335},
		RecordedAt:  time.Now().UTC(),
	}

	uc := create.New(mockRepo, mockTx, nil, nil)
	res, err := uc.Execute(context.Background(), cmd)

	require.Error(t, err)
	var valErr *domainTrip.ValidationError
	require.ErrorAs(t, err, &valErr)
	require.Equal(t, "trip_id", valErr.Field)
	require.Nil(t, res)
}

func TestCreatePositionTxError(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockRepo := NewMockpositionRepository(ctrl)
	mockTx := NewMocktxManager(ctrl)

	tripID := uuid.New()
	cmd := create.Command{
		TripID:      tripID,
		Coordinates: model.Coordinates{Latitude: 59.935, Longitude: 30.335},
		RecordedAt:  time.Now().UTC().Add(-5 * time.Second),
	}

	expectedErr := errors.New("db error")
	mockTx.EXPECT().
		Do(gomock.Any(), gomock.Any()).
		Return(expectedErr)

	uc := create.New(mockRepo, mockTx, nil, nil)
	res, err := uc.Execute(context.Background(), cmd)

	require.ErrorIs(t, err, expectedErr)
	require.Nil(t, res)
}

func TestCreatePositionRepoErrorInsideTx(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockRepo := NewMockpositionRepository(ctrl)
	mockTx := NewMocktxManager(ctrl)

	tripID := uuid.New()
	cmd := create.Command{
		TripID:      tripID,
		Coordinates: model.Coordinates{Latitude: 59.935, Longitude: 30.335},
		RecordedAt:  time.Now().UTC().Add(-5 * time.Second),
	}

	expectedErr := errors.New("repo save error")
	mockTx.EXPECT().
		Do(gomock.Any(), gomock.Any()).
		DoAndReturn(func(ctx context.Context, fn func(context.Context) error) error {
			return fn(ctx)
		})

	mockRepo.EXPECT().
		SavePosition(gomock.Any(), tripID, gomock.Any()).
		Return(nil, expectedErr)

	uc := create.New(mockRepo, mockTx, nil, nil)
	res, err := uc.Execute(context.Background(), cmd)

	require.ErrorIs(t, err, expectedErr)
	require.Nil(t, res)
}
