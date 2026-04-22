package web

import (
	"encoding/json"
	"net/http"

	"simplehermes/internal/app"
)

type Server struct {
	version string
	service app.Service
}

func NewServer(version string, service app.Service) *Server {
	return &Server{
		version: version,
		service: service,
	}
}

func (s *Server) Handler() http.Handler {
	mux := http.NewServeMux()

	mux.HandleFunc("GET /", s.handleRoot)
	mux.HandleFunc("GET /api/state", s.handleState)
	mux.HandleFunc("POST /api/commands", s.handleCommands)
	mux.HandleFunc("POST /api/settings", s.handleSettings)
	mux.HandleFunc("GET /api/audio/rx", s.handleRXAudio)
	mux.HandleFunc("GET /api/audio/tx", s.handleTXAudio)

	return noStore(mux)
}

func (s *Server) handleRoot(w http.ResponseWriter, _ *http.Request) {
	w.Header().Set("Content-Type", "text/plain; charset=utf-8")
	_, _ = w.Write([]byte("SimpleHermes remote control API " + s.version + "\n"))
}

func (s *Server) handleState(w http.ResponseWriter, r *http.Request) {
	state, err := s.service.State(r.Context())
	if err != nil {
		writeJSON(w, http.StatusBadGateway, state)
		return
	}
	writeJSON(w, http.StatusOK, state)
}

func (s *Server) handleCommands(w http.ResponseWriter, r *http.Request) {
	var cmd app.Command
	if err := json.NewDecoder(r.Body).Decode(&cmd); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	state, err := s.service.Dispatch(r.Context(), cmd)
	if err != nil {
		writeJSON(w, http.StatusBadRequest, state)
		return
	}
	writeJSON(w, http.StatusOK, state)
}

func (s *Server) handleSettings(w http.ResponseWriter, r *http.Request) {
	var update app.SettingsUpdate
	if err := json.NewDecoder(r.Body).Decode(&update); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	state, err := s.service.UpdateSettings(r.Context(), update)
	if err != nil {
		writeJSON(w, http.StatusBadRequest, state)
		return
	}
	writeJSON(w, http.StatusOK, state)
}

func (s *Server) handleRXAudio(w http.ResponseWriter, r *http.Request) {
	s.service.HandleRXAudio(w, r)
}

func (s *Server) handleTXAudio(w http.ResponseWriter, r *http.Request) {
	s.service.HandleTXAudio(w, r)
}

func writeJSON(w http.ResponseWriter, status int, value any) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(value)
}

func noStore(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Cache-Control", "no-store")
		next.ServeHTTP(w, r)
	})
}
