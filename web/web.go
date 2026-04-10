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

	// Static files
	staticFS, err := fs.Sub(content, "static")
	if err != nil {
		panic(fmt.Sprintf("web: static subtree missing from embed: %v", err))
	}
	mux.Handle("GET /static/", http.StripPrefix("/static/", http.FileServer(http.FS(staticFS))))
}
