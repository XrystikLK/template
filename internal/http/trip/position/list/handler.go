package list

import (
	"encoding/json"
	"net/http"

	"github.com/google/uuid"

	api "github.com/XrystikLK/template/internal/generated"
	"github.com/XrystikLK/template/internal/http/domainerror"
)

type Handler struct {
	uc useCase
}

func New(uc useCase) *Handler {
	return &Handler{uc: uc}
}

func (h *Handler) Handle(w http.ResponseWriter, r *http.Request, tripID uuid.UUID) {
	items, err := h.uc.Execute(r.Context(), tripID)
	if err != nil {
		domainerror.Write(w, r, err)
		return
	}

	positions := make([]api.TripPosition, 0, len(items))
	for _, pos := range items {
		positions = append(positions, api.TripPosition{
			Id:     pos.ID,
			TripId: pos.TripID,
			Coordinates: api.Coordinates{
				Latitude:  pos.Coordinates.Latitude,
				Longitude: pos.Coordinates.Longitude,
			},
			RecordedAt: pos.RecordedAt,
		})
	}

	resp := api.TripPositionsResponse{
		Positions: positions,
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	_ = json.NewEncoder(w).Encode(resp)
}
