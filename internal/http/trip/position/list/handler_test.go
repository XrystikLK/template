package list_test

import (
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"

	domainTrip "github.com/XrystikLK/template/internal/domain/trip"
	"github.com/XrystikLK/template/internal/http/trip/position/list"
	"github.com/XrystikLK/template/internal/model"
)

func TestHandlerListPositionsSuccess(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockUC := NewMockuseCase(ctrl)
	tripID := uuid.New()
	recordedAt := time.Date(2026, 8, 24, 12, 0, 5, 0, time.UTC)

	positions := []model.Position{
		{
			ID:     42,
			TripID: tripID,
			Coordinates: model.Coordinates{
				Latitude:  59.9350,
				Longitude: 30.3350,
			},
			RecordedAt: recordedAt,
			CreatedAt:  recordedAt,
		},
	}

	mockUC.EXPECT().
		Execute(gomock.Any(), tripID).
		Return(positions, nil)

	h := list.New(mockUC)

	req := httptest.NewRequest(http.MethodGet, "/api/v1/trips/"+tripID.String()+"/positions", nil)
	w := httptest.NewRecorder()

	h.Handle(w, req, tripID)

	require.Equal(t, http.StatusOK, w.Code)
	require.Equal(t, "application/json", w.Header().Get("Content-Type"))
	require.Contains(t, w.Body.String(), `"id":42`)
}

func TestHandlerListPositionsNotFound(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockUC := NewMockuseCase(ctrl)
	tripID := uuid.New()

	mockUC.EXPECT().
		Execute(gomock.Any(), tripID).
		Return(nil, domainTrip.ErrTripNotFound)

	h := list.New(mockUC)

	req := httptest.NewRequest(http.MethodGet, "/api/v1/trips/"+tripID.String()+"/positions", nil)
	w := httptest.NewRecorder()

	h.Handle(w, req, tripID)

	require.Equal(t, http.StatusNotFound, w.Code)
	require.Contains(t, w.Body.String(), "trip_not_found")
}

func TestHandlerListPositionsInternalError(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockUC := NewMockuseCase(ctrl)
	tripID := uuid.New()

	mockUC.EXPECT().
		Execute(gomock.Any(), tripID).
		Return(nil, errors.New("db failure"))

	h := list.New(mockUC)

	req := httptest.NewRequest(http.MethodGet, "/api/v1/trips/"+tripID.String()+"/positions", nil)
	w := httptest.NewRecorder()

	h.Handle(w, req, tripID)

	require.Equal(t, http.StatusInternalServerError, w.Code)
	require.Contains(t, w.Body.String(), "internal_error")
}
