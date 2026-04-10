package store

import (
	"strings"
	"testing"

	"github.com/joelhelbling/kkullm/model"
)

func createTestProject(t *testing.T, s *Store) *model.Project {
	t.Helper()
	p, err := s.CreateProject("test-project", "")
	if err != nil {
		t.Fatalf("CreateProject: %v", err)
	}
	return p
}

func createTestAgent(t *testing.T, s *Store, name string, projectID int) *model.Agent {
	t.Helper()
	a, err := s.CreateAgent(name, projectID, "")
	if err != nil {
		t.Fatalf("CreateAgent: %v", err)
	}
	return a
}

func strPtr(s string) *string { return &s }

func TestCreateAndGetCard(t *testing.T) {
	s := setupTestDB(t)
	proj := createTestProject(t, s)

	card, err := s.CreateCard(CardCreateParams{
		Title:     "My first card",
		Body:      "Some body text",
		ProjectID: proj.ID,
	})
	if err != nil {
		t.Fatalf("CreateCard: %v", err)
	}

	if card.ID == 0 {
		t.Error("expected non-zero card ID")
	}
	if card.Title != "My first card" {
		t.Errorf("title = %q, want 'My first card'", card.Title)
	}
	if card.Body != "Some body text" {
		t.Errorf("body = %q, want 'Some body text'", card.Body)
	}
	if card.Status != "considering" {
		t.Errorf("status = %q, want 'considering'", card.Status)
	}
	if card.ProjectID != proj.ID {
		t.Errorf("project_id = %d, want %d", card.ProjectID, proj.ID)
	}
	if card.Project != "test-project" {
		t.Errorf("project = %q, want 'test-project'", card.Project)
	}

	got, err := s.GetCard(card.ID)
	if err != nil {
		t.Fatalf("GetCard: %v", err)
	}
	if got.Title != card.Title {
		t.Errorf("GetCard title = %q, want %q", got.Title, card.Title)
	}
}

func TestCreateCardWithAssigneesAndTags(t *testing.T) {
	s := setupTestDB(t)
	proj := createTestProject(t, s)
	agent1 := createTestAgent(t, s, "agent-alpha", proj.ID)
	agent2 := createTestAgent(t, s, "agent-beta", proj.ID)

	card, err := s.CreateCard(CardCreateParams{
		Title:     "Card with assignees and tags",
		ProjectID: proj.ID,
		Assignees: []string{agent1.Name, agent2.Name},
		Tags:      []string{"backend", "urgent"},
	})
	if err != nil {
		t.Fatalf("CreateCard: %v", err)
	}

	got, err := s.GetCard(card.ID)
	if err != nil {
		t.Fatalf("GetCard: %v", err)
	}

	if len(got.Assignees) != 2 {
		t.Fatalf("assignees count = %d, want 2", len(got.Assignees))
	}
	// Assignees are ordered by name
	if got.Assignees[0] != "agent-alpha" {
		t.Errorf("assignees[0] = %q, want 'agent-alpha'", got.Assignees[0])
	}
	if got.Assignees[1] != "agent-beta" {
		t.Errorf("assignees[1] = %q, want 'agent-beta'", got.Assignees[1])
	}

	if len(got.Tags) != 2 {
		t.Fatalf("tags count = %d, want 2", len(got.Tags))
	}
	// Tags are ordered alphabetically
	if got.Tags[0] != "backend" {
		t.Errorf("tags[0] = %q, want 'backend'", got.Tags[0])
	}
	if got.Tags[1] != "urgent" {
		t.Errorf("tags[1] = %q, want 'urgent'", got.Tags[1])
	}
}

func TestCreateCardWithRelations(t *testing.T) {
	s := setupTestDB(t)
	proj := createTestProject(t, s)

	parent, err := s.CreateCard(CardCreateParams{
		Title:     "Parent card",
		ProjectID: proj.ID,
	})
	if err != nil {
		t.Fatalf("CreateCard parent: %v", err)
	}

	child, err := s.CreateCard(CardCreateParams{
		Title:     "Child card",
		ProjectID: proj.ID,
		Relations: []model.CardRelation{
			{RelatedCardID: parent.ID, RelationType: "belongs_to"},
		},
	})
	if err != nil {
		t.Fatalf("CreateCard child: %v", err)
	}

	got, err := s.GetCard(child.ID)
	if err != nil {
		t.Fatalf("GetCard: %v", err)
	}

	if len(got.Relations) != 1 {
		t.Fatalf("relations count = %d, want 1", len(got.Relations))
	}
	if got.Relations[0].RelatedCardID != parent.ID {
		t.Errorf("related_card_id = %d, want %d", got.Relations[0].RelatedCardID, parent.ID)
	}
	if got.Relations[0].RelationType != "belongs_to" {
		t.Errorf("relation_type = %q, want 'belongs_to'", got.Relations[0].RelationType)
	}
}

func TestListCardsFiltered(t *testing.T) {
	s := setupTestDB(t)
	proj := createTestProject(t, s)
	agent := createTestAgent(t, s, "test-agent", proj.ID)

	// Card 1: considering, assigned, tagged
	_, err := s.CreateCard(CardCreateParams{
		Title:     "Card One",
		ProjectID: proj.ID,
		Status:    "considering",
		Assignees: []string{agent.Name},
		Tags:      []string{"alpha"},
	})
	if err != nil {
		t.Fatalf("CreateCard 1: %v", err)
	}

	// Card 2: todo, no assignee, no tag
	_, err = s.CreateCard(CardCreateParams{
		Title:     "Card Two",
		ProjectID: proj.ID,
		Status:    "todo",
	})
	if err != nil {
		t.Fatalf("CreateCard 2: %v", err)
	}

	// Card 3: considering, no assignee, tagged differently
	_, err = s.CreateCard(CardCreateParams{
		Title:     "Card Three",
		ProjectID: proj.ID,
		Status:    "considering",
		Tags:      []string{"beta"},
	})
	if err != nil {
		t.Fatalf("CreateCard 3: %v", err)
	}

	// Filter by status=considering
	cards, err := s.ListCards(CardListParams{Status: "considering"})
	if err != nil {
		t.Fatalf("ListCards(status=considering): %v", err)
	}
	if len(cards) != 2 {
		t.Errorf("status=considering: got %d cards, want 2", len(cards))
	}

	// Filter by status=todo
	cards, err = s.ListCards(CardListParams{Status: "todo"})
	if err != nil {
		t.Fatalf("ListCards(status=todo): %v", err)
	}
	if len(cards) != 1 {
		t.Errorf("status=todo: got %d cards, want 1", len(cards))
	}

	// Filter by assignee
	cards, err = s.ListCards(CardListParams{Assignee: agent.Name})
	if err != nil {
		t.Fatalf("ListCards(assignee): %v", err)
	}
	if len(cards) != 1 {
		t.Errorf("assignee filter: got %d cards, want 1", len(cards))
	}

	// Filter by tag=alpha
	cards, err = s.ListCards(CardListParams{Tag: "alpha"})
	if err != nil {
		t.Fatalf("ListCards(tag=alpha): %v", err)
	}
	if len(cards) != 1 {
		t.Errorf("tag=alpha: got %d cards, want 1", len(cards))
	}

	// Filter by project
	cards, err = s.ListCards(CardListParams{Project: "test-project"})
	if err != nil {
		t.Fatalf("ListCards(project): %v", err)
	}
	if len(cards) != 3 {
		t.Errorf("project filter: got %d cards, want 3", len(cards))
	}

	// Filter by comma-separated statuses
	cards, err = s.ListCards(CardListParams{Status: "considering,todo"})
	if err != nil {
		t.Fatalf("ListCards(status=considering,todo): %v", err)
	}
	if len(cards) != 3 {
		t.Errorf("multi-status filter: got %d cards, want 3", len(cards))
	}
}

func TestUpdateCard(t *testing.T) {
	s := setupTestDB(t)
	proj := createTestProject(t, s)

	card, err := s.CreateCard(CardCreateParams{
		Title:     "Original title",
		ProjectID: proj.ID,
	})
	if err != nil {
		t.Fatalf("CreateCard: %v", err)
	}

	updated, err := s.UpdateCard(card.ID, CardUpdateParams{
		Title:  strPtr("Updated title"),
		Status: strPtr("todo"),
	})
	if err != nil {
		t.Fatalf("UpdateCard: %v", err)
	}

	if updated.Title != "Updated title" {
		t.Errorf("title = %q, want 'Updated title'", updated.Title)
	}
	if updated.Status != "todo" {
		t.Errorf("status = %q, want 'todo'", updated.Status)
	}
}

func TestUpdateCardInvalidTransition(t *testing.T) {
	s := setupTestDB(t)
	proj := createTestProject(t, s)

	card, err := s.CreateCard(CardCreateParams{
		Title:     "Test card",
		ProjectID: proj.ID,
		Status:    "considering",
	})
	if err != nil {
		t.Fatalf("CreateCard: %v", err)
	}

	// considering -> in_flight is invalid
	_, err = s.UpdateCard(card.ID, CardUpdateParams{
		Status: strPtr("in_flight"),
	})
	if err == nil {
		t.Fatal("expected error for invalid transition considering -> in_flight, got nil")
	}
	if !strings.Contains(err.Error(), "invalid status transition") {
		t.Errorf("error message = %q, expected 'invalid status transition'", err.Error())
	}
}

func TestUpdateCardAddRelations(t *testing.T) {
	s := setupTestDB(t)
	proj := createTestProject(t, s)

	blocker, err := s.CreateCard(CardCreateParams{
		Title:     "Blocker card",
		ProjectID: proj.ID,
	})
	if err != nil {
		t.Fatalf("CreateCard blocker: %v", err)
	}

	card, err := s.CreateCard(CardCreateParams{
		Title:     "Blocked card",
		ProjectID: proj.ID,
	})
	if err != nil {
		t.Fatalf("CreateCard: %v", err)
	}

	updated, err := s.UpdateCard(card.ID, CardUpdateParams{
		Relations: []model.CardRelation{
			{RelatedCardID: blocker.ID, RelationType: "blocked_by"},
		},
	})
	if err != nil {
		t.Fatalf("UpdateCard: %v", err)
	}

	if len(updated.Relations) != 1 {
		t.Fatalf("relations count = %d, want 1", len(updated.Relations))
	}
	if updated.Relations[0].RelatedCardID != blocker.ID {
		t.Errorf("related_card_id = %d, want %d", updated.Relations[0].RelatedCardID, blocker.ID)
	}
	if updated.Relations[0].RelationType != "blocked_by" {
		t.Errorf("relation_type = %q, want 'blocked_by'", updated.Relations[0].RelationType)
	}
}
