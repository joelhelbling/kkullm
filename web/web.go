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

	// Archived view (older completed/tabled cards beyond the board's cap)
	mux.HandleFunc("GET /ui/archived", ws.handleArchived)

	// Card detail drawer
	mux.HandleFunc("GET /ui/cards/{id}/drawer", ws.handleDrawer)

	// Status change (drag-and-drop or drawer selector)
	mux.HandleFunc("PATCH /ui/cards/{id}/status", ws.handleStatusChange)

	// Add comment to a card
	mux.HandleFunc("POST /ui/cards/{id}/comments", ws.handleAddComment)

	// Delete a card (from the drawer). Gated by RequireAdmin as a
	// chokepoint for future auth.
	mux.Handle("POST /ui/cards/{id}/delete", RequireAdmin(http.HandlerFunc(ws.handleDeleteCard)))

	// Blockers column (all blocked cards across all projects)
	mux.HandleFunc("GET /ui/blockers", ws.handleBlockers)

	// Admin shell + sections (Projects / Agents / Danger Zone).
	// All admin routes are gated by RequireAdmin so future auth lands
	// in one place; today it is a pass-through.
	mux.Handle("GET /admin", RequireAdmin(http.HandlerFunc(ws.handleAdminRoot)))
	mux.Handle("GET /admin/projects", RequireAdmin(http.HandlerFunc(ws.handleAdminProjects)))
	mux.Handle("GET /admin/agents", RequireAdmin(http.HandlerFunc(ws.handleAdminAgents)))
	mux.Handle("GET /admin/danger", RequireAdmin(http.HandlerFunc(ws.handleAdminDanger)))
	mux.Handle("POST /admin/projects/create", RequireAdmin(http.HandlerFunc(ws.handleAdminCreateProject)))
	mux.Handle("POST /admin/projects/{id}/update", RequireAdmin(http.HandlerFunc(ws.handleAdminUpdateProject)))
	mux.Handle("POST /admin/projects/{id}/delete", RequireAdmin(http.HandlerFunc(ws.handleAdminDeleteProject)))
	mux.Handle("POST /admin/agents/create", RequireAdmin(http.HandlerFunc(ws.handleAdminCreateAgent)))
	mux.Handle("POST /admin/agents/{id}/update", RequireAdmin(http.HandlerFunc(ws.handleAdminUpdateAgent)))
	mux.Handle("POST /admin/agents/{id}/delete", RequireAdmin(http.HandlerFunc(ws.handleAdminDeleteAgent)))
	mux.Handle("POST /admin/danger/purge", RequireAdmin(http.HandlerFunc(ws.handleAdminPurge)))

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
