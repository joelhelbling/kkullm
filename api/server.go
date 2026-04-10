package api

import (
	"encoding/json"
	"net/http"

	"github.com/joelhelbling/kkullm/store"
)

type Server struct {
	store  *store.Store
	events *EventBus
}

func NewServer(s *store.Store) *Server {
	return &Server{
		store:  s,
		events: NewEventBus(),
	}
}

func (s *Server) Handler() http.Handler {
	mux := http.NewServeMux()

	// Projects
	mux.HandleFunc("GET /api/projects", s.listProjects)
	mux.HandleFunc("POST /api/projects", s.createProject)
	mux.HandleFunc("GET /api/projects/{id}", s.getProject)

	// Agents
	mux.HandleFunc("GET /api/agents", s.listAgents)
	mux.HandleFunc("POST /api/agents", s.createAgent)
	mux.HandleFunc("GET /api/agents/{id}", s.getAgent)

	// Cards
	mux.HandleFunc("GET /api/cards", s.listCards)
	mux.HandleFunc("POST /api/cards", s.createCard)
	mux.HandleFunc("GET /api/cards/{id}", s.getCard)
	mux.HandleFunc("PATCH /api/cards/{id}", s.updateCard)
	mux.HandleFunc("DELETE /api/cards/{id}", s.deleteCard)

	// Comments
	mux.HandleFunc("GET /api/cards/{id}/comments", s.listComments)
	mux.HandleFunc("POST /api/cards/{id}/comments", s.createComment)

	// Assets
	mux.HandleFunc("GET /api/assets", s.listAssets)
	mux.HandleFunc("POST /api/assets", s.createAsset)
	mux.HandleFunc("GET /api/assets/{id}", s.getAsset)

	// SSE
	mux.HandleFunc("GET /api/events", s.handleSSE)

	return mux
}

func writeJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(v)
}

func writeError(w http.ResponseWriter, status int, msg string) {
	writeJSON(w, status, map[string]string{"error": msg})
}
