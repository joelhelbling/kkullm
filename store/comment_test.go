package store

import (
	"testing"
)

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
