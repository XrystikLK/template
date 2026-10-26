package finish

import (
	"encoding/json"
	"log/slog"
	"net/http"

	"github.com/google/uuid"

	"github.com/XrystikLK/template/internal/http/domainerror"
)

type Handler struct {
	uc      useCase
	logger  *slog.Logger
	metrics metrics
}

func New(uc useCase, logger *slog.Logger, metrics metrics) *Handler {
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
	if h.metrics != nil {
		h.metrics.IncTripsCompleted(r.Context())
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	_ = json.NewEncoder(w).Encode(toAPITrip(tripModel))
}
