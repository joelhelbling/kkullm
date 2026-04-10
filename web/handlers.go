package web

import (
	"fmt"
	"html/template"
	"log"
	"net/http"
	"strconv"
	"strings"
	"time"

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

type layoutData struct {
	Projects         []model.Project
	Agents           []model.Agent
	DefaultProjectID int
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
	Done         []cardView
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
		case "done":
			bd.Done = append(bd.Done, cv)
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

	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	if err := tmpl.ExecuteTemplate(w, "layout.html", layoutData{
		Projects:         projects,
		Agents:           agents,
		DefaultProjectID: defaultProjectID,
	}); err != nil {
		log.Printf("render layout: %v", err)
	}
}

type drawerData struct {
	Card        *model.Card
	Comments    []model.Comment
	Transitions []string
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

	comments, err := ws.store.ListComments(id)
	if err != nil {
		http.Error(w, err.Error(), 500)
		return
	}
	if comments == nil {
		comments = []model.Comment{}
	}

	transitions := model.AllowedTransitions(card.Status)

	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	if err := tmpl.ExecuteTemplate(w, "drawer", drawerData{
		Card:        card,
		Comments:    comments,
		Transitions: transitions,
	}); err != nil {
		log.Printf("render drawer: %v", err)
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
		cards, err = ws.store.ListCards(store.CardListParams{Assignee: agent.Name})
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
		cards, err = ws.store.ListCards(store.CardListParams{Project: project.Name})
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
