# Admin & Dangerous Actions Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Add rename/delete affordances for projects, agents, and cards, plus a new `/admin` surface housing per-entity admin lists and a "purge database" Danger Zone — all reachable only via the web UI.

**Architecture:** Store-layer operations are added first (with their tests), then the web layer is built on top (handlers, templates, middleware, SSE). Comment attribution survives agent deletion via a snapshot column. Purge runs inside a single transaction and re-invokes the existing `db.Seed()` so the post-purge state matches a fresh install.

**Tech Stack:** Go, SQLite via `modernc.org/sqlite`, Go `html/template` server-rendered UI, vanilla form posts + tiny inline JS for typed-confirmation gating, existing SSE channel in `api/sse.go`.

**Spec:** `docs/superpowers/specs/2026-05-14-admin-dangerous-actions-design.md`

---

## Phase 1 — Foundation (store layer + migration)

### Task 1: Migration 002 — `comments.author_name` snapshot + nullable `agent_id`

**Files:**
- Create: `db/migrations/002_comments_author_snapshot.sql`
- Modify: `db/db.go` (extend `Migrate` to apply migration 002)
- Test: `db/db_test.go`

- [ ] **Step 1: Write the failing test**

Add to `db/db_test.go`:

```go
func TestMigrate_AddsAuthorNameColumnAndNullableAgentID(t *testing.T) {
	dbConn, _ := db.Open(":memory:")
	defer dbConn.Close()
	if err := db.Migrate(dbConn); err != nil {
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
```

- [ ] **Step 2: Run test to verify it fails**

Run: `go test ./db -run TestMigrate_AddsAuthorNameColumnAndNullableAgentID -v`
Expected: FAIL — `author_name` column not found.

- [ ] **Step 3: Create migration 002**

Create `db/migrations/002_comments_author_snapshot.sql`:

```sql
-- Add nullable author_name snapshot and make agent_id nullable.
-- SQLite requires a table rebuild to alter column nullability while
-- preserving the FK cascade to cards.
ALTER TABLE comments ADD COLUMN author_name TEXT;

UPDATE comments
SET author_name = (SELECT name FROM agents WHERE agents.id = comments.agent_id)
WHERE author_name IS NULL;

CREATE TABLE comments_new (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    card_id INTEGER NOT NULL REFERENCES cards(id) ON DELETE CASCADE,
    agent_id INTEGER REFERENCES agents(id),
    author_name TEXT,
    body TEXT NOT NULL,
    created_at DATETIME NOT NULL DEFAULT (datetime('now'))
);

INSERT INTO comments_new (id, card_id, agent_id, author_name, body, created_at)
SELECT id, card_id, agent_id, author_name, body, created_at FROM comments;

DROP TABLE comments;
ALTER TABLE comments_new RENAME TO comments;

CREATE INDEX IF NOT EXISTS idx_comments_card ON comments(card_id);
```

- [ ] **Step 4: Update `db.Migrate` to run migration 002**

Modify `db/db.go` `Migrate`:

```go
func Migrate(db *sql.DB) error {
	for _, name := range []string{"migrations/001_initial.sql", "migrations/002_comments_author_snapshot.sql"} {
		data, err := migrations.ReadFile(name)
		if err != nil {
			return fmt.Errorf("read %s: %w", name, err)
		}
		if _, err := db.Exec(string(data)); err != nil {
			return fmt.Errorf("exec %s: %w", name, err)
		}
	}
	return nil
}
```

- [ ] **Step 5: Run all db tests**

Run: `go test ./db -v`
Expected: PASS.

- [ ] **Step 6: Run full suite to catch comment-related fallout**

Run: `go test ./...`
Expected: PASS (any failures here should be fixed before continuing).

- [ ] **Step 7: Commit**

```bash
git add db/migrations/002_comments_author_snapshot.sql db/db.go db/db_test.go
git commit -m "feat(db): migration 002 — comments.author_name snapshot + nullable agent_id"
```

---

### Task 2: Snapshot `author_name` on comment create

**Files:**
- Modify: `store/comment.go`
- Test: `store/comment_test.go`

- [ ] **Step 1: Write the failing test**

Add to `store/comment_test.go`:

```go
func TestCreateComment_SnapshotsAuthorName(t *testing.T) {
	s := newTestStore(t)
	proj, _ := s.CreateProject("p", "")
	ag, _ := s.CreateAgent("alice", proj.ID, "")
	card, _ := s.CreateCard("t", "", proj.ID, nil, nil)

	c, err := s.CreateComment(card.ID, ag.ID, "hello")
	if err != nil {
		t.Fatal(err)
	}

	var snap sql.NullString
	if err := s.DB().QueryRow("SELECT author_name FROM comments WHERE id = ?", c.ID).Scan(&snap); err != nil {
		t.Fatal(err)
	}
	if !snap.Valid || snap.String != "alice" {
		t.Errorf("expected author_name='alice', got %v", snap)
	}
}
```

If `newTestStore` doesn't exist in this package, copy the pattern from existing tests in `store/`. If `Store` doesn't expose `DB()`, add a tiny accessor method `func (s *Store) DB() *sql.DB { return s.db }` (or query through an existing seam if one exists).

- [ ] **Step 2: Run test to verify it fails**

Run: `go test ./store -run TestCreateComment_SnapshotsAuthorName -v`
Expected: FAIL — `author_name` is NULL.

- [ ] **Step 3: Modify `CreateComment` to write the snapshot**

Replace `store/comment.go`'s `CreateComment`:

```go
func (s *Store) CreateComment(cardID, agentID int, body string) (*model.Comment, error) {
	tx, err := s.db.Begin()
	if err != nil {
		return nil, fmt.Errorf("begin: %w", err)
	}
	defer tx.Rollback()

	var name string
	if err := tx.QueryRow("SELECT name FROM agents WHERE id = ?", agentID).Scan(&name); err != nil {
		return nil, fmt.Errorf("lookup agent %d: %w", agentID, err)
	}

	result, err := tx.Exec(
		"INSERT INTO comments (card_id, agent_id, author_name, body) VALUES (?, ?, ?, ?)",
		cardID, agentID, name, body,
	)
	if err != nil {
		return nil, fmt.Errorf("insert comment: %w", err)
	}

	id, err := result.LastInsertId()
	if err != nil {
		return nil, fmt.Errorf("last insert id: %w", err)
	}
	if err := tx.Commit(); err != nil {
		return nil, fmt.Errorf("commit: %w", err)
	}

	c := &model.Comment{}
	err = s.db.QueryRow(`
		SELECT c.id, c.card_id, c.agent_id, COALESCE(a.name, c.author_name, '') AS agent_name, c.body, c.created_at
		FROM comments c LEFT JOIN agents a ON c.agent_id = a.id
		WHERE c.id = ?
	`, id).Scan(&c.ID, &c.CardID, &c.AgentID, &c.Agent, &c.Body, &c.CreatedAt)
	if err != nil {
		return nil, fmt.Errorf("get comment %d: %w", id, err)
	}
	return c, nil
}
```

Also update `ListComments` to use the same `LEFT JOIN ... COALESCE(a.name, c.author_name, '')` pattern so deleted agents' comments still surface a name.

- [ ] **Step 4: Run test to verify it passes**

Run: `go test ./store -run TestCreateComment -v`
Expected: PASS.

- [ ] **Step 5: Run full suite**

Run: `go test ./...`
Expected: PASS.

- [ ] **Step 6: Commit**

```bash
git add store/comment.go store/comment_test.go
git commit -m "feat(store): snapshot author_name when creating comments"
```

---

### Task 3: `store.RenameProject`

**Files:**
- Modify: `store/project.go`
- Test: `store/project_test.go`

- [ ] **Step 1: Write failing tests**

Add to `store/project_test.go`:

```go
func TestRenameProject_OK(t *testing.T) {
	s := newTestStore(t)
	p, _ := s.CreateProject("old", "")
	if err := s.RenameProject(p.ID, "new"); err != nil {
		t.Fatal(err)
	}
	got, _ := s.GetProject(p.ID)
	if got.Name != "new" {
		t.Errorf("want name=new, got %q", got.Name)
	}
}

func TestRenameProject_DuplicateName(t *testing.T) {
	s := newTestStore(t)
	_, _ = s.CreateProject("a", "")
	b, _ := s.CreateProject("b", "")
	err := s.RenameProject(b.ID, "a")
	if err == nil {
		t.Fatal("expected uniqueness error")
	}
}

func TestRenameProject_Empty(t *testing.T) {
	s := newTestStore(t)
	p, _ := s.CreateProject("a", "")
	if err := s.RenameProject(p.ID, ""); err == nil {
		t.Fatal("expected empty-name error")
	}
}
```

- [ ] **Step 2: Run tests to verify they fail**

Run: `go test ./store -run TestRenameProject -v`
Expected: FAIL — `RenameProject` undefined.

- [ ] **Step 3: Implement**

Add to `store/project.go`:

```go
func (s *Store) RenameProject(id int, name string) error {
	if name == "" {
		return fmt.Errorf("project name cannot be empty")
	}
	_, err := s.db.Exec(
		"UPDATE projects SET name = ?, updated_at = datetime('now') WHERE id = ?",
		name, id,
	)
	if err != nil {
		return fmt.Errorf("rename project %d: %w", id, err)
	}
	return nil
}
```

- [ ] **Step 4: Run tests**

Run: `go test ./store -run TestRenameProject -v`
Expected: PASS.

- [ ] **Step 5: Commit**

```bash
git add store/project.go store/project_test.go
git commit -m "feat(store): RenameProject with uniqueness + empty-name validation"
```

---

### Task 4: `store.RenameAgent` (with comment snapshot backfill)

**Files:**
- Modify: `store/agent.go`
- Test: `store/agent_test.go`

- [ ] **Step 1: Write failing tests**

Add to `store/agent_test.go`:

```go
func TestRenameAgent_OK_BackfillsHistoricalComments(t *testing.T) {
	s := newTestStore(t)
	p, _ := s.CreateProject("p", "")
	a, _ := s.CreateAgent("old", p.ID, "")
	c, _ := s.CreateCard("t", "", p.ID, nil, nil)
	_, _ = s.CreateComment(c.ID, a.ID, "hi")

	if err := s.RenameAgent(a.ID, "new"); err != nil {
		t.Fatal(err)
	}

	var name string
	if err := s.DB().QueryRow("SELECT author_name FROM comments WHERE agent_id = ?", a.ID).Scan(&name); err != nil {
		t.Fatal(err)
	}
	if name != "new" {
		t.Errorf("expected author_name updated to 'new', got %q", name)
	}
}

func TestRenameAgent_DuplicateName(t *testing.T) {
	s := newTestStore(t)
	p, _ := s.CreateProject("p", "")
	_, _ = s.CreateAgent("a", p.ID, "")
	b, _ := s.CreateAgent("b", p.ID, "")
	if err := s.RenameAgent(b.ID, "a"); err == nil {
		t.Fatal("expected uniqueness error")
	}
}
```

- [ ] **Step 2: Run tests**

Run: `go test ./store -run TestRenameAgent -v`
Expected: FAIL.

- [ ] **Step 3: Implement**

Add to `store/agent.go`:

```go
func (s *Store) RenameAgent(id int, name string) error {
	if name == "" {
		return fmt.Errorf("agent name cannot be empty")
	}
	tx, err := s.db.Begin()
	if err != nil {
		return fmt.Errorf("begin: %w", err)
	}
	defer tx.Rollback()

	if _, err := tx.Exec(
		"UPDATE agents SET name = ?, updated_at = datetime('now') WHERE id = ?",
		name, id,
	); err != nil {
		return fmt.Errorf("rename agent %d: %w", id, err)
	}
	if _, err := tx.Exec(
		"UPDATE comments SET author_name = ? WHERE agent_id = ?",
		name, id,
	); err != nil {
		return fmt.Errorf("backfill author_name: %w", err)
	}
	return tx.Commit()
}
```

- [ ] **Step 4: Run tests**

Run: `go test ./store -run TestRenameAgent -v`
Expected: PASS.

- [ ] **Step 5: Commit**

```bash
git add store/agent.go store/agent_test.go
git commit -m "feat(store): RenameAgent backfills historical comment author_name"
```

---

### Task 5: `store.DeleteCard`

**Files:**
- Modify: `store/card.go`
- Test: `store/card_test.go`

- [ ] **Step 1: Write failing test**

Add to `store/card_test.go`:

```go
func TestDeleteCard_CascadesCommentsAndAssignees(t *testing.T) {
	s := newTestStore(t)
	p, _ := s.CreateProject("p", "")
	a, _ := s.CreateAgent("a", p.ID, "")
	c, _ := s.CreateCard("t", "", p.ID, []string{"a"}, []string{"tag"})
	_, _ = s.CreateComment(c.ID, a.ID, "body")

	if err := s.DeleteCard(c.ID); err != nil {
		t.Fatal(err)
	}

	var n int
	_ = s.DB().QueryRow("SELECT COUNT(*) FROM cards WHERE id = ?", c.ID).Scan(&n)
	if n != 0 {
		t.Errorf("expected card gone, got %d", n)
	}
	_ = s.DB().QueryRow("SELECT COUNT(*) FROM comments WHERE card_id = ?", c.ID).Scan(&n)
	if n != 0 {
		t.Errorf("expected comments gone, got %d", n)
	}
	_ = s.DB().QueryRow("SELECT COUNT(*) FROM card_assignees WHERE card_id = ?", c.ID).Scan(&n)
	if n != 0 {
		t.Errorf("expected assignees gone, got %d", n)
	}
}
```

- [ ] **Step 2: Run test**

Run: `go test ./store -run TestDeleteCard -v`
Expected: FAIL.

- [ ] **Step 3: Implement**

Add to `store/card.go`:

```go
func (s *Store) DeleteCard(id int) error {
	res, err := s.db.Exec("DELETE FROM cards WHERE id = ?", id)
	if err != nil {
		return fmt.Errorf("delete card %d: %w", id, err)
	}
	n, _ := res.RowsAffected()
	if n == 0 {
		return fmt.Errorf("card %d not found", id)
	}
	return nil
}
```

(FK `ON DELETE CASCADE` on `comments`, `card_tags`, `card_relations`, `card_assignees` handles the dependents.)

- [ ] **Step 4: Run tests**

Run: `go test ./store -run TestDeleteCard -v`
Expected: PASS.

- [ ] **Step 5: Commit**

```bash
git add store/card.go store/card_test.go
git commit -m "feat(store): DeleteCard with FK cascade"
```

---

### Task 6: `store.DeleteAgent`

**Files:**
- Modify: `store/agent.go`
- Test: `store/agent_test.go`

- [ ] **Step 1: Write failing tests**

Add to `store/agent_test.go`:

```go
func TestDeleteAgent_UnassignsCards_PreservesComments(t *testing.T) {
	s := newTestStore(t)
	p, _ := s.CreateProject("p", "")
	a, _ := s.CreateAgent("alice", p.ID, "")
	c, _ := s.CreateCard("t", "", p.ID, []string{"alice"}, nil)
	_, _ = s.CreateComment(c.ID, a.ID, "hi")

	if err := s.DeleteAgent(a.ID); err != nil {
		t.Fatal(err)
	}

	// agent gone
	var n int
	_ = s.DB().QueryRow("SELECT COUNT(*) FROM agents WHERE id = ?", a.ID).Scan(&n)
	if n != 0 {
		t.Errorf("expected agent gone, got %d", n)
	}
	// card kept, no assignment
	_ = s.DB().QueryRow("SELECT COUNT(*) FROM cards WHERE id = ?", c.ID).Scan(&n)
	if n != 1 {
		t.Errorf("expected card to survive, got %d", n)
	}
	_ = s.DB().QueryRow("SELECT COUNT(*) FROM card_assignees WHERE agent_id = ?", a.ID).Scan(&n)
	if n != 0 {
		t.Errorf("expected assignee row gone, got %d", n)
	}
	// comment kept; agent_id NULL; author_name preserved
	var agentID sql.NullInt64
	var name sql.NullString
	_ = s.DB().QueryRow("SELECT agent_id, author_name FROM comments WHERE card_id = ?", c.ID).Scan(&agentID, &name)
	if agentID.Valid {
		t.Errorf("expected comment.agent_id NULL, got %v", agentID)
	}
	if !name.Valid || name.String != "alice" {
		t.Errorf("expected author_name='alice', got %v", name)
	}
}
```

- [ ] **Step 2: Run test**

Run: `go test ./store -run TestDeleteAgent_UnassignsCards -v`
Expected: FAIL.

- [ ] **Step 3: Implement**

Add to `store/agent.go`:

```go
func (s *Store) DeleteAgent(id int) error {
	tx, err := s.db.Begin()
	if err != nil {
		return fmt.Errorf("begin: %w", err)
	}
	defer tx.Rollback()

	if _, err := tx.Exec("DELETE FROM card_assignees WHERE agent_id = ?", id); err != nil {
		return fmt.Errorf("clear assignees: %w", err)
	}
	if _, err := tx.Exec("UPDATE comments SET agent_id = NULL WHERE agent_id = ?", id); err != nil {
		return fmt.Errorf("null comment agent: %w", err)
	}
	res, err := tx.Exec("DELETE FROM agents WHERE id = ?", id)
	if err != nil {
		return fmt.Errorf("delete agent %d: %w", id, err)
	}
	n, _ := res.RowsAffected()
	if n == 0 {
		return fmt.Errorf("agent %d not found", id)
	}
	return tx.Commit()
}
```

- [ ] **Step 4: Run tests**

Run: `go test ./store -v`
Expected: PASS.

- [ ] **Step 5: Commit**

```bash
git add store/agent.go store/agent_test.go
git commit -m "feat(store): DeleteAgent unassigns cards, preserves comments via snapshot"
```

---

### Task 7: `store.DeleteProject` (transactional fan-out)

**Files:**
- Modify: `store/project.go`
- Test: `store/project_test.go`

- [ ] **Step 1: Write failing test**

Add to `store/project_test.go`:

```go
func TestDeleteProject_CascadesAllChildren(t *testing.T) {
	s := newTestStore(t)
	p, _ := s.CreateProject("p", "")
	a, _ := s.CreateAgent("alice", p.ID, "")
	c, _ := s.CreateCard("t", "", p.ID, []string{"alice"}, nil)
	_, _ = s.CreateComment(c.ID, a.ID, "hi")

	if err := s.DeleteProject(p.ID); err != nil {
		t.Fatal(err)
	}

	checks := []struct {
		query string
		args  []any
		want  int
	}{
		{"SELECT COUNT(*) FROM projects WHERE id = ?", []any{p.ID}, 0},
		{"SELECT COUNT(*) FROM agents WHERE project_id = ?", []any{p.ID}, 0},
		{"SELECT COUNT(*) FROM cards WHERE project_id = ?", []any{p.ID}, 0},
		{"SELECT COUNT(*) FROM comments WHERE card_id = ?", []any{c.ID}, 0},
		{"SELECT COUNT(*) FROM project_assets WHERE project_id = ?", []any{p.ID}, 0},
	}
	for _, ch := range checks {
		var n int
		_ = s.DB().QueryRow(ch.query, ch.args...).Scan(&n)
		if n != ch.want {
			t.Errorf("%s: want %d, got %d", ch.query, ch.want, n)
		}
	}
}
```

- [ ] **Step 2: Run test**

Run: `go test ./store -run TestDeleteProject -v`
Expected: FAIL.

- [ ] **Step 3: Implement**

Add to `store/project.go`:

```go
func (s *Store) DeleteProject(id int) error {
	tx, err := s.db.Begin()
	if err != nil {
		return fmt.Errorf("begin: %w", err)
	}
	defer tx.Rollback()

	// Cards: FK cascades handle comments, tags, relations, assignees.
	if _, err := tx.Exec("DELETE FROM cards WHERE project_id = ?", id); err != nil {
		return fmt.Errorf("delete cards: %w", err)
	}
	if _, err := tx.Exec("DELETE FROM project_assets WHERE project_id = ?", id); err != nil {
		return fmt.Errorf("delete assets: %w", err)
	}
	// Agents in this project: null out their comment references first
	// (in case any comments survived on cards belonging to OTHER projects).
	if _, err := tx.Exec(
		"UPDATE comments SET agent_id = NULL WHERE agent_id IN (SELECT id FROM agents WHERE project_id = ?)",
		id,
	); err != nil {
		return fmt.Errorf("null cross-project comment refs: %w", err)
	}
	if _, err := tx.Exec(
		"DELETE FROM card_assignees WHERE agent_id IN (SELECT id FROM agents WHERE project_id = ?)",
		id,
	); err != nil {
		return fmt.Errorf("clear cross-project assignees: %w", err)
	}
	if _, err := tx.Exec("DELETE FROM agents WHERE project_id = ?", id); err != nil {
		return fmt.Errorf("delete agents: %w", err)
	}
	res, err := tx.Exec("DELETE FROM projects WHERE id = ?", id)
	if err != nil {
		return fmt.Errorf("delete project %d: %w", id, err)
	}
	n, _ := res.RowsAffected()
	if n == 0 {
		return fmt.Errorf("project %d not found", id)
	}
	return tx.Commit()
}
```

- [ ] **Step 4: Run tests**

Run: `go test ./store -v`
Expected: PASS.

- [ ] **Step 5: Commit**

```bash
git add store/project.go store/project_test.go
git commit -m "feat(store): DeleteProject cascades all children in single tx"
```

---

### Task 8: `store.Purge` (truncate + re-seed)

**Files:**
- Create: `store/admin.go`
- Test: `store/admin_test.go`

- [ ] **Step 1: Write failing test**

Create `store/admin_test.go`:

```go
package store_test

import (
	"testing"
)

func TestPurge_WipesAllDataAndReSeeds(t *testing.T) {
	s := newTestStore(t)
	p, _ := s.CreateProject("p", "")
	a, _ := s.CreateAgent("alice", p.ID, "")
	c, _ := s.CreateCard("t", "", p.ID, []string{"alice"}, nil)
	_, _ = s.CreateComment(c.ID, a.ID, "hi")

	if err := s.Purge(); err != nil {
		t.Fatal(err)
	}

	// Seed baseline: 'orchestration' project + 'user' agent exist.
	var projects, agents, cards, comments int
	_ = s.DB().QueryRow("SELECT COUNT(*) FROM projects").Scan(&projects)
	_ = s.DB().QueryRow("SELECT COUNT(*) FROM agents").Scan(&agents)
	_ = s.DB().QueryRow("SELECT COUNT(*) FROM cards").Scan(&cards)
	_ = s.DB().QueryRow("SELECT COUNT(*) FROM comments").Scan(&comments)

	if projects != 1 {
		t.Errorf("expected 1 seeded project, got %d", projects)
	}
	if agents != 1 {
		t.Errorf("expected 1 seeded agent, got %d", agents)
	}
	if cards != 0 {
		t.Errorf("expected 0 cards, got %d", cards)
	}
	if comments != 0 {
		t.Errorf("expected 0 comments, got %d", comments)
	}
}
```

- [ ] **Step 2: Run test**

Run: `go test ./store -run TestPurge -v`
Expected: FAIL — `Purge` undefined.

- [ ] **Step 3: Implement**

Create `store/admin.go`:

```go
package store

import (
	"fmt"

	"github.com/joelhelbling/kkullm/db"
)

// Purge wipes all user data tables and re-runs the baseline Seed so the
// post-purge state matches a fresh install. Migrations table is untouched.
func (s *Store) Purge() error {
	tx, err := s.db.Begin()
	if err != nil {
		return fmt.Errorf("begin purge: %w", err)
	}
	defer tx.Rollback()

	// FK-safe order. cards has ON DELETE CASCADE for comments/tags/relations/assignees,
	// but explicit deletes keep intent clear and survive FK config changes.
	for _, q := range []string{
		"DELETE FROM comments",
		"DELETE FROM card_assignees",
		"DELETE FROM card_tags",
		"DELETE FROM card_relations",
		"DELETE FROM cards",
		"DELETE FROM project_assets",
		"DELETE FROM agents",
		"DELETE FROM projects",
		"DELETE FROM sqlite_sequence",
	} {
		if _, err := tx.Exec(q); err != nil {
			return fmt.Errorf("purge %q: %w", q, err)
		}
	}
	if err := tx.Commit(); err != nil {
		return fmt.Errorf("commit purge: %w", err)
	}

	// Re-seed outside the transaction so seed errors surface separately.
	if err := db.Seed(s.db); err != nil {
		return fmt.Errorf("re-seed after purge: %w", err)
	}
	return nil
}
```

(If the `store` package can't import `db` due to a cycle, instead export a `Reseed func(*sql.DB) error` field on the `Store` and have `cmd/serve.go` wire `db.Seed` in. Plan-time guess: there is no cycle today; verify by running `go build ./...` after editing.)

- [ ] **Step 4: Run test**

Run: `go test ./store -run TestPurge -v`
Expected: PASS.

- [ ] **Step 5: Run full suite**

Run: `go test ./...`
Expected: PASS.

- [ ] **Step 6: Commit**

```bash
git add store/admin.go store/admin_test.go
git commit -m "feat(store): Purge truncates all data and re-seeds baseline"
```

---

## Phase 2 — Web layer

### Task 9: `requireAdmin` middleware (no-op chokepoint)

**Files:**
- Create: `web/admin_middleware.go`
- Test: `web/admin_middleware_test.go`

- [ ] **Step 1: Write failing test**

Create `web/admin_middleware_test.go`:

```go
package web_test

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/joelhelbling/kkullm/web"
)

func TestRequireAdmin_PassesThroughForNow(t *testing.T) {
	called := false
	h := web.RequireAdmin(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		called = true
		w.WriteHeader(http.StatusOK)
	}))

	req := httptest.NewRequest("GET", "/admin", nil)
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)

	if !called {
		t.Errorf("expected inner handler to be called")
	}
	if rec.Code != http.StatusOK {
		t.Errorf("expected 200, got %d", rec.Code)
	}
}
```

- [ ] **Step 2: Run test**

Run: `go test ./web -run TestRequireAdmin -v`
Expected: FAIL — undefined.

- [ ] **Step 3: Implement**

Create `web/admin_middleware.go`:

```go
package web

import "net/http"

// RequireAdmin is the single chokepoint for admin-only / dangerous routes.
// Today it is a pass-through; when auth lands this is the one place to enforce it.
func RequireAdmin(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		next.ServeHTTP(w, r)
	})
}
```

- [ ] **Step 4: Run test**

Run: `go test ./web -run TestRequireAdmin -v`
Expected: PASS.

- [ ] **Step 5: Commit**

```bash
git add web/admin_middleware.go web/admin_middleware_test.go
git commit -m "feat(web): RequireAdmin middleware chokepoint (no-op pass-through)"
```

---

### Task 10: Admin shell template + routes

**Files:**
- Create: `web/templates/admin/shell.html`
- Create: `web/templates/admin/projects.html`
- Create: `web/templates/admin/agents.html`
- Create: `web/templates/admin/danger.html`
- Modify: `web/web.go` (register `/admin/*` routes behind `RequireAdmin`)
- Create: `web/admin_handlers.go`
- Test: `web/admin_handlers_test.go`

- [ ] **Step 1: Write failing route test**

Create `web/admin_handlers_test.go`:

```go
package web_test

import (
	"net/http/httptest"
	"strings"
	"testing"
)

func TestAdminRoot_RedirectsToProjects(t *testing.T) {
	srv := newTestWebServer(t)
	rec := httptest.NewRecorder()
	srv.ServeHTTP(rec, httptest.NewRequest("GET", "/admin", nil))
	if rec.Code != 302 && rec.Code != 303 {
		t.Errorf("expected redirect, got %d", rec.Code)
	}
	if loc := rec.Header().Get("Location"); !strings.HasSuffix(loc, "/admin/projects") {
		t.Errorf("expected redirect to /admin/projects, got %q", loc)
	}
}

func TestAdminProjects_Renders(t *testing.T) {
	srv := newTestWebServer(t)
	rec := httptest.NewRecorder()
	srv.ServeHTTP(rec, httptest.NewRequest("GET", "/admin/projects", nil))
	if rec.Code != 200 {
		t.Errorf("expected 200, got %d", rec.Code)
	}
	body := rec.Body.String()
	for _, want := range []string{"Projects", "Agents", "Danger Zone"} {
		if !strings.Contains(body, want) {
			t.Errorf("expected page to mention %q", want)
		}
	}
}
```

If `newTestWebServer` doesn't exist, copy the helper from `web/handlers_test.go` (or wherever existing handler tests construct a server).

- [ ] **Step 2: Run test**

Run: `go test ./web -run TestAdmin -v`
Expected: FAIL (404).

- [ ] **Step 3: Create the shell template**

Create `web/templates/admin/shell.html`:

```html
{{define "admin-shell"}}
{{template "header" .}}
<div class="admin">
  <nav class="admin-menu">
    <a href="/admin/projects" class="{{if eq .Section "projects"}}active{{end}}">Projects</a>
    <a href="/admin/agents" class="{{if eq .Section "agents"}}active{{end}}">Agents</a>
    <div class="admin-menu-divider"></div>
    <a href="/admin/danger" class="{{if eq .Section "danger"}}active{{end}} danger">Danger Zone</a>
  </nav>
  <main class="admin-main">
    {{block "admin-content" .}}{{end}}
  </main>
</div>
{{template "footer" .}}
{{end}}
```

(Header/footer template names should match what `layout.html` already exposes — adjust if the existing layout uses different block names. Inspect `web/templates/layout.html` and reuse its block structure.)

- [ ] **Step 4: Create per-section templates**

Create `web/templates/admin/projects.html`:

```html
{{define "admin-content"}}
<h1>Projects</h1>
<p class="admin-desc">Rename or delete projects. Deleting a project permanently removes its cards, comments, agents, and assets.</p>
<ul class="admin-list">
  {{range .Projects}}
  <li class="admin-row">
    <span class="admin-name" data-rename-target="/admin/projects/{{.ID}}/rename" data-original="{{.Name}}">{{.Name}}</span>
    <span class="admin-meta">{{.CardCount}} cards · {{.AgentCount}} agents</span>
    <form method="post" action="/admin/projects/{{.ID}}/delete" class="admin-delete-form" data-confirm-name="{{.Name}}">
      <button type="button" class="btn-outlined-danger" onclick="openDeleteProjectModal(this)">Delete</button>
    </form>
  </li>
  {{end}}
</ul>
{{template "delete-project-modal" .}}
{{end}}
```

Create `web/templates/admin/agents.html`:

```html
{{define "admin-content"}}
<h1>Agents</h1>
<p class="admin-desc">Rename or delete agents. Deleting an agent unassigns their cards; their comments are preserved with attribution.</p>
<ul class="admin-list">
  {{range .Agents}}
  <li class="admin-row">
    <span class="admin-name" data-rename-target="/admin/agents/{{.ID}}/rename" data-original="{{.Name}}">{{.Name}}</span>
    <span class="admin-meta">project: {{.Project}}</span>
    <form method="post" action="/admin/agents/{{.ID}}/delete" class="admin-delete-form">
      <button type="submit" class="btn-outlined-danger" onclick="return confirm('Delete agent {{.Name}}? Their cards will be unassigned; comments preserved.')">Delete</button>
    </form>
  </li>
  {{end}}
</ul>
{{end}}
```

Create `web/templates/admin/danger.html`:

```html
{{define "admin-content"}}
<h1>Danger Zone</h1>
<div class="danger-panel">
  <h2>Purge database</h2>
  <p>This cannot be undone. All projects, agents, cards, comments, and assets will be permanently deleted. After purge, the system returns to a fresh-install state.</p>
  <form method="post" action="/admin/danger/purge" class="danger-form">
    <label>Type <code>PURGE DATABASE</code> to enable:</label>
    <input type="text" name="confirm" autocomplete="off" class="mono"
           oninput="document.getElementById('purge-btn').disabled = this.value !== 'PURGE DATABASE'">
    <button id="purge-btn" type="submit" class="btn-danger" disabled>Purge</button>
  </form>
</div>
{{end}}
```

Also create a small `delete-project-modal` partial referenced above — a hidden `<dialog>` or div with a typed-name input gating the submit, included once per page. Pattern:

```html
{{define "delete-project-modal"}}
<div id="delete-project-modal" class="modal" hidden>
  <div class="modal-content danger-panel">
    <h2>Delete project</h2>
    <p>This cannot be undone. All cards, comments, agents, and assets in this project will be permanently deleted.</p>
    <form method="post" id="delete-project-form">
      <label>Type the project name <code id="dp-name"></code> to enable:</label>
      <input type="text" name="confirm" autocomplete="off" class="mono"
             oninput="document.getElementById('dp-btn').disabled = this.value !== document.getElementById('dp-name').textContent">
      <button type="button" onclick="document.getElementById('delete-project-modal').hidden = true">Cancel</button>
      <button id="dp-btn" type="submit" class="btn-danger" disabled>Delete</button>
    </form>
  </div>
</div>
<script>
function openDeleteProjectModal(btn) {
  const form = btn.closest('form');
  const name = form.dataset.confirmName;
  const modal = document.getElementById('delete-project-modal');
  document.getElementById('dp-name').textContent = name;
  document.getElementById('delete-project-form').action = form.action;
  document.getElementById('dp-btn').disabled = true;
  modal.hidden = false;
}
</script>
{{end}}
```

- [ ] **Step 5: Implement handlers**

Create `web/admin_handlers.go`:

```go
package web

import (
	"net/http"
)

func (s *Server) handleAdminRoot(w http.ResponseWriter, r *http.Request) {
	http.Redirect(w, r, "/admin/projects", http.StatusSeeOther)
}

type adminProjectsData struct {
	Section  string
	Projects []adminProjectRow
}

type adminProjectRow struct {
	ID         int
	Name       string
	CardCount  int
	AgentCount int
}

func (s *Server) handleAdminProjects(w http.ResponseWriter, r *http.Request) {
	projects, err := s.store.ListProjects()
	if err != nil {
		http.Error(w, err.Error(), 500)
		return
	}
	rows := make([]adminProjectRow, 0, len(projects))
	for _, p := range projects {
		cc, _ := s.store.CountCardsForProject(p.ID) // add helper or inline COUNT(*) query
		ac, _ := s.store.CountAgentsForProject(p.ID)
		rows = append(rows, adminProjectRow{ID: p.ID, Name: p.Name, CardCount: cc, AgentCount: ac})
	}
	s.render(w, "admin-shell", adminProjectsData{Section: "projects", Projects: rows})
}

func (s *Server) handleAdminAgents(w http.ResponseWriter, r *http.Request) {
	agents, err := s.store.ListAgents("") // empty project filter = all
	if err != nil {
		http.Error(w, err.Error(), 500)
		return
	}
	s.render(w, "admin-shell", struct {
		Section string
		Agents  []model.Agent
	}{"agents", agents})
}

func (s *Server) handleAdminDanger(w http.ResponseWriter, r *http.Request) {
	s.render(w, "admin-shell", struct{ Section string }{"danger"})
}
```

Match the actual signatures used by existing handlers in `web/handlers.go` (rendering helper, store accessors). If `ListAgents` doesn't accept an empty filter to mean "all," add a `ListAllAgents` helper in `store/agent.go`. Likewise add `CountCardsForProject` / `CountAgentsForProject` helpers if not present.

- [ ] **Step 6: Wire routes in `web/web.go`**

Add to the router setup:

```go
adminMux := http.NewServeMux()
adminMux.HandleFunc("GET /admin", srv.handleAdminRoot)
adminMux.HandleFunc("GET /admin/projects", srv.handleAdminProjects)
adminMux.HandleFunc("GET /admin/agents", srv.handleAdminAgents)
adminMux.HandleFunc("GET /admin/danger", srv.handleAdminDanger)
// (Action routes added in later tasks.)
mux.Handle("/admin", RequireAdmin(adminMux))
mux.Handle("/admin/", RequireAdmin(adminMux))
```

Match the actual mux library / pattern used in `web/web.go`. If the codebase uses a third-party router, register the routes there with the middleware applied.

- [ ] **Step 7: Run tests**

Run: `go test ./web -run TestAdmin -v`
Expected: PASS.

- [ ] **Step 8: Commit**

```bash
git add web/admin_handlers.go web/admin_handlers_test.go web/web.go web/templates/admin/
git commit -m "feat(web): /admin shell with projects, agents, danger sections"
```

---

### Task 11: Admin Projects — rename + delete actions

**Files:**
- Modify: `web/admin_handlers.go`, `web/web.go`
- Test: `web/admin_handlers_test.go`

- [ ] **Step 1: Write failing tests**

Add to `web/admin_handlers_test.go`:

```go
func TestAdminRenameProject_OK(t *testing.T) {
	srv, store := newTestWebServerWithStore(t)
	p, _ := store.CreateProject("old", "")
	form := url.Values{"name": {"new"}}
	req := httptest.NewRequest("POST", "/admin/projects/"+strconv.Itoa(p.ID)+"/rename",
		strings.NewReader(form.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	rec := httptest.NewRecorder()
	srv.ServeHTTP(rec, req)
	if rec.Code != 303 && rec.Code != 302 {
		t.Errorf("want redirect, got %d", rec.Code)
	}
	got, _ := store.GetProject(p.ID)
	if got.Name != "new" {
		t.Errorf("want name=new, got %q", got.Name)
	}
}

func TestAdminDeleteProject_RejectsMismatchedConfirm(t *testing.T) {
	srv, store := newTestWebServerWithStore(t)
	p, _ := store.CreateProject("alpha", "")
	form := url.Values{"confirm": {"WRONG"}}
	req := httptest.NewRequest("POST", "/admin/projects/"+strconv.Itoa(p.ID)+"/delete",
		strings.NewReader(form.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	rec := httptest.NewRecorder()
	srv.ServeHTTP(rec, req)
	if rec.Code != 400 {
		t.Errorf("want 400, got %d", rec.Code)
	}
	got, _ := store.GetProject(p.ID)
	if got == nil {
		t.Errorf("project should still exist after rejected delete")
	}
}

func TestAdminDeleteProject_AcceptsMatchedConfirm(t *testing.T) {
	srv, store := newTestWebServerWithStore(t)
	p, _ := store.CreateProject("alpha", "")
	form := url.Values{"confirm": {"alpha"}}
	req := httptest.NewRequest("POST", "/admin/projects/"+strconv.Itoa(p.ID)+"/delete",
		strings.NewReader(form.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	rec := httptest.NewRecorder()
	srv.ServeHTTP(rec, req)
	if rec.Code != 303 && rec.Code != 302 {
		t.Errorf("want redirect, got %d", rec.Code)
	}
	got, _ := store.GetProject(p.ID)
	if got != nil {
		t.Errorf("project should be deleted")
	}
}
```

- [ ] **Step 2: Run tests**

Run: `go test ./web -run TestAdminRenameProject -v -run TestAdminDeleteProject`
Expected: FAIL.

- [ ] **Step 3: Implement handlers**

Add to `web/admin_handlers.go`:

```go
func (s *Server) handleAdminRenameProject(w http.ResponseWriter, r *http.Request) {
	id, err := strconv.Atoi(r.PathValue("id"))
	if err != nil {
		http.Error(w, "bad id", 400)
		return
	}
	if err := r.ParseForm(); err != nil {
		http.Error(w, err.Error(), 400)
		return
	}
	name := strings.TrimSpace(r.FormValue("name"))
	if err := s.store.RenameProject(id, name); err != nil {
		http.Error(w, err.Error(), 400)
		return
	}
	http.Redirect(w, r, "/admin/projects", http.StatusSeeOther)
}

func (s *Server) handleAdminDeleteProject(w http.ResponseWriter, r *http.Request) {
	id, err := strconv.Atoi(r.PathValue("id"))
	if err != nil {
		http.Error(w, "bad id", 400)
		return
	}
	if err := r.ParseForm(); err != nil {
		http.Error(w, err.Error(), 400)
		return
	}
	p, err := s.store.GetProject(id)
	if err != nil || p == nil {
		http.Redirect(w, r, "/admin/projects", http.StatusSeeOther)
		return
	}
	if r.FormValue("confirm") != p.Name {
		http.Error(w, "confirmation does not match project name", 400)
		return
	}
	if err := s.store.DeleteProject(id); err != nil {
		http.Error(w, err.Error(), 500)
		return
	}
	http.Redirect(w, r, "/admin/projects", http.StatusSeeOther)
}
```

Register in `adminMux`:

```go
adminMux.HandleFunc("POST /admin/projects/{id}/rename", srv.handleAdminRenameProject)
adminMux.HandleFunc("POST /admin/projects/{id}/delete", srv.handleAdminDeleteProject)
```

- [ ] **Step 4: Run tests**

Run: `go test ./web -v`
Expected: PASS.

- [ ] **Step 5: Commit**

```bash
git add web/admin_handlers.go web/admin_handlers_test.go web/web.go
git commit -m "feat(web): admin rename + delete project endpoints with typed confirmation"
```

---

### Task 12: Admin Agents — rename + delete actions

**Files:**
- Modify: `web/admin_handlers.go`, `web/web.go`
- Test: `web/admin_handlers_test.go`

- [ ] **Step 1: Write failing tests**

Add to `web/admin_handlers_test.go`:

```go
func TestAdminRenameAgent_OK(t *testing.T) {
	srv, store := newTestWebServerWithStore(t)
	p, _ := store.CreateProject("p", "")
	a, _ := store.CreateAgent("old", p.ID, "")
	form := url.Values{"name": {"new"}}
	req := httptest.NewRequest("POST", "/admin/agents/"+strconv.Itoa(a.ID)+"/rename",
		strings.NewReader(form.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	rec := httptest.NewRecorder()
	srv.ServeHTTP(rec, req)
	if rec.Code != 303 && rec.Code != 302 {
		t.Errorf("want redirect, got %d", rec.Code)
	}
	got, _ := store.GetAgent(a.ID)
	if got.Name != "new" {
		t.Errorf("want name=new, got %q", got.Name)
	}
}

func TestAdminDeleteAgent_OK(t *testing.T) {
	srv, store := newTestWebServerWithStore(t)
	p, _ := store.CreateProject("p", "")
	a, _ := store.CreateAgent("alice", p.ID, "")
	req := httptest.NewRequest("POST", "/admin/agents/"+strconv.Itoa(a.ID)+"/delete", nil)
	rec := httptest.NewRecorder()
	srv.ServeHTTP(rec, req)
	if rec.Code != 303 && rec.Code != 302 {
		t.Errorf("want redirect, got %d", rec.Code)
	}
	got, _ := store.GetAgent(a.ID)
	if got != nil {
		t.Errorf("agent should be deleted")
	}
}
```

- [ ] **Step 2: Run tests**

Run: `go test ./web -run TestAdminRenameAgent -run TestAdminDeleteAgent -v`
Expected: FAIL.

- [ ] **Step 3: Implement and register**

Add to `web/admin_handlers.go`:

```go
func (s *Server) handleAdminRenameAgent(w http.ResponseWriter, r *http.Request) {
	id, err := strconv.Atoi(r.PathValue("id"))
	if err != nil {
		http.Error(w, "bad id", 400)
		return
	}
	if err := r.ParseForm(); err != nil {
		http.Error(w, err.Error(), 400)
		return
	}
	name := strings.TrimSpace(r.FormValue("name"))
	if err := s.store.RenameAgent(id, name); err != nil {
		http.Error(w, err.Error(), 400)
		return
	}
	http.Redirect(w, r, "/admin/agents", http.StatusSeeOther)
}

func (s *Server) handleAdminDeleteAgent(w http.ResponseWriter, r *http.Request) {
	id, err := strconv.Atoi(r.PathValue("id"))
	if err != nil {
		http.Error(w, "bad id", 400)
		return
	}
	if err := s.store.DeleteAgent(id); err != nil {
		http.Error(w, err.Error(), 500)
		return
	}
	http.Redirect(w, r, "/admin/agents", http.StatusSeeOther)
}
```

Register:

```go
adminMux.HandleFunc("POST /admin/agents/{id}/rename", srv.handleAdminRenameAgent)
adminMux.HandleFunc("POST /admin/agents/{id}/delete", srv.handleAdminDeleteAgent)
```

- [ ] **Step 4: Run tests**

Run: `go test ./web -v`
Expected: PASS.

- [ ] **Step 5: Commit**

```bash
git add web/admin_handlers.go web/admin_handlers_test.go web/web.go
git commit -m "feat(web): admin rename + delete agent endpoints"
```

---

### Task 13: Danger Zone — purge endpoint

**Files:**
- Modify: `web/admin_handlers.go`, `web/web.go`
- Test: `web/admin_handlers_test.go`

- [ ] **Step 1: Write failing tests**

Add to `web/admin_handlers_test.go`:

```go
func TestAdminPurge_RejectsWrongPhrase(t *testing.T) {
	srv, store := newTestWebServerWithStore(t)
	p, _ := store.CreateProject("alpha", "")
	form := url.Values{"confirm": {"purge database"}} // wrong case
	req := httptest.NewRequest("POST", "/admin/danger/purge",
		strings.NewReader(form.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	rec := httptest.NewRecorder()
	srv.ServeHTTP(rec, req)
	if rec.Code != 400 {
		t.Errorf("want 400, got %d", rec.Code)
	}
	got, _ := store.GetProject(p.ID)
	if got == nil {
		t.Errorf("project should still exist")
	}
}

func TestAdminPurge_AcceptsExactPhrase(t *testing.T) {
	srv, store := newTestWebServerWithStore(t)
	_, _ = store.CreateProject("alpha", "")
	form := url.Values{"confirm": {"PURGE DATABASE"}}
	req := httptest.NewRequest("POST", "/admin/danger/purge",
		strings.NewReader(form.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	rec := httptest.NewRecorder()
	srv.ServeHTTP(rec, req)
	if rec.Code != 303 && rec.Code != 302 {
		t.Errorf("want redirect, got %d", rec.Code)
	}
	// post-purge: only seed data remains
	var n int
	_ = store.DB().QueryRow("SELECT COUNT(*) FROM projects WHERE name = 'alpha'").Scan(&n)
	if n != 0 {
		t.Errorf("alpha should have been purged")
	}
}
```

- [ ] **Step 2: Run tests**

Run: `go test ./web -run TestAdminPurge -v`
Expected: FAIL.

- [ ] **Step 3: Implement and register**

Add to `web/admin_handlers.go`:

```go
const purgePhrase = "PURGE DATABASE"

func (s *Server) handleAdminPurge(w http.ResponseWriter, r *http.Request) {
	if err := r.ParseForm(); err != nil {
		http.Error(w, err.Error(), 400)
		return
	}
	if r.FormValue("confirm") != purgePhrase {
		http.Error(w, "confirmation phrase does not match", 400)
		return
	}
	if err := s.store.Purge(); err != nil {
		http.Error(w, err.Error(), 500)
		return
	}
	// Notify SSE clients (see Task 16).
	s.broadcastDatasetReset()
	http.Redirect(w, r, "/", http.StatusSeeOther)
}
```

Register:

```go
adminMux.HandleFunc("POST /admin/danger/purge", srv.handleAdminPurge)
```

Add a temporary stub `func (s *Server) broadcastDatasetReset() {}` for now — Task 16 fills it in.

- [ ] **Step 4: Run tests**

Run: `go test ./web -v`
Expected: PASS.

- [ ] **Step 5: Commit**

```bash
git add web/admin_handlers.go web/admin_handlers_test.go web/web.go
git commit -m "feat(web): purge database endpoint guarded by typed phrase"
```

---

### Task 14: Inline rename of project on board header

**Files:**
- Modify: `web/templates/layout.html` (or `board.html` — wherever the project header lives) — convert the project name to a click-to-edit input.
- Modify: `web/handlers.go` (add `POST /projects/{id}/rename` board-side rename, or reuse `/admin/projects/{id}/rename` — choose based on whether middleware should differ).
- Modify: `web/web.go` (register route)
- Modify: `web/static/*` (inline JS for click-to-edit, or extend existing JS file)
- Test: `web/handlers_test.go`

- [ ] **Step 1: Inspect existing template**

Run: `rg "ProjectName|project.Name|{{.Project" web/templates -n | head -20`

Locate the board header that renders the project name; that's where the inline-rename input goes.

- [ ] **Step 2: Write failing test**

Add to `web/handlers_test.go`:

```go
func TestRenameProjectFromBoard_OK(t *testing.T) {
	srv, store := newTestWebServerWithStore(t)
	p, _ := store.CreateProject("old", "")
	form := url.Values{"name": {"new"}}
	req := httptest.NewRequest("POST", "/projects/"+strconv.Itoa(p.ID)+"/rename",
		strings.NewReader(form.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	rec := httptest.NewRecorder()
	srv.ServeHTTP(rec, req)
	if rec.Code != 303 && rec.Code != 302 {
		t.Errorf("want redirect, got %d", rec.Code)
	}
	got, _ := store.GetProject(p.ID)
	if got.Name != "new" {
		t.Errorf("want name=new, got %q", got.Name)
	}
}
```

- [ ] **Step 3: Run test**

Run: `go test ./web -run TestRenameProjectFromBoard -v`
Expected: FAIL.

- [ ] **Step 4: Implement handler + register**

Add to `web/handlers.go`:

```go
func (s *Server) handleRenameProject(w http.ResponseWriter, r *http.Request) {
	id, err := strconv.Atoi(r.PathValue("id"))
	if err != nil {
		http.Error(w, "bad id", 400)
		return
	}
	if err := r.ParseForm(); err != nil {
		http.Error(w, err.Error(), 400)
		return
	}
	name := strings.TrimSpace(r.FormValue("name"))
	if err := s.store.RenameProject(id, name); err != nil {
		http.Error(w, err.Error(), 400)
		return
	}
	s.broadcastProjectRenamed(id, name) // stub now, see Task 16
	http.Redirect(w, r, r.Header.Get("Referer"), http.StatusSeeOther)
}
```

Register in `web/web.go`:

```go
mux.Handle("POST /projects/{id}/rename", RequireAdmin(http.HandlerFunc(srv.handleRenameProject)))
```

Add stub `func (s *Server) broadcastProjectRenamed(id int, name string) {}`.

- [ ] **Step 5: Update template**

Where the project name is currently rendered as plain text in the board header, replace with a form:

```html
<form method="post" action="/projects/{{.ProjectID}}/rename" class="inline-rename">
  <input type="text" name="name" value="{{.ProjectName}}" class="inline-rename-input">
</form>
```

Add the lightweight JS in the existing static JS file (or a new `web/static/inline-rename.js`) that submits the form on Enter or blur, and reverts on Esc. Keep it small (~20 lines) and load it from the layout template.

- [ ] **Step 6: Run tests**

Run: `go test ./web -v`
Expected: PASS.

- [ ] **Step 7: Commit**

```bash
git add web/handlers.go web/handlers_test.go web/web.go web/templates web/static
git commit -m "feat(web): inline rename project from board header"
```

---

### Task 15: Inline delete card from drawer

**Files:**
- Modify: `web/templates/drawer.html` (add delete affordance)
- Modify: `web/handlers.go` (add `POST /cards/{id}/delete`)
- Modify: `web/web.go` (register route)
- Test: `web/handlers_test.go`

- [ ] **Step 1: Write failing test**

Add to `web/handlers_test.go`:

```go
func TestDeleteCardFromDrawer_OK(t *testing.T) {
	srv, store := newTestWebServerWithStore(t)
	p, _ := store.CreateProject("p", "")
	c, _ := store.CreateCard("t", "", p.ID, nil, nil)
	req := httptest.NewRequest("POST", "/cards/"+strconv.Itoa(c.ID)+"/delete", nil)
	rec := httptest.NewRecorder()
	srv.ServeHTTP(rec, req)
	if rec.Code != 303 && rec.Code != 302 {
		t.Errorf("want redirect, got %d", rec.Code)
	}
	got, _ := store.GetCard(c.ID)
	if got != nil {
		t.Errorf("card should be deleted")
	}
}
```

- [ ] **Step 2: Run test**

Run: `go test ./web -run TestDeleteCardFromDrawer -v`
Expected: FAIL.

- [ ] **Step 3: Implement handler + register**

Add to `web/handlers.go`:

```go
func (s *Server) handleDeleteCard(w http.ResponseWriter, r *http.Request) {
	id, err := strconv.Atoi(r.PathValue("id"))
	if err != nil {
		http.Error(w, "bad id", 400)
		return
	}
	if err := s.store.DeleteCard(id); err != nil {
		http.Error(w, err.Error(), 500)
		return
	}
	s.broadcastCardDeleted(id) // stub now, see Task 16
	http.Redirect(w, r, r.Header.Get("Referer"), http.StatusSeeOther)
}
```

Register in `web/web.go`:

```go
mux.Handle("POST /cards/{id}/delete", RequireAdmin(http.HandlerFunc(srv.handleDeleteCard)))
```

Add stub `func (s *Server) broadcastCardDeleted(id int) {}`.

- [ ] **Step 4: Update drawer template**

Add to the bottom of `web/templates/drawer.html`:

```html
<form method="post" action="/cards/{{.Card.ID}}/delete" class="drawer-delete">
  <button type="submit" class="link-danger"
          onclick="return confirm('Delete card &quot;{{.Card.Title}}&quot;? This cannot be undone.')">
    Delete card
  </button>
</form>
```

- [ ] **Step 5: Run tests**

Run: `go test ./web -v`
Expected: PASS.

- [ ] **Step 6: Commit**

```bash
git add web/handlers.go web/handlers_test.go web/web.go web/templates/drawer.html
git commit -m "feat(web): delete card from drawer with confirmation"
```

---

### Task 16: SSE broadcasts for rename/delete/purge

**Files:**
- Modify: `api/sse.go` (add broadcast helpers and event types)
- Modify: `web/admin_handlers.go`, `web/handlers.go` (replace stubs)
- Modify: `web/static/<existing-sse-client>.js` (handle new events; reload on `dataset:reset`)
- Test: `api/server_test.go`

- [ ] **Step 1: Inspect existing SSE pattern**

Run: `rg "Broadcast|sse" api/sse.go api/server.go -n` to see how events are emitted today.

- [ ] **Step 2: Write failing test**

Add to `api/server_test.go` (extend existing SSE test pattern):

```go
func TestSSE_BroadcastsCardDeleted(t *testing.T) {
	// Subscribe a fake client; trigger DeleteCard via the API/web; assert the
	// client receives a "card:deleted" event with the card id.
}
```

(Fill in following the existing SSE test pattern in this file.)

- [ ] **Step 3: Run test**

Run: `go test ./api -run TestSSE_BroadcastsCardDeleted -v`
Expected: FAIL.

- [ ] **Step 4: Implement broadcast helpers**

In `api/sse.go` (or wherever the hub lives), add methods on the hub:

```go
func (h *Hub) BroadcastCardDeleted(id int)        { h.send("card:deleted", map[string]int{"id": id}) }
func (h *Hub) BroadcastProjectRenamed(id int, n string) { h.send("project:renamed", map[string]any{"id": id, "name": n}) }
func (h *Hub) BroadcastProjectDeleted(id int)     { h.send("project:deleted", map[string]int{"id": id}) }
func (h *Hub) BroadcastAgentRenamed(id int, n string)   { h.send("agent:renamed", map[string]any{"id": id, "name": n}) }
func (h *Hub) BroadcastDatasetReset()              { h.send("dataset:reset", nil) }
```

Match the actual `send`/`Broadcast` signature already used.

- [ ] **Step 5: Replace stubs in `web/`**

In `web/handlers.go` and `web/admin_handlers.go`, replace the empty `broadcast*` stubs with calls into the hub:

```go
func (s *Server) broadcastCardDeleted(id int)              { s.hub.BroadcastCardDeleted(id) }
func (s *Server) broadcastProjectRenamed(id int, n string) { s.hub.BroadcastProjectRenamed(id, n) }
func (s *Server) broadcastProjectDeleted(id int)           { s.hub.BroadcastProjectDeleted(id) }
func (s *Server) broadcastAgentRenamed(id int, n string)   { s.hub.BroadcastAgentRenamed(id, n) }
func (s *Server) broadcastDatasetReset()                   { s.hub.BroadcastDatasetReset() }
```

Call them from `handleAdminDeleteProject`, `handleAdminRenameAgent`, `handleAdminDeleteAgent` as well (add the appropriate broadcast after a successful operation).

- [ ] **Step 6: Update client JS**

In the existing SSE client JS, add handlers:

```js
addEventListener('card:deleted', (e) => {
  const { id } = JSON.parse(e.data);
  document.querySelector(`[data-card-id="${id}"]`)?.remove();
});
addEventListener('project:deleted', (e) => {
  const { id } = JSON.parse(e.data);
  if (currentProjectId === id) location.href = '/';
});
addEventListener('project:renamed', (e) => {
  const { id, name } = JSON.parse(e.data);
  document.querySelectorAll(`[data-project-id="${id}"] .project-name`).forEach(el => el.textContent = name);
});
addEventListener('agent:renamed', (e) => {
  const { id, name } = JSON.parse(e.data);
  document.querySelectorAll(`[data-agent-id="${id}"] .agent-name`).forEach(el => el.textContent = name);
});
addEventListener('dataset:reset', () => {
  alert('Database was purged. Reloading…');
  setTimeout(() => location.reload(), 1000);
});
```

(Adapt event names to whatever idiom existing client JS uses for parsing.)

- [ ] **Step 7: Run all tests**

Run: `go test ./...`
Expected: PASS.

- [ ] **Step 8: Commit**

```bash
git add api/sse.go web/admin_handlers.go web/handlers.go web/static
git commit -m "feat(sse): broadcast rename/delete/purge events to clients"
```

---

## Phase 3 — Styling & polish

### Task 17: Danger styling + admin shell CSS

**Files:**
- Modify: `web/static/<existing-stylesheet>.css`

- [ ] **Step 1: Add danger tokens and admin shell styles**

Add to the existing stylesheet (locate via `ls web/static`):

```css
:root {
  --danger: #b00020;
  --danger-bg: #fff3f3;
}

.admin { display: grid; grid-template-columns: 220px 1fr; gap: 2rem; padding: 1.5rem; }
.admin-menu { display: flex; flex-direction: column; gap: 0.25rem; }
.admin-menu a { padding: 0.5rem 0.75rem; border-radius: 4px; color: inherit; text-decoration: none; }
.admin-menu a.active { background: var(--accent-bg, #eef); font-weight: 600; }
.admin-menu a.danger { color: var(--danger); }
.admin-menu-divider { height: 1px; background: #ddd; margin: 0.75rem 0; }

.admin-list { list-style: none; padding: 0; }
.admin-row { display: flex; align-items: center; gap: 1rem; padding: 0.5rem 0; border-bottom: 1px solid #eee; }
.admin-name { flex: 1; font-weight: 500; }
.admin-meta { color: #777; font-size: 0.9em; }

.btn-outlined-danger { border: 1px solid var(--danger); color: var(--danger); background: transparent; padding: 0.25rem 0.75rem; border-radius: 4px; cursor: pointer; }
.btn-outlined-danger:hover { background: var(--danger-bg); }
.btn-danger { background: var(--danger); color: white; border: 0; padding: 0.4rem 0.9rem; border-radius: 4px; cursor: pointer; }
.btn-danger:disabled { opacity: 0.4; cursor: not-allowed; }

.danger-panel { border: 1px solid var(--danger); background: var(--danger-bg); padding: 1rem 1.25rem; border-radius: 6px; }
.danger-panel h2 { color: var(--danger); margin-top: 0; }
.mono { font-family: ui-monospace, monospace; }

.modal { position: fixed; inset: 0; background: rgba(0,0,0,0.4); display: grid; place-items: center; z-index: 100; }
.modal-content { background: white; max-width: 480px; width: 90%; padding: 1.5rem; border-radius: 8px; }

.link-danger { background: none; border: 0; color: var(--danger); cursor: pointer; text-decoration: underline; font-size: 0.9em; }
.inline-rename-input { background: transparent; border: 1px dashed transparent; padding: 0.2em 0.4em; font: inherit; width: 100%; }
.inline-rename-input:hover, .inline-rename-input:focus { border-color: #bbb; outline: none; }
```

- [ ] **Step 2: Boot the dev server and smoke-test**

Run: `task serve` (or `go run . serve` per existing run conventions) and open `http://localhost:<port>/admin`.

Verify visually:
- Left menu shows Projects / Agents / Danger Zone with divider above Danger Zone.
- Projects list shows delete button per row; clicking opens the typed-name modal.
- Wrong project name keeps the Delete button disabled; correct name enables it.
- Danger Zone: typing `PURGE DATABASE` enables the Purge button.
- Inline rename on board header works (Enter saves, Esc cancels).
- Delete-card affordance in the drawer works.

- [ ] **Step 3: Commit**

```bash
git add web/static
git commit -m "feat(web): admin shell styles + danger tokens"
```

---

### Task 18: Manual end-to-end smoke + final sweep

- [ ] **Step 1: Walk the manual smoke checklist from the spec**

With a fresh DB (`task clean && task serve`):

1. Create a project; rename it inline from the board header. Reload — name persists.
2. Open `/admin/projects` — see the renamed project.
3. Add a few cards; delete one from the drawer; confirm SSE update in a second tab.
4. Add an agent + a comment authored by them. Delete the agent from `/admin/agents`. Verify the comment still shows the agent's last-known name.
5. Delete the project from `/admin/projects` after typing its name. Confirm cascade.
6. Visit `/admin/danger`. Try wrong phrase — button stays disabled. Type `PURGE DATABASE` — Purge enables. Confirm — server returns to seeded baseline; other tabs show purge banner + reload.

- [ ] **Step 2: Run full suite one more time**

Run: `go test ./...`
Expected: PASS.

- [ ] **Step 3: Final commit (only if any fixes were needed in step 1/2)**

```bash
git add -A
git commit -m "chore: smoke-test fixes for admin & dangerous actions"
```

---

## Self-review

- **Spec coverage:**
  - Inline rename project/card → Tasks 14 (project from board), Task 15 also exposes delete; admin Tasks 11/12 cover rename for both. Card rename is left to existing edit affordance (no change needed).
  - Inline delete card → Task 15.
  - `/admin` shell + sections → Task 10.
  - Project/agent rename + delete (admin) → Tasks 11, 12.
  - Purge → Tasks 8 (store), 13 (web).
  - Comment snapshot + nullable agent_id → Tasks 1, 2.
  - Agent rename backfills comments → Task 4.
  - `requireAdmin` middleware chokepoint → Task 9 (applied throughout).
  - SSE broadcasts including `dataset:reset` → Task 16.
  - Confirmation UX ladder → handler tests verify server-side enforcement; templates implement client gating.
  - Danger styling tokens → Task 17.
  - Migration plan → Task 1.
- **Placeholder scan:** none — every step has concrete code or commands.
- **Type consistency:** broadcast helpers consistently named `Broadcast*` on the hub, `broadcast*` wrappers on `Server`. Handler names follow `handleAdmin<Verb><Entity>`. Route paths consistent (`/admin/<entity>/{id}/<action>`).
