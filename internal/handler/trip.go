package handler

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"time"

	"github.com/XrystikLK/template/internal/db"
	api "github.com/XrystikLK/template/internal/generated"
	"github.com/XrystikLK/template/internal/repository"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
)

type TripHandler struct {
	api.Unimplemented
	repo      *repository.TripRepository
	txManager db.TxManager
	pool      *pgxpool.Pool
}

func NewTripHandler(repo *repository.TripRepository, txManager db.TxManager, pool *pgxpool.Pool) *TripHandler {
	return &TripHandler{
		repo:      repo,
		txManager: txManager,
		pool:      pool,
	}
}

func writeProblem(w http.ResponseWriter, status int, code, title, detail, instance string) {
	w.Header().Set("Content-Type", "application/problem+json")
	w.WriteHeader(status)

	p := api.Problem{
		Code:   code,
		Status: int32(status),
		Title:  title,
		Type:   fmt.Sprintf("https://tripgo.example/problems/%s", code),
	}
	if detail != "" {
		p.Detail = &detail
	}
	if instance != "" {
		p.Instance = &instance
	}

	_ = json.NewEncoder(w).Encode(p)
}

func toAPITrip(t *repository.Trip) api.Trip {
	return api.Trip{
		Id:       t.ID,
		UserId:   t.UserID,
		DriverId: t.DriverID,
		StartPoint: api.Coordinates{
			Latitude:  t.StartLatitude,
			Longitude: t.StartLongitude,
		},
		EndPoint: api.Coordinates{
			Latitude:  t.EndLatitude,
			Longitude: t.EndLongitude,
		},
		Price:          t.Price,
		Status:         api.TripStatus(t.Status),
		StartedAt:      t.StartedAt,
		FinishedAt:     t.FinishedAt,
		LastPositionAt: nil,
	}
}

func (h *TripHandler) CreateTrip(w http.ResponseWriter, r *http.Request, params api.CreateTripParams) {
	var req api.CreateTripJSONRequestBody
	decoder := json.NewDecoder(r.Body)
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&req); err != nil {
		writeProblem(w, http.StatusBadRequest, "invalid_request", "Invalid request", err.Error(), r.URL.Path)
		return
	}

	if req.UserId == uuid.Nil || req.DriverId == uuid.Nil {
		writeProblem(w, http.StatusBadRequest, "invalid_request", "Invalid request", "user_id and driver_id cannot be empty", r.URL.Path)
		return
	}

	if req.Price < 0 {
		writeProblem(w, http.StatusBadRequest, "invalid_request", "Invalid request", "price must be greater than or equal to 0", r.URL.Path)
		return
	}

	if req.StartPoint.Latitude < -90 || req.StartPoint.Latitude > 90 ||
		req.StartPoint.Longitude < -180 || req.StartPoint.Longitude > 180 ||
		req.EndPoint.Latitude < -90 || req.EndPoint.Latitude > 90 ||
		req.EndPoint.Longitude < -180 || req.EndPoint.Longitude > 180 {
		writeProblem(w, http.StatusBadRequest, "invalid_request", "Invalid request", "coordinates out of valid range", r.URL.Path)
		return
	}

	now := time.Now().UTC()
	tripID := uuid.New()

	trip := &repository.Trip{
		ID:             tripID,
		UserID:         req.UserId,
		DriverID:       req.DriverId,
		StartLatitude:  req.StartPoint.Latitude,
		StartLongitude: req.StartPoint.Longitude,
		EndLatitude:    req.EndPoint.Latitude,
		EndLongitude:   req.EndPoint.Longitude,
		Price:          req.Price,
		Status:         "active",
		StartedAt:      now,
		CreatedAt:      now,
		UpdatedAt:      now,
	}

	err := h.txManager.Do(r.Context(), func(ctx context.Context) error {
		if err := h.repo.CreateTrip(ctx, trip); err != nil {
			return err
		}

		history := &repository.StatusHistory{
			TripID:     tripID,
			FromStatus: nil,
			ToStatus:   "active",
			ChangedAt:  now,
		}
		return h.repo.AddStatusHistory(ctx, history)
	})

	if err != nil {
		if errors.Is(err, repository.ErrDriverBusy) {
			writeProblem(w, http.StatusConflict, "driver_busy", "Driver busy", "Driver already has an active trip", r.URL.Path)
			return
		}
		writeProblem(w, http.StatusInternalServerError, "internal_error", "Internal Server Error", err.Error(), r.URL.Path)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("Location", fmt.Sprintf("/api/v1/trips/%s", trip.ID.String()))
	w.WriteHeader(http.StatusCreated)
	_ = json.NewEncoder(w).Encode(toAPITrip(trip))
}

func (h *TripHandler) GetTrip(w http.ResponseWriter, r *http.Request, tripId api.TripId) {
	trip, err := h.repo.GetTripByID(r.Context(), tripId)
	if err != nil {
		if errors.Is(err, repository.ErrTripNotFound) {
			writeProblem(w, http.StatusNotFound, "trip_not_found", "Trip not found", "Trip was not found", r.URL.Path)
			return
		}
		writeProblem(w, http.StatusInternalServerError, "internal_error", "Internal Server Error", err.Error(), r.URL.Path)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	_ = json.NewEncoder(w).Encode(toAPITrip(trip))
}

func (h *TripHandler) FinishTrip(w http.ResponseWriter, r *http.Request, tripId api.TripId) {
	now := time.Now().UTC()
	var finishedTrip *repository.Trip

	err := h.txManager.Do(r.Context(), func(ctx context.Context) error {
		trip, err := h.repo.FinishTrip(ctx, tripId, now)
		if err != nil {
			return err
		}
		finishedTrip = trip

		fromStatus := "active"
		history := &repository.StatusHistory{
			TripID:     tripId,
			FromStatus: &fromStatus,
			ToStatus:   "completed",
			ChangedAt:  now,
		}
		return h.repo.AddStatusHistory(ctx, history)
	})

	if err != nil {
		if errors.Is(err, repository.ErrTripNotFound) {
			writeProblem(w, http.StatusNotFound, "trip_not_found", "Trip not found", "Trip was not found", r.URL.Path)
			return
		}
		if errors.Is(err, repository.ErrTripAlreadyCompleted) {
			writeProblem(w, http.StatusConflict, "trip_completed", "Trip completed", "Operation is not allowed for a completed trip", r.URL.Path)
			return
		}
		writeProblem(w, http.StatusInternalServerError, "internal_error", "Internal Server Error", err.Error(), r.URL.Path)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	_ = json.NewEncoder(w).Encode(toAPITrip(finishedTrip))
}

func (h *TripHandler) Health(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	_ = json.NewEncoder(w).Encode(api.HealthResponse{Status: api.Ok})
}

func (h *TripHandler) Ready(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	if err := h.pool.Ping(r.Context()); err != nil {
		w.WriteHeader(http.StatusServiceUnavailable)
		_ = json.NewEncoder(w).Encode(api.HealthResponse{Status: api.Unavailable})
		return
	}
	w.WriteHeader(http.StatusOK)
	_ = json.NewEncoder(w).Encode(api.HealthResponse{Status: api.Ok})
}
