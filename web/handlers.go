package web

import (
	"encoding/json"
	"fmt"
	"html/template"
	"log"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/joelhelbling/kkullm/api"
	"github.com/joelhelbling/kkullm/model"
	"github.com/joelhelbling/kkullm/store"
)

var tmpl *template.Template

var funcMap = template.FuncMap{
	"projectColor": projectColor,
	"tagBg":        tagBg,
	"tagColor":     tagColor,
	"joinStrings":  joinStrings,
	"timeAgo":      timeAgo,
}

var projectColors = []string{
	"#0969da", "#1a7f37", "#9a6700", "#cf222e", "#8250df",
	"#bf3989", "#0550ae", "#116329", "#7d4e00", "#a40e26",
}

func projectColor(name string) string {
	h := 0
	for _, c := range name {
		h = h*31 + int(c)
	}
	if h < 0 {
		h = -h
	}
	return projectColors[h%len(projectColors)]
}

var tagColorMap = map[string][2]string{
	"bug":         {"#ffebe9", "#cf222e"},
	"feature":     {"#dafbe1", "#1a7f37"},
	"enhancement": {"#ddf4ff", "#0969da"},
	"docs":        {"#dafbe1", "#1a7f37"},
	"rfc":         {"#fff8c5", "#9a6700"},
	"infra":       {"#dafbe1", "#1a7f37"},
	"urgent":      {"#ffebe9", "#cf222e"},
}

var defaultTagColors = [2]string{"#ddf4ff", "#0969da"}

func tagBg(tag string) string {
	if colors, ok := tagColorMap[tag]; ok {
		return colors[0]
	}
	return defaultTagColors[0]
}

func tagColor(tag string) string {
	if colors, ok := tagColorMap[tag]; ok {
		return colors[1]
	}
	return defaultTagColors[1]
}

func joinStrings(strs []string, sep string) string {
	return strings.Join(strs, sep)
}

func timeAgo(t time.Time) string {
	d := time.Since(t)
	switch {
	case d < time.Minute:
		return "just now"
	case d < time.Hour:
		m := int(d.Minutes())
		if m == 1 {
			return "1m ago"
		}
		return fmt.Sprintf("%dm ago", m)
	case d < 24*time.Hour:
		h := int(d.Hours())
		if h == 1 {
			return "1h ago"
		}
		return fmt.Sprintf("%dh ago", h)
	default:
		days := int(d.Hours() / 24)
		if days == 1 {
			return "1d ago"
		}
		return fmt.Sprintf("%dd ago", days)
	}
}

func initTemplates() {
	var err error
	tmpl, err = template.New("").Funcs(funcMap).ParseFS(content, "templates/*.html")
	if err != nil {
		log.Fatalf("parse templates: %v", err)
	}
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
		http.Error(w, err.Error(), 500)
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
		http.Error(w, err.Error(), 404)
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

	// Decide response format based on the htmx target header.
	// When the click came from the drawer's status selector, htmx sets
	// HX-Target to "drawer-container" (the id of the target element).
	// When the click came from drag-and-drop on the board, we don't
	// target the drawer — return just the updated card tile.
	hxTarget := r.Header.Get("HX-Target")
	if hxTarget == "drawer-container" {
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
			http.Error(w, agentErr.Error(), 404)
			return
		}
		cards, err = ws.store.ListCards(store.CardListParams{Assignee: agent.Name, ArchiveLimit: webArchiveLimit})
		showProject = true
	} else {
		projectID := r.URL.Query().Get("project")
		if projectID == "" {
			projectID = "1"
		}
		id, parseErr := strconv.Atoi(projectID)
		if parseErr != nil {
			http.Error(w, "invalid project id", 400)
			return
		}
		project, projErr := ws.store.GetProject(id)
		if projErr != nil {
			http.Error(w, projErr.Error(), 404)
			return
		}
		cards, err = ws.store.ListCards(store.CardListParams{Project: project.Name, ArchiveLimit: webArchiveLimit})
		showProject = false
	}

	if err != nil {
		http.Error(w, err.Error(), 500)
		return
	}

	bd := groupCards(cards, showProject)
	bd.BlockedCards = nil // blocked column is always global, not per-scope

	blockedCards, blockedErr := ws.store.ListCards(store.CardListParams{
		Status: "blocked",
	})
	if blockedErr != nil {
		log.Printf("list blocked cards: %v", blockedErr)
	}
	for _, c := range blockedCards {
		bd.BlockedCards = append(bd.BlockedCards, cardView{Card: c, ShowProject: true})
	}

	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	if err := tmpl.ExecuteTemplate(w, "board", bd); err != nil {
		log.Printf("render board: %v", err)
	}
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
			http.Error(w, agentErr.Error(), 404)
			return
		}
		params.Assignee = agent.Name
		scope = "agent: " + agent.Name
		boardURL = "/ui/board?agent=" + strconv.Itoa(id)
		showProject = true
	} else {
		projectID := r.URL.Query().Get("project")
		if projectID == "" {
			projectID = "1"
		}
		id, parseErr := strconv.Atoi(projectID)
		if parseErr != nil {
			http.Error(w, "invalid project id", 400)
			return
		}
		project, projErr := ws.store.GetProject(id)
		if projErr != nil {
			http.Error(w, projErr.Error(), 404)
			return
		}
		params.Project = project.Name
		scope = "project: " + project.Name
		boardURL = "/ui/board?project=" + strconv.Itoa(id)
	}

	cards, err := ws.store.ListCards(params)
	if err != nil {
		http.Error(w, err.Error(), 500)
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
		http.Error(w, err.Error(), 404)
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
		http.Error(w, err.Error(), 500)
		return
	}

	ws.events.Publish(api.Event{Type: "comment_created", Data: comment})

	ws.renderDrawer(w, card, "")
}

func (ws *WebServer) handleBlockers(w http.ResponseWriter, r *http.Request) {
	cards, err := ws.store.ListCards(store.CardListParams{
		Status: "blocked",
	})
	if err != nil {
		http.Error(w, err.Error(), 500)
		return
	}

	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	if err := tmpl.ExecuteTemplate(w, "blockers", cards); err != nil {
		log.Printf("render blockers: %v", err)
	}
}
