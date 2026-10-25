package finish

import (
	"encoding/json"
	"log/slog"
	"net/http"

	"github.com/XrystikLK/template/internal/http/domainerror"
	"github.com/XrystikLK/template/internal/infra/telemetry"
	"github.com/google/uuid"
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

func (h *Handler) Handle(w http.ResponseWriter, r *http.Request, tripID uuid.UUID) {
	tripModel, err := h.uc.Execute(r.Context(), tripID)
	if err != nil {
		domainerror.Write(w, r, err)
		return
	}

	if h.logger != nil {
		h.logger.InfoContext(r.Context(), "trip completed", slog.String("trip_id", tripModel.ID.String()))
	}
	if h.metrics != nil && h.metrics.TripsCompleted != nil {
		h.metrics.TripsCompleted.Add(r.Context(), 1)
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	_ = json.NewEncoder(w).Encode(toAPITrip(tripModel))
}
