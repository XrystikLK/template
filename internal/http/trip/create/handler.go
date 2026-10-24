package create

import (
	"encoding/json"
	"fmt"
	"net/http"

	domainTrip "github.com/XrystikLK/template/internal/domain/trip"
	api "github.com/XrystikLK/template/internal/generated"
	"github.com/XrystikLK/template/internal/http/domainerror"
)

type Handler struct {
	uc useCase
}

func New(uc useCase) *Handler {
	return &Handler{uc: uc}
}

func (h *Handler) Handle(w http.ResponseWriter, r *http.Request) {
	var req api.CreateTripJSONRequestBody
	decoder := json.NewDecoder(r.Body)
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&req); err != nil {
		domainerror.Write(w, r, &domainTrip.ValidationError{
			Field:   "body",
			Message: err.Error(),
		})
		return
	}

	tripModel, err := h.uc.Execute(r.Context(), toUseCaseRequest(req))
	if err != nil {
		domainerror.Write(w, r, err)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("Location", fmt.Sprintf("/api/v1/trips/%s", tripModel.ID.String()))
	w.WriteHeader(http.StatusCreated)
	_ = json.NewEncoder(w).Encode(toAPITrip(tripModel))
}

