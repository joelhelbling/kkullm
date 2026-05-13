package db

import (
	"testing"
)

func TestOpenAndMigrate(t *testing.T) {
	database, err := Open(":memory:")
	if err != nil {
		t.Fatalf("Open: %v", err)
	}
	defer database.Close()

	if err := Migrate(database); err != nil {
		t.Fatalf("Migrate: %v", err)
	}

	tables := []string{"projects", "agents", "cards", "card_assignees", "card_tags", "card_relations", "comments", "project_assets"}
	for _, table := range tables {
		var count int
		err := database.QueryRow("SELECT COUNT(*) FROM " + table).Scan(&count)
		if err != nil {
			t.Errorf("table %s does not exist: %v", table, err)
		}
	}
}

func TestSeed(t *testing.T) {
	database, err := Open(":memory:")
	if err != nil {
		t.Fatalf("Open: %v", err)
	}
	defer database.Close()

	if err := Migrate(database); err != nil {
		t.Fatalf("Migrate: %v", err)
	}

	if err := Seed(database); err != nil {
		t.Fatalf("Seed: %v", err)
	}

	var projName string
	err = database.QueryRow("SELECT name FROM projects WHERE name = 'orchestration'").Scan(&projName)
	if err != nil {
		t.Fatalf("orchestration project not found: %v", err)
	}

	var agentName, agentProjectName string
	err = database.QueryRow(`
		SELECT a.name, p.name FROM agents a
		JOIN projects p ON a.project_id = p.id
		WHERE a.name = 'user'
	`).Scan(&agentName, &agentProjectName)
	if err != nil {
		t.Fatalf("user agent not found: %v", err)
	}
	if agentProjectName != "orchestration" {
		t.Errorf("user agent project = %q, want 'orchestration'", agentProjectName)
	}

	if err := Seed(database); err != nil {
		t.Fatalf("second Seed call failed: %v", err)
	}
}

func TestSeedCreatesUserAgent(t *testing.T) {
	database, err := Open(":memory:")
	if err != nil {
		t.Fatalf("Open: %v", err)
	}
	defer database.Close()

	if err := Migrate(database); err != nil {
		t.Fatalf("Migrate: %v", err)
	}
	if err := Seed(database); err != nil {
		t.Fatalf("Seed: %v", err)
	}

	var count int
	if err := database.QueryRow(`SELECT COUNT(*) FROM agents WHERE name = 'user'`).Scan(&count); err != nil {
		t.Fatalf("query user agent: %v", err)
	}
	if count != 1 {
		t.Fatalf("expected exactly 1 'user' agent after Seed, got %d", count)
	}

	// Idempotency: re-running Seed must not duplicate.
	if err := Seed(database); err != nil {
		t.Fatalf("re-Seed: %v", err)
	}
	if err := database.QueryRow(`SELECT COUNT(*) FROM agents WHERE name = 'user'`).Scan(&count); err != nil {
		t.Fatalf("query user agent after re-seed: %v", err)
	}
	if count != 1 {
		t.Fatalf("expected 1 'user' agent after re-Seed, got %d", count)
	}
}
