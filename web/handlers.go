package web

import (
	"html/template"
	"log"
	"net/http"

	"github.com/joelhelbling/kkullm/model"
)

var tmpl *template.Template

func initTemplates() {
	var err error
	tmpl, err = template.ParseFS(content, "templates/*.html")
	if err != nil {
		log.Fatalf("parse templates: %v", err)
	}
}

type layoutData struct {
	Projects         []model.Project
	Agents           []model.Agent
	DefaultProjectID int
}

func (ws *WebServer) handleRoot(w http.ResponseWriter, r *http.Request) {
	if r.URL.Path != "/" {
		http.NotFound(w, r)
		return
	}

	projects, err := ws.store.ListProjects()
	if err != nil {
		http.Error(w, err.Error(), 500)
		return
	}

	agents, err := ws.store.ListAgents("")
	if err != nil {
		http.Error(w, err.Error(), 500)
		return
	}

	defaultProjectID := 0
	if len(projects) > 0 {
		defaultProjectID = projects[0].ID
	}

	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	if err := tmpl.ExecuteTemplate(w, "layout.html", layoutData{
		Projects:         projects,
		Agents:           agents,
		DefaultProjectID: defaultProjectID,
	}); err != nil {
		log.Printf("render layout: %v", err)
	}
}
