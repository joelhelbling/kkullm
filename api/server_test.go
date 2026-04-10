package api

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strconv"
	"strings"
	"testing"

	"github.com/joelhelbling/kkullm/db"
	"github.com/joelhelbling/kkullm/model"
	"github.com/joelhelbling/kkullm/store"
)

func setupTestServer(t *testing.T) *httptest.Server {
	t.Helper()
	database, err := db.Open(":memory:")
	if err != nil {
		t.Fatalf("Open: %v", err)
	}
	if err := db.Migrate(database); err != nil {
		t.Fatalf("Migrate: %v", err)
	}
	if err := db.Seed(database); err != nil {
		t.Fatalf("Seed: %v", err)
	}
	t.Cleanup(func() { database.Close() })

	s := store.New(database)
	srv := NewServer(s)
	return httptest.NewServer(srv.Handler())
}

func TestListProjects(t *testing.T) {
	ts := setupTestServer(t)
	defer ts.Close()

	resp, err := http.Get(ts.URL + "/api/projects")
	if err != nil {
		t.Fatalf("GET /api/projects: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != 200 {
		t.Fatalf("expected 200, got %d", resp.StatusCode)
	}

	var projects []model.Project
	if err := json.NewDecoder(resp.Body).Decode(&projects); err != nil {
		t.Fatalf("decode: %v", err)
	}

	if len(projects) != 1 {
		t.Fatalf("expected 1 project, got %d", len(projects))
	}
	if projects[0].Name != "orchestration" {
		t.Errorf("expected project name 'orchestration', got %q", projects[0].Name)
	}
}

func TestCreateProject(t *testing.T) {
	ts := setupTestServer(t)
	defer ts.Close()

	body := `{"name":"test-project","description":"A test project"}`
	resp, err := http.Post(ts.URL+"/api/projects", "application/json", strings.NewReader(body))
	if err != nil {
		t.Fatalf("POST /api/projects: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != 201 {
		t.Fatalf("expected 201, got %d", resp.StatusCode)
	}

	var project model.Project
	if err := json.NewDecoder(resp.Body).Decode(&project); err != nil {
		t.Fatalf("decode: %v", err)
	}

	if project.Name != "test-project" {
		t.Errorf("expected name 'test-project', got %q", project.Name)
	}
}

func TestCreateAndListAgents(t *testing.T) {
	ts := setupTestServer(t)
	defer ts.Close()

	// Create a project
	resp, err := http.Post(ts.URL+"/api/projects", "application/json",
		strings.NewReader(`{"name":"agentproj","description":"for agents"}`))
	if err != nil {
		t.Fatalf("POST project: %v", err)
	}
	resp.Body.Close()

	// Create an agent in that project
	resp, err = http.Post(ts.URL+"/api/agents", "application/json",
		strings.NewReader(`{"name":"bot1","project":"agentproj","bio":"A test bot"}`))
	if err != nil {
		t.Fatalf("POST agent: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != 201 {
		t.Fatalf("expected 201, got %d", resp.StatusCode)
	}

	var agent model.Agent
	if err := json.NewDecoder(resp.Body).Decode(&agent); err != nil {
		t.Fatalf("decode agent: %v", err)
	}
	if agent.Name != "bot1" {
		t.Errorf("expected agent name 'bot1', got %q", agent.Name)
	}

	// List agents filtered by project
	resp2, err := http.Get(ts.URL + "/api/agents?project=agentproj")
	if err != nil {
		t.Fatalf("GET agents: %v", err)
	}
	defer resp2.Body.Close()

	var agents []model.Agent
	if err := json.NewDecoder(resp2.Body).Decode(&agents); err != nil {
		t.Fatalf("decode agents: %v", err)
	}
	if len(agents) != 1 {
		t.Fatalf("expected 1 agent, got %d", len(agents))
	}
	if agents[0].Name != "bot1" {
		t.Errorf("expected 'bot1', got %q", agents[0].Name)
	}
}

func TestCardCRUD(t *testing.T) {
	ts := setupTestServer(t)
	defer ts.Close()

	// Create a project and agent
	http.Post(ts.URL+"/api/projects", "application/json",
		strings.NewReader(`{"name":"cardproj"}`))
	http.Post(ts.URL+"/api/agents", "application/json",
		strings.NewReader(`{"name":"worker","project":"cardproj"}`))

	// Create a card with assignees and tags
	cardBody := `{
		"title": "Test card",
		"body": "Card body",
		"project": "cardproj",
		"assignees": ["worker"],
		"tags": ["urgent", "backend"]
	}`
	resp, err := http.Post(ts.URL+"/api/cards", "application/json", strings.NewReader(cardBody))
	if err != nil {
		t.Fatalf("POST card: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != 201 {
		t.Fatalf("expected 201, got %d", resp.StatusCode)
	}

	var card model.Card
	if err := json.NewDecoder(resp.Body).Decode(&card); err != nil {
		t.Fatalf("decode card: %v", err)
	}
	if card.Title != "Test card" {
		t.Errorf("expected title 'Test card', got %q", card.Title)
	}
	if card.Status != "considering" {
		t.Errorf("expected status 'considering', got %q", card.Status)
	}

	// Update status: considering -> todo
	cardID := strconv.Itoa(card.ID)
	req, _ := http.NewRequest("PATCH", ts.URL+"/api/cards/"+cardID, strings.NewReader(`{"status":"todo"}`))
	req.Header.Set("Content-Type", "application/json")
	resp2, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("PATCH card: %v", err)
	}
	defer resp2.Body.Close()

	if resp2.StatusCode != 200 {
		t.Fatalf("expected 200 for update, got %d", resp2.StatusCode)
	}

	var updated model.Card
	json.NewDecoder(resp2.Body).Decode(&updated)
	if updated.Status != "todo" {
		t.Errorf("expected status 'todo', got %q", updated.Status)
	}

	// Update status: todo -> in_flight
	req, _ = http.NewRequest("PATCH", ts.URL+"/api/cards/"+cardID, strings.NewReader(`{"status":"in_flight"}`))
	req.Header.Set("Content-Type", "application/json")
	resp3, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("PATCH card: %v", err)
	}
	defer resp3.Body.Close()

	json.NewDecoder(resp3.Body).Decode(&updated)
	if updated.Status != "in_flight" {
		t.Errorf("expected status 'in_flight', got %q", updated.Status)
	}

	// List cards by status
	resp4, err := http.Get(ts.URL + "/api/cards?status=in_flight")
	if err != nil {
		t.Fatalf("GET cards: %v", err)
	}
	defer resp4.Body.Close()

	var cards []model.Card
	json.NewDecoder(resp4.Body).Decode(&cards)
	if len(cards) != 1 {
		t.Fatalf("expected 1 card with status in_flight, got %d", len(cards))
	}
}

func TestCardComments(t *testing.T) {
	ts := setupTestServer(t)
	defer ts.Close()

	// Use the seeded orchestration project and user agent
	cardBody := `{"title":"Comment test card","project":"orchestration"}`
	resp, err := http.Post(ts.URL+"/api/cards", "application/json", strings.NewReader(cardBody))
	if err != nil {
		t.Fatalf("POST card: %v", err)
	}
	defer resp.Body.Close()

	var card model.Card
	json.NewDecoder(resp.Body).Decode(&card)
	cardID := strconv.Itoa(card.ID)

	// Add a comment
	commentBody := `{"agent":"user","body":"This is a comment"}`
	resp2, err := http.Post(ts.URL+"/api/cards/"+cardID+"/comments", "application/json",
		strings.NewReader(commentBody))
	if err != nil {
		t.Fatalf("POST comment: %v", err)
	}
	defer resp2.Body.Close()

	if resp2.StatusCode != 201 {
		t.Fatalf("expected 201 for comment, got %d", resp2.StatusCode)
	}

	var comment model.Comment
	json.NewDecoder(resp2.Body).Decode(&comment)
	if comment.Body != "This is a comment" {
		t.Errorf("expected comment body 'This is a comment', got %q", comment.Body)
	}

	// List comments
	resp3, err := http.Get(ts.URL + "/api/cards/" + cardID + "/comments")
	if err != nil {
		t.Fatalf("GET comments: %v", err)
	}
	defer resp3.Body.Close()

	var comments []model.Comment
	json.NewDecoder(resp3.Body).Decode(&comments)
	if len(comments) != 1 {
		t.Fatalf("expected 1 comment, got %d", len(comments))
	}
}

func TestServerEventBus(t *testing.T) {
	database, err := db.Open(":memory:")
	if err != nil {
		t.Fatalf("Open: %v", err)
	}
	defer database.Close()
	if err := db.Migrate(database); err != nil {
		t.Fatalf("Migrate: %v", err)
	}

	s := NewServer(store.New(database))
	eb := s.EventBus()
	if eb == nil {
		t.Fatal("expected non-nil EventBus")
	}

	// Verify it's functional
	ch := eb.Subscribe()
	defer eb.Unsubscribe(ch)

	eb.Publish(Event{Type: "test", Data: "hello"})

	select {
	case e := <-ch:
		if e.Type != "test" {
			t.Errorf("expected event type 'test', got %q", e.Type)
		}
	default:
		t.Fatal("expected to receive event")
	}
}

func TestSSEStream(t *testing.T) {
	ts := setupTestServer(t)
	defer ts.Close()

	resp, err := http.Get(ts.URL + "/api/events")
	if err != nil {
		t.Fatalf("GET /api/events: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != 200 {
		t.Fatalf("expected 200, got %d", resp.StatusCode)
	}

	ct := resp.Header.Get("Content-Type")
	if ct != "text/event-stream" {
		t.Errorf("expected Content-Type 'text/event-stream', got %q", ct)
	}
}
