package create_test

import (
	"bytes"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"

	domainTrip "github.com/XrystikLK/template/internal/domain/trip"
	"github.com/XrystikLK/template/internal/http/trip/position/create"
	"github.com/XrystikLK/template/internal/model"
)

func TestHandlerCreatePositionSuccess(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockUC := NewMockuseCase(ctrl)
	tripID := uuid.New()
	recordedAt := time.Date(2026, 8, 24, 12, 0, 5, 0, time.UTC)

	expectedPos := &model.Position{
		ID:     42,
		TripID: tripID,
		Coordinates: model.Coordinates{
			Latitude:  59.9350,
			Longitude: 30.3350,
		},
		RecordedAt: recordedAt,
		CreatedAt:  time.Now().UTC(),
	}

	mockUC.EXPECT().
		Execute(gomock.Any(), gomock.Any()).
		Return(expectedPos, nil)

	h := create.New(mockUC)

	body := `{"coordinates":{"latitude":59.9350,"longitude":30.3350},"recorded_at":"2026-08-24T12:00:05Z"}`
	req := httptest.NewRequest(http.MethodPost, "/api/v1/trips/"+tripID.String()+"/positions", bytes.NewBufferString(body))
	w := httptest.NewRecorder()

	h.Handle(w, req, tripID)

	require.Equal(t, http.StatusCreated, w.Code)
	require.Equal(t, "application/json", w.Header().Get("Content-Type"))
	require.Contains(t, w.Body.String(), `"id":42`)
}

func TestHandlerCreatePositionInvalidJSON(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockUC := NewMockuseCase(ctrl)
	h := create.New(mockUC)

	tripID := uuid.New()
	req := httptest.NewRequest(http.MethodPost, "/api/v1/trips/"+tripID.String()+"/positions", bytes.NewBufferString(`{invalid json`))
	w := httptest.NewRecorder()

	h.Handle(w, req, tripID)

	require.Equal(t, http.StatusBadRequest, w.Code)
	require.Contains(t, w.Body.String(), "invalid_request")
}

func TestHandlerCreatePositionTrailingGarbage(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockUC := NewMockuseCase(ctrl)
	h := create.New(mockUC)

	tripID := uuid.New()
	body := `{"coordinates":{"latitude":59.9350,"longitude":30.3350},"recorded_at":"2026-08-24T12:00:05Z"} {"extra":1}`
	req := httptest.NewRequest(http.MethodPost, "/api/v1/trips/"+tripID.String()+"/positions", bytes.NewBufferString(body))
	w := httptest.NewRecorder()

	h.Handle(w, req, tripID)

	require.Equal(t, http.StatusBadRequest, w.Code)
	require.Contains(t, w.Body.String(), "invalid_request")
}

func TestHandlerCreatePositionMissingFields(t *testing.T) {
	tests := []struct {
		name string
		body string
	}{
		{
			name: "missing coordinates",
			body: `{"recorded_at":"2026-08-24T12:00:05Z"}`,
		},
		{
			name: "missing latitude",
			body: `{"coordinates":{"longitude":30.3350},"recorded_at":"2026-08-24T12:00:05Z"}`,
		},
		{
			name: "missing longitude",
			body: `{"coordinates":{"latitude":59.9350},"recorded_at":"2026-08-24T12:00:05Z"}`,
		},
		{
			name: "missing recorded_at",
			body: `{"coordinates":{"latitude":59.9350,"longitude":30.3350}}`,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctrl := gomock.NewController(t)
			defer ctrl.Finish()

			mockUC := NewMockuseCase(ctrl)
			h := create.New(mockUC)

			tripID := uuid.New()
			req := httptest.NewRequest(http.MethodPost, "/api/v1/trips/"+tripID.String()+"/positions", bytes.NewBufferString(tt.body))
			w := httptest.NewRecorder()

			h.Handle(w, req, tripID)

			require.Equal(t, http.StatusBadRequest, w.Code)
			require.Contains(t, w.Body.String(), "invalid_request")
		})
	}
}

func TestHandlerCreatePositionDomainErrors(t *testing.T) {
	tests := []struct {
		name       string
		ucErr      error
		wantStatus int
		wantCode   string
	}{
		{
			name:       "not found",
			ucErr:      domainTrip.ErrTripNotFound,
			wantStatus: http.StatusNotFound,
			wantCode:   "trip_not_found",
		},
		{
			name:       "completed",
			ucErr:      domainTrip.ErrTripCompleted,
			wantStatus: http.StatusConflict,
			wantCode:   "trip_completed",
		},
		{
			name:       "validation error",
			ucErr:      &domainTrip.ValidationError{Field: "coordinates", Message: "invalid"},
			wantStatus: http.StatusBadRequest,
			wantCode:   "invalid_request",
		},
		{
			name:       "internal error",
			ucErr:      errors.New("db down"),
			wantStatus: http.StatusInternalServerError,
			wantCode:   "internal_error",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctrl := gomock.NewController(t)
			defer ctrl.Finish()

			mockUC := NewMockuseCase(ctrl)
			mockUC.EXPECT().
				Execute(gomock.Any(), gomock.Any()).
				Return(nil, tt.ucErr)

			h := create.New(mockUC)
			tripID := uuid.New()
			body := `{"coordinates":{"latitude":59.9350,"longitude":30.3350},"recorded_at":"2026-08-24T12:00:05Z"}`
			req := httptest.NewRequest(http.MethodPost, "/api/v1/trips/"+tripID.String()+"/positions", bytes.NewBufferString(body))
			w := httptest.NewRecorder()

			h.Handle(w, req, tripID)

			require.Equal(t, tt.wantStatus, w.Code)
			require.Contains(t, w.Body.String(), tt.wantCode)
		})
	}
}
