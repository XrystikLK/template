package create_test

import (
	"bytes"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/google/uuid"
	"go.uber.org/mock/gomock"

	domainTrip "github.com/XrystikLK/template/internal/domain/trip"
	api "github.com/XrystikLK/template/internal/generated"
	createHandler "github.com/XrystikLK/template/internal/http/trip/create"
	"github.com/XrystikLK/template/internal/model"
)

func TestCreateTripHandler(t *testing.T) {
	validUserID := uuid.New()
	validDriverID := uuid.New()
	tripID := uuid.New()

	reqBody := api.CreateTripJSONRequestBody{
		UserId:   validUserID,
		DriverId: validDriverID,
		StartPoint: api.Coordinates{
			Latitude:  55.75,
			Longitude: 37.61,
		},
		EndPoint: api.Coordinates{
			Latitude:  55.80,
			Longitude: 37.70,
		},
		Price: 500,
	}

	t.Run("success", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		defer ctrl.Finish()

		mockUC := NewMockuseCase(ctrl)
		now := time.Now().UTC()
		returnedModel := &model.Trip{
			ID:         tripID,
			UserID:     validUserID,
			DriverID:   validDriverID,
			StartPoint: model.Coordinates{Latitude: 55.75, Longitude: 37.61},
			EndPoint:   model.Coordinates{Latitude: 55.80, Longitude: 37.70},
			Price:      500,
			Status:     model.StatusActive,
			StartedAt:  now,
			CreatedAt:  now,
			UpdatedAt:  now,
		}

		mockUC.EXPECT().
			Execute(gomock.Any(), gomock.Any()).
			Return(returnedModel, nil)

		h := createHandler.New(mockUC, nil, nil)

		bodyBytes, _ := json.Marshal(reqBody)
		req := httptest.NewRequest(http.MethodPost, "/api/v1/trips", bytes.NewReader(bodyBytes))
		rec := httptest.NewRecorder()

		h.Handle(rec, req)

		if rec.Code != http.StatusCreated {
			t.Fatalf("expected status 201, got %d", rec.Code)
		}
		if loc := rec.Header().Get("Location"); loc != "/api/v1/trips/"+tripID.String() {
			t.Fatalf("expected location /api/v1/trips/%s, got %s", tripID, loc)
		}
	})

	t.Run("invalid json body", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		defer ctrl.Finish()

		mockUC := NewMockuseCase(ctrl)
		h := createHandler.New(mockUC, nil, nil)

		req := httptest.NewRequest(http.MethodPost, "/api/v1/trips", bytes.NewReader([]byte("{invalid-json")))
		rec := httptest.NewRecorder()

		h.Handle(rec, req)

		if rec.Code != http.StatusBadRequest {
			t.Fatalf("expected status 400, got %d", rec.Code)
		}
	})

	t.Run("driver busy conflict", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		defer ctrl.Finish()

		mockUC := NewMockuseCase(ctrl)
		mockUC.EXPECT().
			Execute(gomock.Any(), gomock.Any()).
			Return(nil, domainTrip.ErrDriverBusy)

		h := createHandler.New(mockUC, nil, nil)

		bodyBytes, _ := json.Marshal(reqBody)
		req := httptest.NewRequest(http.MethodPost, "/api/v1/trips", bytes.NewReader(bodyBytes))
		rec := httptest.NewRecorder()

		h.Handle(rec, req)

		if rec.Code != http.StatusConflict {
			t.Fatalf("expected status 409, got %d", rec.Code)
		}
	})

	t.Run("unexpected error returns 500", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		defer ctrl.Finish()

		mockUC := NewMockuseCase(ctrl)
		mockUC.EXPECT().
			Execute(gomock.Any(), gomock.Any()).
			Return(nil, errors.New("database timeout"))

		h := createHandler.New(mockUC, nil, nil)

		bodyBytes, _ := json.Marshal(reqBody)
		req := httptest.NewRequest(http.MethodPost, "/api/v1/trips", bytes.NewReader(bodyBytes))
		rec := httptest.NewRecorder()

		h.Handle(rec, req)

		if rec.Code != http.StatusInternalServerError {
			t.Fatalf("expected status 500, got %d", rec.Code)
		}
	})
}
