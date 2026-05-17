package web

import (
	"database/sql"
	"errors"
	"fmt"
	"net/http"
	"strconv"
	"strings"

	"github.com/joelhelbling/kkullm/api"
	"github.com/joelhelbling/kkullm/model"
)

type adminProjectRow struct {
	ID          int
	Name        string
	Description string
	CardCount   int
	AgentCount  int
}

// adminProjectForm carries the values a user submitted, so an error
// re-render can repopulate the reopened modal.
type adminProjectForm struct {
	ID          int
	Name        string
	Description string
}

type adminProjectsData struct {
	Section  string
	Projects []adminProjectRow
	Error    string
	Form     adminProjectForm
	Reopen   string // "", "create", or "edit"
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
	data, err := ws.projectsPageData()
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	ws.renderProjectsPage(w, http.StatusOK, data)
}

// projectsPageData builds the full Projects admin page model from the store.
func (ws *WebServer) projectsPageData() (adminProjectsData, error) {
	projects, err := ws.store.ListProjects()
	if err != nil {
		return adminProjectsData{}, err
	}
	rows := make([]adminProjectRow, 0, len(projects))
	for _, p := range projects {
		cc, err := ws.store.CountCardsForProject(p.ID)
		if err != nil {
			return adminProjectsData{}, err
		}
		ac, err := ws.store.CountAgentsForProject(p.ID)
		if err != nil {
			return adminProjectsData{}, err
		}
		rows = append(rows, adminProjectRow{
			ID: p.ID, Name: p.Name, Description: p.Description,
			CardCount: cc, AgentCount: ac,
		})
	}
	return adminProjectsData{Section: "projects", Projects: rows}, nil
}

func (ws *WebServer) renderProjectsPage(w http.ResponseWriter, status int, data adminProjectsData) {
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.WriteHeader(status)
	if err := tmpl.ExecuteTemplate(w, "admin_projects", data); err != nil {
		// Headers are already sent; nothing more we can do but log via the error.
		http.Error(w, err.Error(), http.StatusInternalServerError)
	}
}

// renderProjectError re-renders the Projects page with an error banner and the
// submitted values, reopening the modal named by reopen ("create" or "edit").
func (ws *WebServer) renderProjectError(w http.ResponseWriter, msg string, form adminProjectForm, reopen string) {
	data, err := ws.projectsPageData()
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	data.Error = msg
	data.Form = form
	data.Reopen = reopen
	ws.renderProjectsPage(w, http.StatusBadRequest, data)
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

func (ws *WebServer) handleAdminCreateProject(w http.ResponseWriter, r *http.Request) {
	if err := r.ParseForm(); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	name := strings.TrimSpace(r.FormValue("name"))
	description := strings.TrimSpace(r.FormValue("description"))
	form := adminProjectForm{Name: name, Description: description}

	if name == "" {
		ws.renderProjectError(w, "Project name is required.", form, "create")
		return
	}
	if _, err := ws.store.CreateProject(name, description); err != nil {
		if isUniqueViolation(err) {
			ws.renderProjectError(w, fmt.Sprintf("A project named %q already exists.", name), form, "create")
			return
		}
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	http.Redirect(w, r, "/admin/projects", http.StatusSeeOther)
}

func (ws *WebServer) handleAdminUpdateProject(w http.ResponseWriter, r *http.Request) {
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
	description := strings.TrimSpace(r.FormValue("description"))
	form := adminProjectForm{ID: id, Name: name, Description: description}

	if name == "" {
		ws.renderProjectError(w, "Project name is required.", form, "edit")
		return
	}
	if err := ws.store.UpdateProject(id, name, description); err != nil {
		if isUniqueViolation(err) {
			ws.renderProjectError(w, fmt.Sprintf("A project named %q already exists.", name), form, "edit")
			return
		}
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	ws.broadcastProjectRenamed(id, name)
	http.Redirect(w, r, "/admin/projects", http.StatusSeeOther)
}

// isUniqueViolation reports whether err is a SQLite UNIQUE constraint failure.
// The modernc.org/sqlite driver reports these with the text
// "UNIQUE constraint failed" in the error message.
func isUniqueViolation(err error) bool {
	return err != nil && strings.Contains(err.Error(), "UNIQUE constraint failed")
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
	ws.broadcastProjectDeleted(id)
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
	ws.broadcastAgentRenamed(id, name)
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
	ws.broadcastAgentDeleted(id)
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

func (ws *WebServer) broadcastDatasetReset() {
	ws.events.Publish(api.Event{Type: "dataset_reset"})
}

func (ws *WebServer) broadcastProjectRenamed(id int, name string) {
	ws.events.Publish(api.Event{Type: "project_renamed", Data: map[string]any{"id": id, "name": name}})
}

func (ws *WebServer) broadcastProjectDeleted(id int) {
	ws.events.Publish(api.Event{Type: "project_deleted", Data: map[string]int{"id": id}})
}

func (ws *WebServer) broadcastAgentRenamed(id int, name string) {
	ws.events.Publish(api.Event{Type: "agent_renamed", Data: map[string]any{"id": id, "name": name}})
}

func (ws *WebServer) broadcastAgentDeleted(id int) {
	ws.events.Publish(api.Event{Type: "agent_deleted", Data: map[string]int{"id": id}})
}
