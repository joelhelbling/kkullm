package test

import (
	"net/http/httptest"
	"testing"

	"github.com/joelhelbling/kkullm/api"
	"github.com/joelhelbling/kkullm/client"
	"github.com/joelhelbling/kkullm/db"
	"github.com/joelhelbling/kkullm/model"
	"github.com/joelhelbling/kkullm/store"
)

func TestFullWorkflow(t *testing.T) {
	// Setup: in-memory DB, migrate, seed, create httptest server with API
	database, _ := db.Open(":memory:")
	defer database.Close()
	db.Migrate(database)
	db.Seed(database)

	s := store.New(database)
	srv := api.NewServer(s)
	ts := httptest.NewServer(srv.Handler())
	defer ts.Close()

	c := client.New(ts.URL)

	// 1. Create project
	proj, err := c.CreateProject("acme-backend", "The ACME backend")
	if err != nil {
		t.Fatalf("create project: %v", err)
	}
	if proj.Name != "acme-backend" {
		t.Fatalf("project name = %q", proj.Name)
	}

	// 2. Create agent
	agent, err := c.CreateAgent("dev-agent", "acme-backend", "Writes Go code")
	if err != nil {
		t.Fatalf("create agent: %v", err)
	}

	// 3. Create asset
	_, err = c.CreateAsset("acme-backend", "GitHub repo", "Main repo", "https://github.com/acme/backend")
	if err != nil {
		t.Fatalf("create asset: %v", err)
	}

	// 4. Create card with assignee and tags
	card, err := c.CreateCard(client.CardCreateRequest{
		Title:     "Implement JWT auth",
		Body:      "Add JWT middleware to all API routes",
		Status:    "todo",
		Project:   "acme-backend",
		Assignees: []string{agent.Name},
		Tags:      []string{"auth", "security"},
	})
	if err != nil {
		t.Fatalf("create card: %v", err)
	}

	// 5. Agent claims card (todo -> in_flight)
	status := "in_flight"
	claimed, err := c.UpdateCard(card.ID, client.CardUpdateRequest{Status: &status})
	if err != nil {
		t.Fatalf("claim card: %v", err)
	}
	if claimed.Status != "in_flight" {
		t.Fatalf("status = %q, want 'in_flight'", claimed.Status)
	}

	// 6. Agent adds comment
	_, err = c.CreateComment(card.ID, "dev-agent", "Started implementing JWT middleware")
	if err != nil {
		t.Fatalf("add comment: %v", err)
	}

	// 7. Agent creates sub-task with relation
	subtask, err := c.CreateCard(client.CardCreateRequest{
		Title:     "Write JWT tests",
		Status:    "todo",
		Project:   "acme-backend",
		Assignees: []string{"dev-agent"},
		Relations: []model.CardRelation{{RelatedCardID: card.ID, RelationType: "belongs_to"}},
	})
	if err != nil {
		t.Fatalf("create subtask: %v", err)
	}
	if len(subtask.Relations) != 1 {
		t.Fatalf("subtask relations = %d, want 1", len(subtask.Relations))
	}

	// 8. Agent completes card (in_flight -> completed)
	completed := "completed"
	_, err = c.UpdateCard(card.ID, client.CardUpdateRequest{Status: &completed})
	if err != nil {
		t.Fatalf("complete card: %v", err)
	}

	// 9. List cards by status + assignee
	cards, err := c.ListCards("acme-backend", "dev-agent", "todo", "")
	if err != nil {
		t.Fatalf("list cards: %v", err)
	}
	if len(cards) != 1 {
		t.Fatalf("got %d todo cards for dev-agent, want 1 (subtask)", len(cards))
	}

	// 10. Asset discovery by URL glob
	assets, err := c.ListAssets("", "", "*github*acme*")
	if err != nil {
		t.Fatalf("list assets: %v", err)
	}
	if len(assets) != 1 {
		t.Fatalf("got %d assets matching url glob, want 1", len(assets))
	}
	if assets[0].Project != "acme-backend" {
		t.Fatalf("asset project = %q, want 'acme-backend'", assets[0].Project)
	}

	// 11. Verify invalid transition is rejected (done -> in_flight)
	// First: completed -> in_flight (valid)
	_, err = c.UpdateCard(card.ID, client.CardUpdateRequest{Status: &status})
	if err != nil {
		t.Fatalf("completed -> in_flight should be valid: %v", err)
	}
	// Then: in_flight -> completed -> done
	c.UpdateCard(card.ID, client.CardUpdateRequest{Status: &completed})
	done := "done"
	c.UpdateCard(card.ID, client.CardUpdateRequest{Status: &done})
	// done -> in_flight should fail
	_, err = c.UpdateCard(card.ID, client.CardUpdateRequest{Status: &status})
	if err == nil {
		t.Fatal("expected error for done -> in_flight transition")
	}
}
