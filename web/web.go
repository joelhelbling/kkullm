package web

import (
	"embed"
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
	ws := &WebServer{store: s, events: events}
	_ = ws // handlers added in later tasks

	// Static files
	staticFS, _ := fs.Sub(content, "static")
	mux.Handle("GET /static/", http.StripPrefix("/static/", http.FileServer(http.FS(staticFS))))
}
