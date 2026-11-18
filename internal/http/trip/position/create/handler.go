package create

import (
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"time"

	"github.com/google/uuid"

	domainTrip "github.com/XrystikLK/template/internal/domain/trip"
	api "github.com/XrystikLK/template/internal/generated"
	"github.com/XrystikLK/template/internal/http/domainerror"
	"github.com/XrystikLK/template/internal/model"
	usecase "github.com/XrystikLK/template/internal/usecase/trip/position/create"
)

const isRequiredMsg = "is required"

type Handler struct {
	uc useCase
}

func New(uc useCase) *Handler {
	return &Handler{uc: uc}
}

type rawCoordinates struct {
	Latitude  *float64 `json:"latitude"`
	Longitude *float64 `json:"longitude"`
}

type rawPositionRequest struct {
	Coordinates *rawCoordinates `json:"coordinates"`
	RecordedAt  *time.Time      `json:"recorded_at"`
}

func (h *Handler) Handle(w http.ResponseWriter, r *http.Request, tripID uuid.UUID) {
	decoder := json.NewDecoder(r.Body)
	decoder.DisallowUnknownFields()

	var req rawPositionRequest
	if err := decoder.Decode(&req); err != nil {
		domainerror.Write(w, r, &domainTrip.ValidationError{
			Field:   "body",
			Message: err.Error(),
		})
		return
	}

	var extra json.RawMessage
	if err := decoder.Decode(&extra); !errors.Is(err, io.EOF) {
		domainerror.Write(w, r, &domainTrip.ValidationError{
			Field:   "body",
			Message: "unexpected data after JSON object",
		})
		return
	}

	if req.Coordinates == nil {
		domainerror.Write(w, r, &domainTrip.ValidationError{Field: "coordinates", Message: isRequiredMsg})
		return
	}
	if req.Coordinates.Latitude == nil {
		domainerror.Write(w, r, &domainTrip.ValidationError{Field: "coordinates.latitude", Message: isRequiredMsg})
		return
	}
	if req.Coordinates.Longitude == nil {
		domainerror.Write(w, r, &domainTrip.ValidationError{Field: "coordinates.longitude", Message: isRequiredMsg})
		return
	}
	if req.RecordedAt == nil {
		domainerror.Write(w, r, &domainTrip.ValidationError{Field: "recorded_at", Message: isRequiredMsg})
		return
	}

	cmd := usecase.Command{
		TripID: tripID,
		Coordinates: model.Coordinates{
			Latitude:  *req.Coordinates.Latitude,
			Longitude: *req.Coordinates.Longitude,
		},
		RecordedAt: *req.RecordedAt,
	}

	pos, err := h.uc.Execute(r.Context(), cmd)
	if err != nil {
		domainerror.Write(w, r, err)
		return
	}

	resp := api.TripPosition{
		Id:     pos.ID,
		TripId: pos.TripID,
		Coordinates: api.Coordinates{
			Latitude:  pos.Coordinates.Latitude,
			Longitude: pos.Coordinates.Longitude,
		},
		RecordedAt: pos.RecordedAt,
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusCreated)
	_ = json.NewEncoder(w).Encode(resp)
}
