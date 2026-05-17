package store

import (
	"testing"
)

func TestCreateAndListAgents(t *testing.T) {
	s := setupTestDB(t)

	agents, err := s.ListAgents("")
	if err != nil {
		t.Fatalf("ListAgents: %v", err)
	}
	if len(agents) != 1 {
		t.Fatalf("got %d agents, want 1 (user)", len(agents))
	}
	if agents[0].Name != "user" {
		t.Errorf("seeded agent name = %q, want 'user'", agents[0].Name)
	}

	proj, err := s.CreateProject("acme", "")
	if err != nil {
		t.Fatalf("CreateProject: %v", err)
	}

	agent, err := s.CreateAgent("dev-agent", proj.ID, "Writes Go code")
	if err != nil {
		t.Fatalf("CreateAgent: %v", err)
	}
	if agent.Name != "dev-agent" {
		t.Errorf("name = %q, want 'dev-agent'", agent.Name)
	}
	if agent.Bio != "Writes Go code" {
		t.Errorf("bio = %q, want 'Writes Go code'", agent.Bio)
	}

	agents, err = s.ListAgents("")
	if err != nil {
		t.Fatalf("ListAgents: %v", err)
	}
	if len(agents) != 2 {
		t.Fatalf("got %d agents, want 2", len(agents))
	}

	agents, err = s.ListAgents("acme")
	if err != nil {
		t.Fatalf("ListAgents(acme): %v", err)
	}
	if len(agents) != 1 {
		t.Fatalf("got %d agents for acme, want 1", len(agents))
	}
}

func TestUpdateAgent_OK_BackfillsHistoricalComments(t *testing.T) {
	s := setupTestDB(t)
	proj := createTestProject(t, s)
	agent := createTestAgent(t, s, "old-name", proj.ID)

	card, err := s.CreateCard(CardCreateParams{
		Title:     "test card",
		ProjectID: proj.ID,
	})
	if err != nil {
		t.Fatalf("CreateCard: %v", err)
	}
	if _, err := s.CreateComment(card.ID, agent.ID, "a comment"); err != nil {
		t.Fatalf("CreateComment: %v", err)
	}

	if err := s.UpdateAgent(agent.ID, "new-name", "new bio"); err != nil {
		t.Fatalf("UpdateAgent: %v", err)
	}

	var authorName string
	if err := s.db.QueryRow(
		"SELECT author_name FROM comments WHERE agent_id = ?", agent.ID,
	).Scan(&authorName); err != nil {
		t.Fatalf("query author_name: %v", err)
	}
	if authorName != "new-name" {
		t.Errorf("author_name = %q, want 'new-name'", authorName)
	}

	reloaded, err := s.GetAgent(agent.ID)
	if err != nil {
		t.Fatalf("GetAgent: %v", err)
	}
	if reloaded.Name != "new-name" {
		t.Errorf("agent name = %q, want 'new-name'", reloaded.Name)
	}
	if reloaded.Bio != "new bio" {
		t.Errorf("agent bio = %q, want 'new bio'", reloaded.Bio)
	}
}

func TestUpdateAgent_DuplicateName(t *testing.T) {
	s := setupTestDB(t)
	proj := createTestProject(t, s)
	a1 := createTestAgent(t, s, "alpha", proj.ID)
	a2 := createTestAgent(t, s, "beta", proj.ID)

	if err := s.UpdateAgent(a2.ID, "alpha", ""); err == nil {
		t.Fatalf("expected error updating to duplicate name, got nil")
	}

	reloaded, err := s.GetAgent(a1.ID)
	if err != nil {
		t.Fatalf("GetAgent: %v", err)
	}
	if reloaded.Name != "alpha" {
		t.Errorf("alpha agent name = %q, want 'alpha'", reloaded.Name)
	}
}

func TestUpdateAgent_EmptyName(t *testing.T) {
	s := setupTestDB(t)
	proj := createTestProject(t, s)
	agent := createTestAgent(t, s, "alpha", proj.ID)

	if err := s.UpdateAgent(agent.ID, "", "bio"); err == nil {
		t.Fatalf("expected error for empty name, got nil")
	}
}

func TestDeleteAgent_UnassignsCards_PreservesComments(t *testing.T) {
	s := setupTestDB(t)

	proj := createTestProject(t, s)
	agent := createTestAgent(t, s, "alice", proj.ID)

	card, err := s.CreateCard(CardCreateParams{
		Title:     "Test card",
		ProjectID: proj.ID,
		Assignees: []string{"alice"},
	})
	if err != nil {
		t.Fatalf("CreateCard: %v", err)
	}

	if _, err := s.CreateComment(card.ID, agent.ID, "Working on it"); err != nil {
		t.Fatalf("CreateComment: %v", err)
	}

	if err := s.DeleteAgent(agent.ID); err != nil {
		t.Fatalf("DeleteAgent: %v", err)
	}

	var n int
	if err := s.db.QueryRow("SELECT COUNT(*) FROM agents WHERE id = ?", agent.ID).Scan(&n); err != nil {
		t.Fatalf("count agents: %v", err)
	}
	if n != 0 {
		t.Errorf("agents rows for id = %d, want 0", n)
	}

	if err := s.db.QueryRow("SELECT COUNT(*) FROM cards WHERE id = ?", card.ID).Scan(&n); err != nil {
		t.Fatalf("count cards: %v", err)
	}
	if n != 1 {
		t.Errorf("cards rows for id = %d, want 1", n)
	}

	if err := s.db.QueryRow("SELECT COUNT(*) FROM card_assignees WHERE agent_id = ?", agent.ID).Scan(&n); err != nil {
		t.Fatalf("count card_assignees: %v", err)
	}
	if n != 0 {
		t.Errorf("card_assignees rows for agent_id = %d, want 0", n)
	}

	if err := s.db.QueryRow("SELECT COUNT(*) FROM comments WHERE card_id = ?", card.ID).Scan(&n); err != nil {
		t.Fatalf("count comments: %v", err)
	}
	if n != 1 {
		t.Errorf("comments rows = %d, want 1", n)
	}

	var (
		agentID    *int
		authorName string
	)
	if err := s.db.QueryRow("SELECT agent_id, author_name FROM comments WHERE card_id = ?", card.ID).Scan(&agentID, &authorName); err != nil {
		t.Fatalf("query comment: %v", err)
	}
	if agentID != nil {
		t.Errorf("comment.agent_id = %v, want NULL", *agentID)
	}
	if authorName != "alice" {
		t.Errorf("comment.author_name = %q, want %q", authorName, "alice")
	}
}

func TestGetAgentByName(t *testing.T) {
	s := setupTestDB(t)

	agent, err := s.GetAgentByName("user")
	if err != nil {
		t.Fatalf("GetAgentByName: %v", err)
	}
	if agent.Name != "user" {
		t.Errorf("name = %q, want 'user'", agent.Name)
	}
	if agent.Project != "orchestration" {
		t.Errorf("project = %q, want 'orchestration'", agent.Project)
	}
}
