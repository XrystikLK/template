package http

import (
	"context"
	"encoding/json"
	"net/http"

	api "github.com/XrystikLK/template/internal/generated"
	createHandler "github.com/XrystikLK/template/internal/http/trip/create"
	finishHandler "github.com/XrystikLK/template/internal/http/trip/finish"
	getHandler "github.com/XrystikLK/template/internal/http/trip/get"
)

type pinger interface {
	Ping(ctx context.Context) error
}

type Server struct {
	api.Unimplemented
	createHandler *createHandler.Handler
	getHandler    *getHandler.Handler
	finishHandler *finishHandler.Handler
	pinger        pinger
}

func NewServer(
	createH *createHandler.Handler,
	getH *getHandler.Handler,
	finishH *finishHandler.Handler,
	pinger pinger,
) *Server {
	return &Server{
		createHandler: createH,
		getHandler:    getH,
		finishHandler: finishH,
		pinger:        pinger,
	}
}

func (s *Server) CreateTrip(w http.ResponseWriter, r *http.Request, _ api.CreateTripParams) {
	s.createHandler.Handle(w, r)
}

func (s *Server) GetTrip(w http.ResponseWriter, r *http.Request, tripId api.TripId) {
	s.getHandler.Handle(w, r, tripId)
}

func (s *Server) FinishTrip(w http.ResponseWriter, r *http.Request, tripId api.TripId) {
	s.finishHandler.Handle(w, r, tripId)
}

func (s *Server) Health(w http.ResponseWriter, _ *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	_ = json.NewEncoder(w).Encode(api.HealthResponse{Status: api.Ok})
}

func (s *Server) Ready(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	if s.pinger != nil {
		if err := s.pinger.Ping(r.Context()); err != nil {
			w.WriteHeader(http.StatusServiceUnavailable)
			_ = json.NewEncoder(w).Encode(api.HealthResponse{Status: api.Unavailable})
			return
		}
	}
	w.WriteHeader(http.StatusOK)
	_ = json.NewEncoder(w).Encode(api.HealthResponse{Status: api.Ok})
}
