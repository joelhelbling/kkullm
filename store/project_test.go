package store

import (
	"testing"

	"github.com/joelhelbling/kkullm/db"
)

func setupTestDB(t *testing.T) *Store {
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
	return New(database)
}

func TestCreateAndListProjects(t *testing.T) {
	s := setupTestDB(t)

	projects, err := s.ListProjects()
	if err != nil {
		t.Fatalf("ListProjects: %v", err)
	}
	if len(projects) != 1 {
		t.Fatalf("got %d projects, want 1 (orchestration)", len(projects))
	}

	p, err := s.CreateProject("acme-backend", "The ACME backend service")
	if err != nil {
		t.Fatalf("CreateProject: %v", err)
	}
	if p.Name != "acme-backend" {
		t.Errorf("name = %q, want 'acme-backend'", p.Name)
	}
	if p.ID == 0 {
		t.Error("expected non-zero ID")
	}

	projects, err = s.ListProjects()
	if err != nil {
		t.Fatalf("ListProjects: %v", err)
	}
	if len(projects) != 2 {
		t.Fatalf("got %d projects, want 2", len(projects))
	}
}

func TestCreateProjectDuplicateName(t *testing.T) {
	s := setupTestDB(t)

	_, err := s.CreateProject("acme-backend", "")
	if err != nil {
		t.Fatalf("first create: %v", err)
	}

	_, err = s.CreateProject("acme-backend", "")
	if err == nil {
		t.Error("expected error on duplicate name, got nil")
	}
}

func TestUpdateProject_OK(t *testing.T) {
	s := setupTestDB(t)

	created, err := s.CreateProject("old-name", "old description")
	if err != nil {
		t.Fatalf("CreateProject: %v", err)
	}

	if err := s.UpdateProject(created.ID, "new-name", "new description"); err != nil {
		t.Fatalf("UpdateProject: %v", err)
	}

	found, err := s.GetProject(created.ID)
	if err != nil {
		t.Fatalf("GetProject: %v", err)
	}
	if found.Name != "new-name" {
		t.Errorf("name = %q, want 'new-name'", found.Name)
	}
	if found.Description != "new description" {
		t.Errorf("description = %q, want 'new description'", found.Description)
	}
}

func TestUpdateProject_DuplicateName(t *testing.T) {
	s := setupTestDB(t)

	if _, err := s.CreateProject("first", ""); err != nil {
		t.Fatalf("CreateProject first: %v", err)
	}
	second, err := s.CreateProject("second", "")
	if err != nil {
		t.Fatalf("CreateProject second: %v", err)
	}

	if err := s.UpdateProject(second.ID, "first", ""); err == nil {
		t.Error("expected error updating to duplicate name, got nil")
	}
}

func TestUpdateProject_EmptyName(t *testing.T) {
	s := setupTestDB(t)

	created, err := s.CreateProject("some-proj", "")
	if err != nil {
		t.Fatalf("CreateProject: %v", err)
	}

	if err := s.UpdateProject(created.ID, "", "desc"); err == nil {
		t.Error("expected error updating to empty name, got nil")
	}
}

func TestDeleteProject_CascadesAllChildren(t *testing.T) {
	s := setupTestDB(t)

	proj := createTestProject(t, s)
	agent := createTestAgent(t, s, "doomed-agent", proj.ID)

	card, err := s.CreateCard(CardCreateParams{
		Title:     "doomed card",
		ProjectID: proj.ID,
		Assignees: []string{agent.Name},
	})
	if err != nil {
		t.Fatalf("CreateCard: %v", err)
	}

	if _, err := s.CreateComment(card.ID, agent.ID, "doomed comment"); err != nil {
		t.Fatalf("CreateComment: %v", err)
	}

	if _, err := s.db.Exec(
		"INSERT INTO project_assets (project_id, name, description, url) VALUES (?, ?, ?, ?)",
		proj.ID, "docs", "Some docs", "https://example.com",
	); err != nil {
		t.Fatalf("insert project_asset: %v", err)
	}

	if err := s.DeleteProject(proj.ID); err != nil {
		t.Fatalf("DeleteProject: %v", err)
	}

	assertCount := func(label, query string, args ...any) {
		t.Helper()
		var n int
		if err := s.db.QueryRow(query, args...).Scan(&n); err != nil {
			t.Fatalf("%s count query: %v", label, err)
		}
		if n != 0 {
			t.Errorf("%s: got %d rows, want 0", label, n)
		}
	}

	assertCount("projects", "SELECT COUNT(*) FROM projects WHERE id = ?", proj.ID)
	assertCount("agents", "SELECT COUNT(*) FROM agents WHERE project_id = ?", proj.ID)
	assertCount("cards", "SELECT COUNT(*) FROM cards WHERE project_id = ?", proj.ID)
	assertCount("comments", "SELECT COUNT(*) FROM comments WHERE card_id = ?", card.ID)
	assertCount("project_assets", "SELECT COUNT(*) FROM project_assets WHERE project_id = ?", proj.ID)
}

func TestGetProjectByID(t *testing.T) {
	s := setupTestDB(t)

	created, err := s.CreateProject("test-proj", "A test project")
	if err != nil {
		t.Fatalf("CreateProject: %v", err)
	}

	found, err := s.GetProject(created.ID)
	if err != nil {
		t.Fatalf("GetProject: %v", err)
	}
	if found.Name != "test-proj" {
		t.Errorf("name = %q, want 'test-proj'", found.Name)
	}
	if found.Description != "A test project" {
		t.Errorf("description = %q, want 'A test project'", found.Description)
	}
}
