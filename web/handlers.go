package web

import (
	"encoding/json"
	"html/template"
	"log"
	"net/http"
	"strconv"
	"strings"

	"github.com/joelhelbling/kkullm/api"
	"github.com/joelhelbling/kkullm/model"
	"github.com/joelhelbling/kkullm/store"
)

// renderError writes a user-facing error response and logs the underlying
// detail. The publicMsg is what the browser sees; err is for the server log.
func renderError(w http.ResponseWriter, code int, publicMsg string, err error) {
	if err != nil {
		log.Printf("%s: %v", publicMsg, err)
	}
	http.Error(w, publicMsg, code)
}

// defaultProjectID returns the id of the first project in storage, used when
// a request omits ?project=. Returns 0 (and no error) if no projects exist,
// letting the caller decide how to respond.
func (ws *WebServer) defaultProjectID() (int, error) {
	projects, err := ws.store.ListProjects()
	if err != nil {
		return 0, err
	}
	if len(projects) == 0 {
		return 0, nil
	}
	return projects[0].ID, nil
}

// resolveProjectID parses the ?project= query value, falling back to the
// first project when empty. On any failure it writes the HTTP error and
// returns ok=false so the caller can return without further response writes.
func (ws *WebServer) resolveProjectID(w http.ResponseWriter, raw string) (int, bool) {
	if raw == "" {
		id, err := ws.defaultProjectID()
		if err != nil {
			renderError(w, 500, "internal error", err)
			return 0, false
		}
		if id == 0 {
			renderError(w, 404, "no projects available", nil)
			return 0, false
		}
		return id, true
	}
	id, err := strconv.Atoi(raw)
	if err != nil {
		http.Error(w, "invalid project id", 400)
		return 0, false
	}
	return id, true
}

// webArchiveLimit caps the number of completed/tabled cards rendered on
// the main board. Older terminal cards spill into the /archived view.
const webArchiveLimit = 20

type layoutData struct {
	Projects         []model.Project
	Agents           []model.Agent
	DefaultProjectID int
	BootData         template.JS
}

type cardView struct {
	model.Card
	ShowProject bool
}

type boardData struct {
	Considering  []cardView
	Todo         []cardView
	InFlight     []cardView
	Completed    []cardView
	Tabled       []cardView
	BlockedCards []cardView
	ShowProject  bool
}

func groupCards(cards []model.Card, showProject bool) boardData {
	bd := boardData{ShowProject: showProject}
	for _, c := range cards {
		cv := cardView{Card: c, ShowProject: showProject}
		switch c.Status {
		case "considering":
			bd.Considering = append(bd.Considering, cv)
		case "todo":
			bd.Todo = append(bd.Todo, cv)
		case "in_flight":
			bd.InFlight = append(bd.InFlight, cv)
		case "completed":
			bd.Completed = append(bd.Completed, cv)
		case "tabled":
			bd.Tabled = append(bd.Tabled, cv)
		case "blocked":
			bd.BlockedCards = append(bd.BlockedCards, cv)
		}
	}
	return bd
}

func (ws *WebServer) handleRoot(w http.ResponseWriter, r *http.Request) {
	if r.URL.Path != "/" {
		http.NotFound(w, r)
		return
	}

	projects, err := ws.store.ListProjects()
	if err != nil {
		renderError(w, 500, "internal error", err)
		return
	}

	agents, err := ws.store.ListAgents("")
	if err != nil {
		renderError(w, 500, "internal error", err)
		return
	}

	defaultProjectID := 0
	if len(projects) > 0 {
		defaultProjectID = projects[0].ID
	}

	type bootProject struct {
		ID   int    `json:"id"`
		Name string `json:"name"`
	}
	type bootAgent struct {
		ID      int    `json:"id"`
		Name    string `json:"name"`
		Project string `json:"project"`
	}
	type bootPayload struct {
		Projects         []bootProject `json:"projects"`
		Agents           []bootAgent   `json:"agents"`
		DefaultProjectID int           `json:"defaultProjectId"`
	}
	payload := bootPayload{DefaultProjectID: defaultProjectID}
	for _, p := range projects {
		payload.Projects = append(payload.Projects, bootProject{ID: p.ID, Name: p.Name})
	}
	for _, a := range agents {
		payload.Agents = append(payload.Agents, bootAgent{ID: a.ID, Name: a.Name, Project: a.Project})
	}
	bootJSON, _ := json.Marshal(payload)

	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	if err := tmpl.ExecuteTemplate(w, "layout.html", layoutData{
		Projects:         projects,
		Agents:           agents,
		DefaultProjectID: defaultProjectID,
		BootData:         template.JS(bootJSON),
	}); err != nil {
		log.Printf("render layout: %v", err)
	}
}

type statusPill struct {
	Status    string
	Reachable bool
}

type drawerData struct {
	Card         *model.Card
	Comments     []model.Comment
	StatusPills  []statusPill
	CommentError string
}

func buildStatusPills(current string) []statusPill {
	pills := make([]statusPill, 0, len(model.AllStatuses)-1)
	for _, s := range model.AllStatuses {
		if s == current {
			continue
		}
		pills = append(pills, statusPill{
			Status:    s,
			Reachable: model.CanTransition(current, s),
		})
	}
	return pills
}

// renderDrawer loads comments for the card and writes the rendered drawer
// template. commentError, when non-empty, is shown above the comment form.
func (ws *WebServer) renderDrawer(w http.ResponseWriter, card *model.Card, commentError string) {
	comments, err := ws.store.ListComments(card.ID)
	if err != nil {
		renderError(w, 500, "internal error", err)
		return
	}
	if comments == nil {
		comments = []model.Comment{}
	}
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	if err := tmpl.ExecuteTemplate(w, "drawer", drawerData{
		Card:         card,
		Comments:     comments,
		StatusPills:  buildStatusPills(card.Status),
		CommentError: commentError,
	}); err != nil {
		log.Printf("render drawer: %v", err)
	}
}

func (ws *WebServer) handleDrawer(w http.ResponseWriter, r *http.Request) {
	id, err := strconv.Atoi(r.PathValue("id"))
	if err != nil {
		http.Error(w, "invalid id", 400)
		return
	}

	card, err := ws.store.GetCard(id)
	if err != nil {
		renderError(w, 404, "card not found", err)
		return
	}

	ws.renderDrawer(w, card, "")
}

func (ws *WebServer) handleStatusChange(w http.ResponseWriter, r *http.Request) {
	id, err := strconv.Atoi(r.PathValue("id"))
	if err != nil {
		http.Error(w, "invalid id", 400)
		return
	}

	if err := r.ParseForm(); err != nil {
		http.Error(w, "invalid form data", 400)
		return
	}

	newStatus := r.FormValue("status")
	if newStatus == "" {
		http.Error(w, "status is required", 400)
		return
	}

	card, err := ws.store.UpdateCard(id, store.CardUpdateParams{
		Status: &newStatus,
	})
	if err != nil {
		http.Error(w, err.Error(), 422)
		return
	}

	ws.events.Publish(api.Event{Type: "card_updated", Data: card})

	// Callers tell us which fragment they want via an explicit ?response=
	// hint, decoupled from any DOM id. The drawer's status pills pass
	// ?response=drawer; drag-and-drop on the board passes nothing and
	// gets back just the updated card tile.
	if r.URL.Query().Get("response") == "drawer" {
		ws.renderDrawer(w, card, "")
		return
	}

	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	if err := tmpl.ExecuteTemplate(w, "card", cardView{Card: *card, ShowProject: false}); err != nil {
		log.Printf("render card: %v", err)
	}
}

func (ws *WebServer) handleBoard(w http.ResponseWriter, r *http.Request) {
	var cards []model.Card
	var showProject bool
	var err error

	if agentID := r.URL.Query().Get("agent"); agentID != "" {
		id, parseErr := strconv.Atoi(agentID)
		if parseErr != nil {
			http.Error(w, "invalid agent id", 400)
			return
		}
		agent, agentErr := ws.store.GetAgent(id)
		if agentErr != nil {
			renderError(w, 404, "agent not found", agentErr)
			return
		}
		cards, err = ws.store.ListCards(store.CardListParams{Assignee: agent.Name, ArchiveLimit: webArchiveLimit})
		showProject = true
	} else {
		id, ok := ws.resolveProjectID(w, r.URL.Query().Get("project"))
		if !ok {
			return
		}
		project, projErr := ws.store.GetProject(id)
		if projErr != nil {
			renderError(w, 404, "project not found", projErr)
			return
		}
		cards, err = ws.store.ListCards(store.CardListParams{Project: project.Name, ArchiveLimit: webArchiveLimit})
		showProject = false
	}

	if err != nil {
		renderError(w, 500, "internal error", err)
		return
	}

	bd := groupCards(cards, showProject)
	bd.BlockedCards = ws.loadBlockers(w) // global; nil on error with toast trigger set

	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	if err := tmpl.ExecuteTemplate(w, "board", bd); err != nil {
		log.Printf("render board: %v", err)
	}
}

// blockerToastTrigger is the HX-Trigger payload that asks the front-end to
// show a non-blocking error toast when the blocker query fails. The board
// still renders so the user keeps the rest of their work in view.
const blockerToastTrigger = `{"showToast":{"message":"Couldn't load blockers","variant":"error"}}`

// loadBlockers returns the global Blocked column. On failure it logs the
// detail, sets an HX-Trigger header for a toast, and returns nil so the
// board renders with an empty Blocked column.
func (ws *WebServer) loadBlockers(w http.ResponseWriter) []cardView {
	cards, err := ws.store.ListCards(store.CardListParams{Status: "blocked"})
	if err != nil {
		log.Printf("list blocked cards: %v", err)
		w.Header().Set("HX-Trigger", blockerToastTrigger)
		return nil
	}
	out := make([]cardView, 0, len(cards))
	for _, c := range cards {
		out = append(out, cardView{Card: c, ShowProject: true})
	}
	return out
}

type archivedData struct {
	Completed   []cardView
	Tabled      []cardView
	ShowProject bool
	Scope       string // human-readable label for the current scope
	BoardURL    string // URL to return to the live board for the same scope
}

func (ws *WebServer) handleArchived(w http.ResponseWriter, r *http.Request) {
	params := store.CardListParams{
		ArchiveLimit: webArchiveLimit,
		ArchiveView:  "archived",
	}
	var scope, boardURL string
	showProject := false

	if agentID := r.URL.Query().Get("agent"); agentID != "" {
		id, parseErr := strconv.Atoi(agentID)
		if parseErr != nil {
			http.Error(w, "invalid agent id", 400)
			return
		}
		agent, agentErr := ws.store.GetAgent(id)
		if agentErr != nil {
			renderError(w, 404, "agent not found", agentErr)
			return
		}
		params.Assignee = agent.Name
		scope = "agent: " + agent.Name
		boardURL = "/ui/board?agent=" + strconv.Itoa(id)
		showProject = true
	} else {
		id, ok := ws.resolveProjectID(w, r.URL.Query().Get("project"))
		if !ok {
			return
		}
		project, projErr := ws.store.GetProject(id)
		if projErr != nil {
			renderError(w, 404, "project not found", projErr)
			return
		}
		params.Project = project.Name
		scope = "project: " + project.Name
		boardURL = "/ui/board?project=" + strconv.Itoa(id)
	}

	cards, err := ws.store.ListCards(params)
	if err != nil {
		renderError(w, 500, "internal error", err)
		return
	}

	data := archivedData{ShowProject: showProject, Scope: scope, BoardURL: boardURL}
	for _, c := range cards {
		cv := cardView{Card: c, ShowProject: showProject}
		switch c.Status {
		case "completed":
			data.Completed = append(data.Completed, cv)
		case "tabled":
			data.Tabled = append(data.Tabled, cv)
		}
	}

	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	if err := tmpl.ExecuteTemplate(w, "archived", data); err != nil {
		log.Printf("render archived: %v", err)
	}
}

func (ws *WebServer) handleAddComment(w http.ResponseWriter, r *http.Request) {
	id, err := strconv.Atoi(r.PathValue("id"))
	if err != nil {
		http.Error(w, "invalid id", 400)
		return
	}

	card, err := ws.store.GetCard(id)
	if err != nil {
		renderError(w, 404, "card not found", err)
		return
	}

	if err := r.ParseForm(); err != nil {
		http.Error(w, "invalid form data", 400)
		return
	}

	body := strings.TrimSpace(r.FormValue("body"))
	if body == "" {
		ws.renderDrawer(w, card, "Comment can't be empty.")
		return
	}

	agent, err := ws.store.GetAgentByName("user")
	if err != nil {
		http.Error(w, "user agent missing — check seed data", 500)
		return
	}

	comment, err := ws.store.CreateComment(card.ID, agent.ID, body)
	if err != nil {
		renderError(w, 500, "internal error", err)
		return
	}

	ws.events.Publish(api.Event{Type: "comment_created", Data: comment})

	ws.renderDrawer(w, card, "")
}

func (ws *WebServer) handleDeleteCard(w http.ResponseWriter, r *http.Request) {
	id, err := strconv.Atoi(r.PathValue("id"))
	if err != nil {
		http.Error(w, "bad id", 400)
		return
	}
	if err := ws.store.DeleteCard(id); err != nil {
		if strings.Contains(err.Error(), "not found") {
			http.Redirect(w, r, "/", http.StatusSeeOther)
			return
		}
		renderError(w, 500, "internal error", err)
		return
	}
	ws.broadcastCardDeleted(id)
	http.Redirect(w, r, "/", http.StatusSeeOther)
}

func (ws *WebServer) broadcastCardDeleted(id int) {
	ws.events.Publish(api.Event{Type: "card_deleted", Data: map[string]int{"id": id}})
}

func (ws *WebServer) handleBlockers(w http.ResponseWriter, r *http.Request) {
	cards, err := ws.store.ListCards(store.CardListParams{
		Status: "blocked",
	})
	if err != nil {
		renderError(w, 500, "internal error", err)
		return
	}

	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	if err := tmpl.ExecuteTemplate(w, "blockers", cards); err != nil {
		log.Printf("render blockers: %v", err)
	}
}
