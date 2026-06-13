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

func TestUpdateAsset(t *testing.T) {
	s := setupTestDB(t)
	proj := createTestProject(t, s)

	created, _ := s.CreateAsset(proj.ID, "Old name", "old desc", "https://old.example.com")

	if err := s.UpdateAsset(created.ID, "New name", "new desc", "https://new.example.com"); err != nil {
		t.Fatalf("UpdateAsset: %v", err)
	}

	got, err := s.GetAsset(created.ID)
	if err != nil {
		t.Fatalf("GetAsset: %v", err)
	}
	if got.Name != "New name" {
		t.Errorf("name = %q, want 'New name'", got.Name)
	}
	if got.Description != "new desc" {
		t.Errorf("description = %q, want 'new desc'", got.Description)
	}
	if got.URL != "https://new.example.com" {
		t.Errorf("url = %q, want 'https://new.example.com'", got.URL)
	}
}

func TestUpdateAssetEmptyName(t *testing.T) {
	s := setupTestDB(t)
	proj := createTestProject(t, s)
	created, _ := s.CreateAsset(proj.ID, "Name", "", "")

	if err := s.UpdateAsset(created.ID, "", "", ""); err == nil {
		t.Errorf("expected error for empty name, got nil")
	}
}

func TestUpdateAssetClearsOptionalFields(t *testing.T) {
	s := setupTestDB(t)
	proj := createTestProject(t, s)
	created, _ := s.CreateAsset(proj.ID, "Name", "desc", "https://x.example.com")

	if err := s.UpdateAsset(created.ID, "Name", "", ""); err != nil {
		t.Fatalf("UpdateAsset: %v", err)
	}
	got, _ := s.GetAsset(created.ID)
	if got.Description != "" {
		t.Errorf("description = %q, want empty", got.Description)
	}
	if got.URL != "" {
		t.Errorf("url = %q, want empty", got.URL)
	}
}

func TestDeleteAsset(t *testing.T) {
	s := setupTestDB(t)
	proj := createTestProject(t, s)
	created, _ := s.CreateAsset(proj.ID, "Doomed", "", "")

	if err := s.DeleteAsset(created.ID); err != nil {
		t.Fatalf("DeleteAsset: %v", err)
	}
	if _, err := s.GetAsset(created.ID); err == nil {
		t.Errorf("expected asset to be gone after delete")
	}
}

func TestDeleteAssetNotFound(t *testing.T) {
	s := setupTestDB(t)
	if err := s.DeleteAsset(99999); err == nil {
		t.Errorf("expected error deleting nonexistent asset, got nil")
	}
}
