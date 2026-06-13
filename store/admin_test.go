package store

import (
	"testing"
)

func TestPurge_WipesAllDataAndReSeeds(t *testing.T) {
	s := setupTestDB(t)

	proj, err := s.CreateProject("doomed-proj", "to be purged")
	if err != nil {
		t.Fatalf("CreateProject: %v", err)
	}
	agent := createTestAgent(t, s, "doomed-agent", proj.ID)

	card, err := s.CreateCard(CardCreateParams{
		Title:     "doomed card",
		ProjectID: proj.ID,
		Assignees: []string{agent.Name},
	})
	if err != nil {
		t.Fatalf("CreateCard: %v", err)
	}

	if _, err := s.CreateComment(card.ID, agent.ID, "doomed comment", ""); err != nil {
		t.Fatalf("CreateComment: %v", err)
	}

	if err := s.Purge(); err != nil {
		t.Fatalf("Purge: %v", err)
	}

	countOf := func(table string) int {
		t.Helper()
		var n int
		if err := s.db.QueryRow("SELECT COUNT(*) FROM " + table).Scan(&n); err != nil {
			t.Fatalf("count %s: %v", table, err)
		}
		return n
	}

	if got := countOf("projects"); got != 1 {
		t.Errorf("projects count = %d, want 1 (re-seeded orchestration)", got)
	}
	if got := countOf("agents"); got != 1 {
		t.Errorf("agents count = %d, want 1 (re-seeded user)", got)
	}
	if got := countOf("cards"); got != 0 {
		t.Errorf("cards count = %d, want 0", got)
	}
	if got := countOf("comments"); got != 0 {
		t.Errorf("comments count = %d, want 0", got)
	}

	// Verify the re-seeded baseline is correct by name.
	var projName string
	if err := s.db.QueryRow("SELECT name FROM projects").Scan(&projName); err != nil {
		t.Fatalf("scan project name: %v", err)
	}
	if projName != "orchestration" {
		t.Errorf("seeded project name = %q, want 'orchestration'", projName)
	}

	var agentName string
	if err := s.db.QueryRow("SELECT name FROM agents").Scan(&agentName); err != nil {
		t.Fatalf("scan agent name: %v", err)
	}
	if agentName != "user" {
		t.Errorf("seeded agent name = %q, want 'user'", agentName)
	}
}
