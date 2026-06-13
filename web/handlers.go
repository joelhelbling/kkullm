package web

import (
	"encoding/json"
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
	Considering []cardView
	Todo        []cardView
	InFlight    []cardView
	Completed   []cardView
	Tabled      []cardView
	ShowProject bool
	Project     *model.Project // set in project-scoped view; nil for agent view
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
		}
		// blocked is an orthogonal flag now, so a blocked card stays in its
		// real status column above. The global Blocked column is sourced
		// separately by flag (see loadBlockers).
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
	Card          *model.Card
	Comments      []model.Comment
	StatusPills   []statusPill
	ProjectAgents []model.Agent
	CommentError  string
	EditError     string
	// BlockReason is the body of the most recent block/unblock kinded
	// comment, surfaced so the operator sees why a card is blocked. Empty
	// when no such note exists.
	BlockReason string
}

// latestBlockReason returns the body of the most recent block/unblock kinded
// comment in the list. comments are assumed ascending by created_at (as
// ListComments returns them), so the last matching entry wins.
func latestBlockReason(comments []model.Comment) string {
	reason := ""
	for _, c := range comments {
		if c.Kind == "block" || c.Kind == "unblock" {
			reason = c.Body
		}
	}
	return reason
}

// latestBlockComment returns the most recent kind="block" comment (the active
// block note), or nil if the card has no block note. comments are assumed
// ascending by created_at (as ListComments returns them), so the last block
// entry wins. Unblock notes don't override — the orchestrator view wants the
// reason a card is currently blocked, sourced from the active block.
func latestBlockComment(comments []model.Comment) *model.Comment {
	var found *model.Comment
	for i := range comments {
		if comments[i].Kind == "block" {
			found = &comments[i]
		}
	}
	return found
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
	ws.renderDrawerWith(w, card, commentError, "")
}

// renderDrawerWith renders the drawer with optional inline errors for the
// comment form (commentError) and the edit form (editError).
func (ws *WebServer) renderDrawerWith(w http.ResponseWriter, card *model.Card, commentError, editError string) {
	comments, err := ws.store.ListComments(card.ID)
	if err != nil {
		renderError(w, 500, "internal error", err)
		return
	}
	if comments == nil {
		comments = []model.Comment{}
	}
	// Agents scoped to the card's project populate the assignee picker.
	agents, err := ws.store.ListAgents(card.Project)
	if err != nil {
		renderError(w, 500, "internal error", err)
		return
	}
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	if err := tmpl.ExecuteTemplate(w, "drawer", drawerData{
		Card:          card,
		Comments:      comments,
		StatusPills:   buildStatusPills(card.Status),
		ProjectAgents: agents,
		CommentError:  commentError,
		EditError:     editError,
		BlockReason:   latestBlockReason(comments),
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

// wantsUnblock reports whether the request carries the "also unblock" signal.
// The unblock-on-edit confirm (status drag / re-assign on a blocked card)
// sends ?unblock=1 so the flag clears and an unblock note posts atomically
// with the edit.
func wantsUnblock(r *http.Request) bool {
	v := r.URL.Query().Get("unblock")
	return v == "1" || v == "true"
}

// postUnblockComment clears the blocked flag (already done by the caller's
// UpdateCard) by posting the kind="unblock" note authored by the operator, so
// the timeline records why/when the card was unblocked. The reason defaults to
// a generic note since the unblock-on-edit confirm doesn't collect one.
func (ws *WebServer) postUnblockComment(cardID int, reason string) {
	if reason == "" {
		reason = "Unblocked while editing the card."
	}
	agent, err := ws.store.GetAgentByName("user")
	if err != nil {
		log.Printf("unblock comment: user agent missing: %v", err)
		return
	}
	if _, err := ws.store.CreateComment(cardID, agent.ID, reason, "unblock"); err != nil {
		log.Printf("unblock comment: %v", err)
	}
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

	params := store.CardUpdateParams{Status: &newStatus}
	unblock := wantsUnblock(r)
	if unblock {
		blocked := false
		params.Blocked = &blocked
	}

	card, err := ws.store.UpdateCard(id, params)
	if err != nil {
		http.Error(w, err.Error(), 422)
		return
	}

	if unblock {
		ws.postUnblockComment(card.ID, "")
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

// allProjectsSelector is the sentinel ?project= value that selects the
// cross-project "All" board view.
const allProjectsSelector = "all"

func (ws *WebServer) handleBoard(w http.ResponseWriter, r *http.Request) {
	// The "All projects" aggregation is an operator/admin view; route it
	// through the RequireAdmin chokepoint so future auth lands in one place,
	// consistent with the other admin-gated routes.
	if r.URL.Query().Get("project") == allProjectsSelector {
		RequireAdmin(http.HandlerFunc(ws.handleBoardAll)).ServeHTTP(w, r)
		return
	}

	var cards []model.Card
	var showProject bool
	var project *model.Project
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
		p, projErr := ws.store.GetProject(id)
		if projErr != nil {
			renderError(w, 404, "project not found", projErr)
			return
		}
		project = p
		cards, err = ws.store.ListCards(store.CardListParams{Project: project.Name, ArchiveLimit: webArchiveLimit})
		showProject = false
	}

	if err != nil {
		renderError(w, 500, "internal error", err)
		return
	}

	bd := groupCards(cards, showProject)
	bd.Project = project // nil in agent view; suppresses the intro banner

	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	if err := tmpl.ExecuteTemplate(w, "board", bd); err != nil {
		log.Printf("render board: %v", err)
	}
}

// handleBoardAll renders the cross-project "All" board: every card from every
// project. Reached via ?project=all and gated by RequireAdmin (see handleBoard).
// Cards span projects here, so each tile shows its project label.
func (ws *WebServer) handleBoardAll(w http.ResponseWriter, r *http.Request) {
	// Empty Project filter lists cards across all projects.
	cards, err := ws.store.ListCards(store.CardListParams{ArchiveLimit: webArchiveLimit})
	if err != nil {
		renderError(w, 500, "internal error", err)
		return
	}

	bd := groupCards(cards, true) // showProject: tiles span projects
	bd.Project = nil              // no single-project intro banner in the All view

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
			renderError(w, 404, "agent not found", agentErr)
			return
		}
		params.Assignee = agent.Name
		scope = "agent: " + agent.Name
		boardURL = "/ui/board?agent=" + strconv.Itoa(id)
		showProject = true
	} else if r.URL.Query().Get("project") == allProjectsSelector {
		// Cross-project archived view (operator/admin); empty Project filter
		// aggregates across every project. Gated by RequireAdmin below.
		RequireAdmin(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			ws.renderArchived(w, store.CardListParams{
				ArchiveLimit: webArchiveLimit,
				ArchiveView:  "archived",
			}, "all projects", "/ui/board?project=all", true)
		})).ServeHTTP(w, r)
		return
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

	ws.renderArchived(w, params, scope, boardURL, showProject)
}

// renderArchived runs the archived-cards query for the given scope and writes
// the archived template. Shared by the per-project/agent path and the
// cross-project "All" path.
func (ws *WebServer) renderArchived(w http.ResponseWriter, params store.CardListParams, scope, boardURL string, showProject bool) {
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

// blockedCardView is a board tile for the orchestrator blocked view, carrying
// the active block reason/who/when sourced from the latest kind="block" comment
// so the operator can triage without opening each card.
type blockedCardView struct {
	cardView
	BlockReason string
	BlockedBy   string
	BlockedAt   time.Time
	HasBlock    bool
}

// blockedData mirrors boardData but holds blockedCardViews, so the blocked view
// can render each card in its real status column with the block reason inline.
type blockedData struct {
	Considering []blockedCardView
	Todo        []blockedCardView
	InFlight    []blockedCardView
	Completed   []blockedCardView
	Tabled      []blockedCardView
	ShowProject bool
}

// handleBlockedView is the orchestrator's global triage surface: every blocked
// card, across all projects, each still sitting in its real status column, with
// the active block reason/who/when surfaced inline. Clicking a card opens the
// existing drawer (with the #32 unblock/reassign controls). Gated by
// RequireAdmin (operator-only) — see RegisterRoutes.
func (ws *WebServer) handleBlockedView(w http.ResponseWriter, r *http.Request) {
	blocked := true
	cards, err := ws.store.ListCards(store.CardListParams{
		Blocked:      &blocked,
		ArchiveLimit: webArchiveLimit,
	})
	if err != nil {
		renderError(w, 500, "internal error", err)
		return
	}

	// Cards span projects here, so tiles show their project label.
	data := blockedData{ShowProject: true}
	for _, c := range cards {
		bcv := blockedCardView{cardView: cardView{Card: c, ShowProject: true}}
		// Surface the active block note (who/when/why) from the card's timeline.
		if comments, cErr := ws.store.ListComments(c.ID); cErr == nil {
			if bc := latestBlockComment(comments); bc != nil {
				bcv.BlockReason = bc.Body
				bcv.BlockedBy = bc.Agent
				bcv.BlockedAt = bc.CreatedAt
				bcv.HasBlock = true
			}
		} else {
			log.Printf("blocked view: list comments for card %d: %v", c.ID, cErr)
		}

		switch c.Status {
		case "considering":
			data.Considering = append(data.Considering, bcv)
		case "todo":
			data.Todo = append(data.Todo, bcv)
		case "in_flight":
			data.InFlight = append(data.InFlight, bcv)
		case "completed":
			data.Completed = append(data.Completed, bcv)
		case "tabled":
			data.Tabled = append(data.Tabled, bcv)
		}
	}

	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	if err := tmpl.ExecuteTemplate(w, "blocked_view", data); err != nil {
		log.Printf("render blocked view: %v", err)
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

	comment, err := ws.store.CreateComment(card.ID, agent.ID, body, "")
	if err != nil {
		renderError(w, 500, "internal error", err)
		return
	}

	ws.events.Publish(api.Event{Type: "comment_created", Data: comment})

	ws.renderDrawer(w, card, "")
}

// handleEditCard updates a card's title and body via the existing
// store.UpdateCard path and returns the re-rendered drawer fragment. The
// card_updated event is broadcast so other clients (and the board behind the
// open drawer) live-refresh, mirroring the status-change and comment flows.
func (ws *WebServer) handleEditCard(w http.ResponseWriter, r *http.Request) {
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

	title := strings.TrimSpace(r.FormValue("title"))
	body := strings.TrimSpace(r.FormValue("body"))
	if title == "" {
		ws.renderDrawerWith(w, card, "", "Title can't be empty.")
		return
	}

	updated, err := ws.store.UpdateCard(id, store.CardUpdateParams{
		Title: &title,
		Body:  &body,
	})
	if err != nil {
		ws.renderDrawerWith(w, card, "", err.Error())
		return
	}

	ws.events.Publish(api.Event{Type: "card_updated", Data: updated})

	ws.renderDrawer(w, updated, "")
}

// handleAssignCard replaces a card's assignees with the set submitted by the
// drawer's assignee picker. The picker sends a repeatable "assignee" field
// (one per chosen agent); an empty submission clears all assignees. It reuses
// store.UpdateCard's full-replace assignee path — the same path the API/CLI
// --assignee flag drives — and broadcasts card_updated so the board and other
// clients live-refresh, mirroring handleEditCard.
func (ws *WebServer) handleAssignCard(w http.ResponseWriter, r *http.Request) {
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

	// A non-nil slice signals UpdateCard to replace assignees; an empty
	// (but non-nil) slice clears them. r.PostForm["assignee"] is nil only
	// when the field is absent, so default to an empty slice.
	assignees := r.PostForm["assignee"]
	if assignees == nil {
		assignees = []string{}
	}

	params := store.CardUpdateParams{Assignees: assignees}
	unblock := wantsUnblock(r)
	if unblock {
		blocked := false
		params.Blocked = &blocked
	}

	updated, err := ws.store.UpdateCard(id, params)
	if err != nil {
		ws.renderDrawerWith(w, card, "", err.Error())
		return
	}

	if unblock {
		ws.postUnblockComment(updated.ID, "")
	}

	ws.events.Publish(api.Event{Type: "card_updated", Data: updated})

	ws.renderDrawer(w, updated, "")
}

// handleBlock sets the blocked flag and posts a kind="block" comment authored
// by the operator (the "user" agent, as handleAddComment resolves it). An
// optional "reason" form field becomes the comment body. The drawer is
// re-rendered and card_updated is broadcast so the board and other clients
// live-refresh the badge.
func (ws *WebServer) handleBlock(w http.ResponseWriter, r *http.Request) {
	ws.setBlocked(w, r, true)
}

// handleUnblock clears the blocked flag and posts a kind="unblock" comment,
// mirroring handleBlock.
func (ws *WebServer) handleUnblock(w http.ResponseWriter, r *http.Request) {
	ws.setBlocked(w, r, false)
}

func (ws *WebServer) setBlocked(w http.ResponseWriter, r *http.Request, blocked bool) {
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

	reason := strings.TrimSpace(r.FormValue("reason"))

	updated, err := ws.store.UpdateCard(id, store.CardUpdateParams{Blocked: &blocked})
	if err != nil {
		ws.renderDrawerWith(w, card, "", err.Error())
		return
	}

	// Author the kinded note as the operator, the same way handleAddComment
	// resolves the acting agent. A blank reason still posts a note so the
	// timeline records the state change.
	kind := "unblock"
	body := reason
	if blocked {
		kind = "block"
		if body == "" {
			body = "Blocked."
		}
	} else if body == "" {
		body = "Unblocked."
	}
	if agent, agentErr := ws.store.GetAgentByName("user"); agentErr == nil {
		if comment, cErr := ws.store.CreateComment(updated.ID, agent.ID, body, kind); cErr == nil {
			ws.events.Publish(api.Event{Type: "comment_created", Data: comment})
		} else {
			log.Printf("create %s comment: %v", kind, cErr)
		}
	} else {
		log.Printf("%s comment: user agent missing: %v", kind, agentErr)
	}

	ws.events.Publish(api.Event{Type: "card_updated", Data: updated})

	ws.renderDrawer(w, updated, "")
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
