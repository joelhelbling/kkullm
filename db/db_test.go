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
	dbConn, err := Open(":memory:")
	if err != nil {
		t.Fatal(err)
	}
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

func TestMigrate_IdempotentAcrossRuns(t *testing.T) {
	d, err := Open(":memory:")
	if err != nil {
		t.Fatal(err)
	}
	defer d.Close()
	if err := Migrate(d); err != nil {
		t.Fatalf("first migrate: %v", err)
	}
	if err := Migrate(d); err != nil {
		t.Fatalf("second migrate should be no-op, got: %v", err)
	}
}

func TestMigrate_AddsBlockedAndKindColumns(t *testing.T) {
	d, err := Open(":memory:")
	if err != nil {
		t.Fatal(err)
	}
	defer d.Close()
	if err := Migrate(d); err != nil {
		t.Fatalf("migrate: %v", err)
	}

	hasColumn := func(table, col string) bool {
		rows, err := d.Query("PRAGMA table_info(" + table + ")")
		if err != nil {
			t.Fatalf("PRAGMA table_info(%s): %v", table, err)
		}
		defer rows.Close()
		for rows.Next() {
			var cid, notNull, pk int
			var name, ctype string
			var dflt sql.NullString
			_ = rows.Scan(&cid, &name, &ctype, &notNull, &dflt, &pk)
			if name == col {
				return true
			}
		}
		return false
	}

	if !hasColumn("cards", "blocked") {
		t.Error("expected cards.blocked column after migrate")
	}
	if !hasColumn("comments", "kind") {
		t.Error("expected comments.kind column after migrate")
	}
}

func TestMigrate_ConvertsBlockedStatusToFlag(t *testing.T) {
	d, err := Open(":memory:")
	if err != nil {
		t.Fatal(err)
	}
	defer d.Close()

	// Run only the first two migrations by hand so we can insert a legacy
	// status='blocked' row, then run the full Migrate to apply 003.
	if _, err := d.Exec(`CREATE TABLE IF NOT EXISTS schema_migrations (version TEXT NOT NULL PRIMARY KEY)`); err != nil {
		t.Fatalf("create schema_migrations: %v", err)
	}
	for _, name := range []string{"migrations/001_initial.sql", "migrations/002_comments_author_snapshot.sql"} {
		data, err := migrations.ReadFile(name)
		if err != nil {
			t.Fatalf("read %s: %v", name, err)
		}
		if _, err := d.Exec(string(data)); err != nil {
			t.Fatalf("exec %s: %v", name, err)
		}
		if _, err := d.Exec(`INSERT INTO schema_migrations(version) VALUES (?)`, name); err != nil {
			t.Fatalf("record %s: %v", name, err)
		}
	}
	if _, err := d.Exec(`INSERT INTO projects (id, name) VALUES (1, 'p')`); err != nil {
		t.Fatalf("insert project: %v", err)
	}
	if _, err := d.Exec(`INSERT INTO cards (id, title, status, project_id) VALUES (7, 'legacy', 'blocked', 1)`); err != nil {
		t.Fatalf("insert legacy blocked card: %v", err)
	}

	if err := Migrate(d); err != nil {
		t.Fatalf("migrate: %v", err)
	}

	var status string
	var blocked int
	if err := d.QueryRow(`SELECT status, blocked FROM cards WHERE id = 7`).Scan(&status, &blocked); err != nil {
		t.Fatalf("query migrated card: %v", err)
	}
	if status != "todo" {
		t.Errorf("legacy blocked card status = %q, want \"todo\"", status)
	}
	if blocked != 1 {
		t.Errorf("legacy blocked card blocked = %d, want 1", blocked)
	}
}

func TestMigrate_CreatesCardEventsTable(t *testing.T) {
	d, err := Open(":memory:")
	if err != nil {
		t.Fatal(err)
	}
	defer d.Close()
	if err := Migrate(d); err != nil {
		t.Fatalf("migrate: %v", err)
	}

	cols := map[string]struct{}{}
	rows, err := d.Query("PRAGMA table_info(card_events)")
	if err != nil {
		t.Fatalf("PRAGMA table_info(card_events): %v", err)
	}
	defer rows.Close()
	for rows.Next() {
		var cid, notNull, pk int
		var name, ctype string
		var dflt sql.NullString
		_ = rows.Scan(&cid, &name, &ctype, &notNull, &dflt, &pk)
		cols[name] = struct{}{}
	}

	for _, want := range []string{"id", "card_id", "actor", "event_type", "from_value", "to_value", "created_at"} {
		if _, ok := cols[want]; !ok {
			t.Errorf("expected card_events.%s column after migrate", want)
		}
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
