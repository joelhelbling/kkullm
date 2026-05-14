package web

import (
	"net/http"

	"github.com/joelhelbling/kkullm/model"
)

type adminProjectRow struct {
	ID         int
	Name       string
	CardCount  int
	AgentCount int
}

type adminProjectsData struct {
	Section  string
	Projects []adminProjectRow
}

type adminAgentsData struct {
	Section string
	Agents  []model.Agent
}

type adminDangerData struct {
	Section string
}

func (ws *WebServer) handleAdminRoot(w http.ResponseWriter, r *http.Request) {
	http.Redirect(w, r, "/admin/projects", http.StatusSeeOther)
}

func (ws *WebServer) handleAdminProjects(w http.ResponseWriter, r *http.Request) {
	projects, err := ws.store.ListProjects()
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	rows := make([]adminProjectRow, 0, len(projects))
	for _, p := range projects {
		cc, err := ws.store.CountCardsForProject(p.ID)
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		ac, err := ws.store.CountAgentsForProject(p.ID)
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		rows = append(rows, adminProjectRow{ID: p.ID, Name: p.Name, CardCount: cc, AgentCount: ac})
	}
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	if err := tmpl.ExecuteTemplate(w, "admin_projects", adminProjectsData{Section: "projects", Projects: rows}); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
	}
}

func (ws *WebServer) handleAdminAgents(w http.ResponseWriter, r *http.Request) {
	agents, err := ws.store.ListAgents("")
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	if err := tmpl.ExecuteTemplate(w, "admin_agents", adminAgentsData{Section: "agents", Agents: agents}); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
	}
}

func (ws *WebServer) handleAdminDanger(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	if err := tmpl.ExecuteTemplate(w, "admin_danger", adminDangerData{Section: "danger"}); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
	}
}
