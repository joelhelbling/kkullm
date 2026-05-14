package db

import (
	"database/sql"
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

func TestMigrate_AddsAuthorNameColumnAndNullableAgentID(t *testing.T) {
	dbConn, _ := Open(":memory:")
	defer dbConn.Close()
	if err := Migrate(dbConn); err != nil {
		t.Fatalf("migrate: %v", err)
	}

	rows, err := dbConn.Query("PRAGMA table_info(comments)")
	if err != nil {
		t.Fatal(err)
	}
	defer rows.Close()

	cols := map[string]struct{ notNull int }{}
	for rows.Next() {
		var cid, notNull, pk int
		var name, ctype string
		var dflt sql.NullString
		_ = rows.Scan(&cid, &name, &ctype, &notNull, &dflt, &pk)
		cols[name] = struct{ notNull int }{notNull}
	}
	if _, ok := cols["author_name"]; !ok {
		t.Errorf("expected author_name column")
	}
	if cols["agent_id"].notNull != 0 {
		t.Errorf("expected agent_id to be nullable, got NOT NULL")
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
