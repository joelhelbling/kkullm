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
