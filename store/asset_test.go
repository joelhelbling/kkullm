package store

import (
	"testing"
)

func TestCreateAndListAssets(t *testing.T) {
	s := setupTestDB(t)
	proj := createTestProject(t, s)

	asset, err := s.CreateAsset(proj.ID, "GitHub repo", "Main source repo", "https://github.com/acme/backend")
	if err != nil {
		t.Fatalf("CreateAsset: %v", err)
	}
	if asset.Name != "GitHub repo" {
		t.Errorf("name = %q, want 'GitHub repo'", asset.Name)
	}

	s.CreateAsset(proj.ID, "Notion workspace", "Team docs", "https://notion.so/acme")
	s.CreateAsset(proj.ID, "Prod database", "PostgreSQL on AWS", "")

	assets, err := s.ListAssets(AssetListParams{Project: "test-project"})
	if err != nil {
		t.Fatalf("ListAssets: %v", err)
	}
	if len(assets) != 3 {
		t.Fatalf("got %d assets, want 3", len(assets))
	}
}

func TestListAssetsGlobName(t *testing.T) {
	s := setupTestDB(t)
	proj := createTestProject(t, s)

	s.CreateAsset(proj.ID, "GitHub repo", "", "https://github.com/acme/backend")
	s.CreateAsset(proj.ID, "GitHub Actions", "", "")
	s.CreateAsset(proj.ID, "Notion workspace", "", "")

	assets, err := s.ListAssets(AssetListParams{NameGlob: "GitHub*"})
	if err != nil {
		t.Fatalf("ListAssets name glob: %v", err)
	}
	if len(assets) != 2 {
		t.Errorf("got %d assets matching 'GitHub*', want 2", len(assets))
	}
}

func TestListAssetsGlobURL(t *testing.T) {
	s := setupTestDB(t)
	proj := createTestProject(t, s)

	s.CreateAsset(proj.ID, "Backend repo", "", "https://github.com/acme/backend")
	s.CreateAsset(proj.ID, "Frontend repo", "", "https://github.com/acme/frontend")
	s.CreateAsset(proj.ID, "Docs site", "", "https://notion.so/acme")

	assets, err := s.ListAssets(AssetListParams{URLGlob: "*github*acme*"})
	if err != nil {
		t.Fatalf("ListAssets url glob: %v", err)
	}
	if len(assets) != 2 {
		t.Errorf("got %d assets matching url '*github*acme*', want 2", len(assets))
	}
}

func TestGetAsset(t *testing.T) {
	s := setupTestDB(t)
	proj := createTestProject(t, s)

	created, _ := s.CreateAsset(proj.ID, "Test asset", "Description here", "https://example.com")

	got, err := s.GetAsset(created.ID)
	if err != nil {
		t.Fatalf("GetAsset: %v", err)
	}
	if got.Name != "Test asset" {
		t.Errorf("name = %q, want 'Test asset'", got.Name)
	}
	if got.Description != "Description here" {
		t.Errorf("description = %q, want 'Description here'", got.Description)
	}
	if got.Project != "test-project" {
		t.Errorf("project = %q, want 'test-project'", got.Project)
	}
}
