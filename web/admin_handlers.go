package web

import (
	"database/sql"
	"errors"
	"net/http"
	"strconv"
	"strings"

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

const purgePhrase = "PURGE DATABASE"

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

func (ws *WebServer) handleAdminRenameProject(w http.ResponseWriter, r *http.Request) {
	id, err := strconv.Atoi(r.PathValue("id"))
	if err != nil {
		http.Error(w, "bad id", http.StatusBadRequest)
		return
	}
	if err := r.ParseForm(); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	name := strings.TrimSpace(r.FormValue("name"))
	if err := ws.store.RenameProject(id, name); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	http.Redirect(w, r, "/admin/projects", http.StatusSeeOther)
}

func (ws *WebServer) handleAdminDeleteProject(w http.ResponseWriter, r *http.Request) {
	id, err := strconv.Atoi(r.PathValue("id"))
	if err != nil {
		http.Error(w, "bad id", http.StatusBadRequest)
		return
	}
	if err := r.ParseForm(); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	p, err := ws.store.GetProject(id)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			// Stale id — recover by redirecting back to list.
			http.Redirect(w, r, "/admin/projects", http.StatusSeeOther)
			return
		}
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	if r.FormValue("confirm") != p.Name {
		http.Error(w, "confirmation does not match project name", http.StatusBadRequest)
		return
	}
	if err := ws.store.DeleteProject(id); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	http.Redirect(w, r, "/admin/projects", http.StatusSeeOther)
}

func (ws *WebServer) handleAdminRenameAgent(w http.ResponseWriter, r *http.Request) {
	id, err := strconv.Atoi(r.PathValue("id"))
	if err != nil {
		http.Error(w, "bad id", http.StatusBadRequest)
		return
	}
	if err := r.ParseForm(); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	name := strings.TrimSpace(r.FormValue("name"))
	if err := ws.store.RenameAgent(id, name); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	http.Redirect(w, r, "/admin/agents", http.StatusSeeOther)
}

func (ws *WebServer) handleAdminDeleteAgent(w http.ResponseWriter, r *http.Request) {
	id, err := strconv.Atoi(r.PathValue("id"))
	if err != nil {
		http.Error(w, "bad id", http.StatusBadRequest)
		return
	}
	if err := ws.store.DeleteAgent(id); err != nil {
		// Stale id (not found) is an idempotent recovery.
		if errors.Is(err, sql.ErrNoRows) || strings.Contains(err.Error(), "not found") {
			http.Redirect(w, r, "/admin/agents", http.StatusSeeOther)
			return
		}
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	http.Redirect(w, r, "/admin/agents", http.StatusSeeOther)
}

func (ws *WebServer) handleAdminDanger(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	if err := tmpl.ExecuteTemplate(w, "admin_danger", adminDangerData{Section: "danger"}); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
	}
}

func (ws *WebServer) handleAdminPurge(w http.ResponseWriter, r *http.Request) {
	if err := r.ParseForm(); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	if r.FormValue("confirm") != purgePhrase {
		http.Error(w, "confirmation phrase does not match", http.StatusBadRequest)
		return
	}
	if err := ws.store.Purge(); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	ws.broadcastDatasetReset()
	http.Redirect(w, r, "/", http.StatusSeeOther)
}

// broadcastDatasetReset is filled in by Task 16 (SSE). Stub now so this
// task can wire in the call site.
func (ws *WebServer) broadcastDatasetReset() {}
