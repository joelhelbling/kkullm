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

func TestRenameProject_OK(t *testing.T) {
	s := setupTestDB(t)

	created, err := s.CreateProject("old-name", "")
	if err != nil {
		t.Fatalf("CreateProject: %v", err)
	}

	if err := s.RenameProject(created.ID, "new-name"); err != nil {
		t.Fatalf("RenameProject: %v", err)
	}

	found, err := s.GetProject(created.ID)
	if err != nil {
		t.Fatalf("GetProject: %v", err)
	}
	if found.Name != "new-name" {
		t.Errorf("name = %q, want 'new-name'", found.Name)
	}
}

func TestRenameProject_DuplicateName(t *testing.T) {
	s := setupTestDB(t)

	if _, err := s.CreateProject("first", ""); err != nil {
		t.Fatalf("CreateProject first: %v", err)
	}
	second, err := s.CreateProject("second", "")
	if err != nil {
		t.Fatalf("CreateProject second: %v", err)
	}

	if err := s.RenameProject(second.ID, "first"); err == nil {
		t.Error("expected error renaming to duplicate name, got nil")
	}
}

func TestRenameProject_Empty(t *testing.T) {
	s := setupTestDB(t)

	created, err := s.CreateProject("some-proj", "")
	if err != nil {
		t.Fatalf("CreateProject: %v", err)
	}

	if err := s.RenameProject(created.ID, ""); err == nil {
		t.Error("expected error renaming to empty name, got nil")
	}
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
