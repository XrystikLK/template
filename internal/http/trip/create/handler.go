package create

import (
	"encoding/json"
	"fmt"
	"log/slog"
	"net/http"

	domainTrip "github.com/XrystikLK/template/internal/domain/trip"
	api "github.com/XrystikLK/template/internal/generated"
	"github.com/XrystikLK/template/internal/http/domainerror"
	"github.com/XrystikLK/template/internal/infra/telemetry"
)

type Handler struct {
	uc      useCase
	logger  *slog.Logger
	metrics *telemetry.Metrics
}

func New(uc useCase, logger *slog.Logger, metrics *telemetry.Metrics) *Handler {
	return &Handler{
		uc:      uc,
		logger:  logger,
		metrics: metrics,
	}
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

	if h.logger != nil {
		h.logger.InfoContext(r.Context(), "trip created", slog.String("trip_id", tripModel.ID.String()))
	}
	if h.metrics != nil && h.metrics.TripsCreated != nil {
		h.metrics.TripsCreated.Add(r.Context(), 1)
	}

	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("Location", fmt.Sprintf("/api/v1/trips/%s", tripModel.ID.String()))
	w.WriteHeader(http.StatusCreated)
	_ = json.NewEncoder(w).Encode(toAPITrip(tripModel))
}
