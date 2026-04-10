package web

import (
	"embed"
	"fmt"
	"io/fs"
	"net/http"

	"github.com/joelhelbling/kkullm/api"
	"github.com/joelhelbling/kkullm/store"
)

//go:embed static templates
var content embed.FS

type WebServer struct {
	store  *store.Store
	events *api.EventBus
}

func RegisterRoutes(mux *http.ServeMux, s *store.Store, events *api.EventBus) {
	initTemplates()

	ws := &WebServer{store: s, events: events}

	// Root page
	mux.HandleFunc("GET /{$}", ws.handleRoot)

	// Board view
	mux.HandleFunc("GET /ui/board", ws.handleBoard)

	// Card detail drawer
	mux.HandleFunc("GET /ui/cards/{id}/drawer", ws.handleDrawer)

	// Status change (drag-and-drop or drawer selector)
	mux.HandleFunc("PATCH /ui/cards/{id}/status", ws.handleStatusChange)

	// Blockers column (all blocked cards across all projects)
	mux.HandleFunc("GET /ui/blockers", ws.handleBlockers)

	// Static files (no-cache during development so edits are visible on reload)
	staticFS, err := fs.Sub(content, "static")
	if err != nil {
		panic(fmt.Sprintf("web: static subtree missing from embed: %v", err))
	}
	staticHandler := http.StripPrefix("/static/", http.FileServer(http.FS(staticFS)))
	mux.Handle("GET /static/", http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Cache-Control", "no-cache, no-store, must-revalidate")
		staticHandler.ServeHTTP(w, r)
	}))
}
