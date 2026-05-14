package store

import (
	"testing"
)

func TestCreateComment_SnapshotsAuthorName(t *testing.T) {
	s := setupTestDB(t)
	proj := createTestProject(t, s)
	agent := createTestAgent(t, s, "snapshot-agent", proj.ID)

	card, err := s.CreateCard(CardCreateParams{
		Title: "Test card", Status: "todo", ProjectID: proj.ID,
	})
	if err != nil {
		t.Fatalf("CreateCard: %v", err)
	}

	comment, err := s.CreateComment(card.ID, agent.ID, "hello")
	if err != nil {
		t.Fatalf("CreateComment: %v", err)
	}

	var authorName *string
	err = s.db.QueryRow(`SELECT author_name FROM comments WHERE id = ?`, comment.ID).Scan(&authorName)
	if err != nil {
		t.Fatalf("query author_name: %v", err)
	}
	if authorName == nil {
		t.Fatal("author_name is NULL, want snapshot of agent name")
	}
	if *authorName != agent.Name {
		t.Errorf("author_name = %q, want %q", *authorName, agent.Name)
	}
	if comment.Agent != agent.Name {
		t.Errorf("comment.Agent = %q, want %q", comment.Agent, agent.Name)
	}
}

func TestCreateAndListComments(t *testing.T) {
	s := setupTestDB(t)
	proj := createTestProject(t, s)
	agent := createTestAgent(t, s, "dev-agent", proj.ID)

	card, _ := s.CreateCard(CardCreateParams{
		Title: "Test card", Status: "todo", ProjectID: proj.ID,
	})

	comment, err := s.CreateComment(card.ID, agent.ID, "Started working on this")
	if err != nil {
		t.Fatalf("CreateComment: %v", err)
	}
	if comment.Body != "Started working on this" {
		t.Errorf("body = %q, want 'Started working on this'", comment.Body)
	}
	if comment.Agent != "dev-agent" {
		t.Errorf("agent = %q, want 'dev-agent'", comment.Agent)
	}

	s.CreateComment(card.ID, agent.ID, "Making progress")

	comments, err := s.ListComments(card.ID)
	if err != nil {
		t.Fatalf("ListComments: %v", err)
	}
	if len(comments) != 2 {
		t.Fatalf("got %d comments, want 2", len(comments))
	}

	got, _ := s.GetCard(card.ID)
	if got.CommentCount != 2 {
		t.Errorf("comment_count = %d, want 2", got.CommentCount)
	}
}

// Regression: after the author has been deleted, comments.agent_id is NULL,
// and ListComments must still return the rows (scanning NULL into the model's
// int AgentID via COALESCE), surfacing the snapshot name.
func TestListComments_HandlesDeletedAuthor(t *testing.T) {
	s := setupTestDB(t)
	proj := createTestProject(t, s)
	agent := createTestAgent(t, s, "queen_bee", proj.ID)

	card, _ := s.CreateCard(CardCreateParams{
		Title: "Test card", Status: "todo", ProjectID: proj.ID,
	})
	if _, err := s.CreateComment(card.ID, agent.ID, "discretion above all"); err != nil {
		t.Fatalf("CreateComment: %v", err)
	}

	if err := s.DeleteAgent(agent.ID); err != nil {
		t.Fatalf("DeleteAgent: %v", err)
	}

	comments, err := s.ListComments(card.ID)
	if err != nil {
		t.Fatalf("ListComments after agent delete: %v", err)
	}
	if len(comments) != 1 {
		t.Fatalf("got %d comments, want 1", len(comments))
	}
	if comments[0].AgentID != 0 {
		t.Errorf("AgentID = %d, want 0 (sentinel for deleted agent)", comments[0].AgentID)
	}
	if comments[0].Agent != "queen_bee" {
		t.Errorf("Agent = %q, want snapshot %q", comments[0].Agent, "queen_bee")
	}
}
