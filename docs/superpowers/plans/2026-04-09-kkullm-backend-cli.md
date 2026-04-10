# Kkullm Backend + CLI Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Build the Kkullm server (REST API + SSE + SQLite) and CLI client as a single Go binary.

**Architecture:** Single Go binary using cobra for CLI subcommands. `kkullm serve` starts an HTTP server that owns the SQLite database (WAL mode) and exposes a REST API. All other subcommands are thin HTTP clients. Pure Go SQLite via `modernc.org/sqlite` for easy cross-compilation.

**Tech Stack:** Go 1.22+, cobra (CLI), modernc.org/sqlite (pure Go SQLite driver), database/sql, net/http (Go 1.22 routing), httptest (testing)

**Spec:** `docs/superpowers/specs/2026-04-09-kkullm-prd-design.md`

**Scope:** This plan covers the backend and CLI only. The web UI SPA is a separate plan that builds on top of this API.

---

## File Structure

```
kkullm/
├── main.go                     # Entry point, calls cmd.Execute()
├── go.mod
├── go.sum
├── cmd/
│   ├── root.go                 # Cobra root command, global flags (--server, --as, --project)
│   ├── serve.go                # kkullm serve
│   ├── card.go                 # kkullm card {list,show,create,update}
│   ├── comment.go              # kkullm comment {list,add}
│   ├── project.go              # kkullm project {list,create}
│   ├── agent.go                # kkullm agent {list,create,show}
│   └── asset.go                # kkullm asset {list,create,show}
├── db/
│   ├── db.go                   # Open, Close, Migrate, Seed functions
│   ├── db_test.go              # Database setup and migration tests
│   └── migrations/
│       └── 001_initial.sql     # Full schema DDL
├── model/
│   └── model.go                # All struct types: Project, Agent, Card, Comment, etc.
├── store/
│   ├── project.go              # Project CRUD queries
│   ├── project_test.go
│   ├── agent.go                # Agent CRUD queries
│   ├── agent_test.go
│   ├── card.go                 # Card CRUD + assignees, tags, relations, status transitions
│   ├── card_test.go
│   ├── comment.go              # Comment queries
│   ├── comment_test.go
│   ├── asset.go                # Asset CRUD + glob search
│   └── asset_test.go
├── api/
│   ├── server.go               # HTTP server setup, middleware, route registration
│   ├── server_test.go          # Integration tests for full request/response cycles
│   ├── projects.go             # Project HTTP handlers
│   ├── agents.go               # Agent HTTP handlers
│   ├── cards.go                # Card HTTP handlers
│   ├── comments.go             # Comment HTTP handlers
│   ├── assets.go               # Asset HTTP handlers
│   └── sse.go                  # SSE event stream handler + event bus
└── client/
    └── client.go               # HTTP client used by CLI commands
```

---

## Task 1: Project Scaffolding

**Files:**
- Create: `go.mod`, `main.go`, `cmd/root.go`

- [ ] **Step 1: Initialize Go module**

```bash
cd /Users/joelhelbling/code/ai/kkullm
go mod init github.com/joelhelbling/kkullm
```

- [ ] **Step 2: Install cobra**

```bash
go get github.com/spf13/cobra@latest
```

- [ ] **Step 3: Create main.go**

```go
// main.go
package main

import "github.com/joelhelbling/kkullm/cmd"

func main() {
	cmd.Execute()
}
```

- [ ] **Step 4: Create cmd/root.go with global flags**

```go
// cmd/root.go
package cmd

import (
	"fmt"
	"os"

	"github.com/spf13/cobra"
)

var (
	serverURL   string
	agentName   string
	projectName string
)

var rootCmd = &cobra.Command{
	Use:   "kkullm",
	Short: "Agent orchestration system based on the blackboard pattern",
}

func Execute() {
	if err := rootCmd.Execute(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

func init() {
	rootCmd.PersistentFlags().StringVar(&serverURL, "server", envOrDefault("KKULLM_SERVER", "http://localhost:8080"), "Kkullm server URL")
	rootCmd.PersistentFlags().StringVar(&agentName, "as", os.Getenv("KKULLM_AGENT"), "Agent identity")
	rootCmd.PersistentFlags().StringVar(&projectName, "project", os.Getenv("KKULLM_PROJECT"), "Default project")
}

func envOrDefault(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}

// requireAgent returns the agent name or exits with an error if not set.
func requireAgent() string {
	if agentName == "" {
		fmt.Fprintln(os.Stderr, "Error: agent identity required. Set KKULLM_AGENT or use --as flag.")
		os.Exit(1)
	}
	return agentName
}
```

- [ ] **Step 5: Verify it compiles and runs**

```bash
go build -o kkullm . && ./kkullm --help
```

Expected: help output showing "Agent orchestration system based on the blackboard pattern" with `--server`, `--as`, `--project` flags.

- [ ] **Step 6: Commit**

```bash
git add main.go go.mod go.sum cmd/root.go
git commit -m "feat: scaffold Go project with cobra root command and global flags"
```

---

## Task 2: SQLite Database Layer

**Files:**
- Create: `db/db.go`, `db/db_test.go`, `db/migrations/001_initial.sql`

- [ ] **Step 1: Install pure Go SQLite driver**

```bash
go get modernc.org/sqlite@latest
```

- [ ] **Step 2: Write the schema migration file**

```sql
-- db/migrations/001_initial.sql

-- Projects
CREATE TABLE IF NOT EXISTS projects (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    name TEXT NOT NULL UNIQUE,
    description TEXT,
    created_at DATETIME NOT NULL DEFAULT (datetime('now')),
    updated_at DATETIME NOT NULL DEFAULT (datetime('now'))
);

-- Agents
CREATE TABLE IF NOT EXISTS agents (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    name TEXT NOT NULL UNIQUE,
    project_id INTEGER NOT NULL REFERENCES projects(id),
    bio TEXT,
    created_at DATETIME NOT NULL DEFAULT (datetime('now')),
    updated_at DATETIME NOT NULL DEFAULT (datetime('now'))
);

-- Project Assets
CREATE TABLE IF NOT EXISTS project_assets (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    project_id INTEGER NOT NULL REFERENCES projects(id),
    name TEXT NOT NULL,
    description TEXT,
    url TEXT,
    created_at DATETIME NOT NULL DEFAULT (datetime('now')),
    updated_at DATETIME NOT NULL DEFAULT (datetime('now'))
);

-- Cards
CREATE TABLE IF NOT EXISTS cards (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    title TEXT NOT NULL,
    body TEXT,
    status TEXT NOT NULL DEFAULT 'considering'
        CHECK(status IN ('considering', 'todo', 'in_flight', 'completed', 'done', 'tabled', 'blocked')),
    project_id INTEGER NOT NULL REFERENCES projects(id),
    created_at DATETIME NOT NULL DEFAULT (datetime('now')),
    updated_at DATETIME NOT NULL DEFAULT (datetime('now'))
);

-- Card Assignees (join table)
CREATE TABLE IF NOT EXISTS card_assignees (
    card_id INTEGER NOT NULL REFERENCES cards(id) ON DELETE CASCADE,
    agent_id INTEGER NOT NULL REFERENCES agents(id),
    PRIMARY KEY (card_id, agent_id)
);

-- Card Tags
CREATE TABLE IF NOT EXISTS card_tags (
    card_id INTEGER NOT NULL REFERENCES cards(id) ON DELETE CASCADE,
    tag TEXT NOT NULL,
    PRIMARY KEY (card_id, tag)
);

-- Card Relations
CREATE TABLE IF NOT EXISTS card_relations (
    card_id INTEGER NOT NULL REFERENCES cards(id) ON DELETE CASCADE,
    related_card_id INTEGER NOT NULL REFERENCES cards(id) ON DELETE CASCADE,
    relation_type TEXT NOT NULL
        CHECK(relation_type IN ('blocked_by', 'belongs_to', 'interested_in')),
    PRIMARY KEY (card_id, related_card_id, relation_type)
);

-- Comments
CREATE TABLE IF NOT EXISTS comments (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    card_id INTEGER NOT NULL REFERENCES cards(id) ON DELETE CASCADE,
    agent_id INTEGER NOT NULL REFERENCES agents(id),
    body TEXT NOT NULL,
    created_at DATETIME NOT NULL DEFAULT (datetime('now'))
);

-- Indexes for common queries
CREATE INDEX IF NOT EXISTS idx_cards_project_status ON cards(project_id, status);
CREATE INDEX IF NOT EXISTS idx_card_assignees_agent ON card_assignees(agent_id);
CREATE INDEX IF NOT EXISTS idx_comments_card ON comments(card_id);
CREATE INDEX IF NOT EXISTS idx_agents_project ON agents(project_id);
CREATE INDEX IF NOT EXISTS idx_project_assets_project ON project_assets(project_id);
```

- [ ] **Step 3: Write db.go with Open, Migrate, Seed**

```go
// db/db.go
package db

import (
	"database/sql"
	"embed"
	"fmt"

	_ "modernc.org/sqlite"
)

//go:embed migrations/*.sql
var migrations embed.FS

// Open opens a SQLite database at the given path with WAL mode enabled.
// Use ":memory:" for testing.
func Open(path string) (*sql.DB, error) {
	db, err := sql.Open("sqlite", path)
	if err != nil {
		return nil, fmt.Errorf("open database: %w", err)
	}

	// Enable WAL mode and foreign keys
	for _, pragma := range []string{
		"PRAGMA journal_mode=WAL",
		"PRAGMA foreign_keys=ON",
	} {
		if _, err := db.Exec(pragma); err != nil {
			db.Close()
			return nil, fmt.Errorf("exec %q: %w", pragma, err)
		}
	}

	return db, nil
}

// Migrate runs all SQL migration files in order.
func Migrate(db *sql.DB) error {
	data, err := migrations.ReadFile("migrations/001_initial.sql")
	if err != nil {
		return fmt.Errorf("read migration: %w", err)
	}
	if _, err := db.Exec(string(data)); err != nil {
		return fmt.Errorf("exec migration: %w", err)
	}
	return nil
}

// Seed creates the "orchestration" project and "user" agent if they don't exist.
func Seed(db *sql.DB) error {
	tx, err := db.Begin()
	if err != nil {
		return fmt.Errorf("begin seed tx: %w", err)
	}
	defer tx.Rollback()

	_, err = tx.Exec(`INSERT OR IGNORE INTO projects (name, description) VALUES ('orchestration', 'Oversight of the Kkullm board')`)
	if err != nil {
		return fmt.Errorf("seed orchestration project: %w", err)
	}

	_, err = tx.Exec(`INSERT OR IGNORE INTO agents (name, project_id, bio) VALUES ('user', (SELECT id FROM projects WHERE name = 'orchestration'), 'The human operator')`)
	if err != nil {
		return fmt.Errorf("seed user agent: %w", err)
	}

	return tx.Commit()
}
```

- [ ] **Step 4: Write the failing test for database setup**

```go
// db/db_test.go
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

	// Verify tables exist by querying them
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

	// Verify orchestration project exists
	var projName string
	err = database.QueryRow("SELECT name FROM projects WHERE name = 'orchestration'").Scan(&projName)
	if err != nil {
		t.Fatalf("orchestration project not found: %v", err)
	}

	// Verify user agent exists and belongs to orchestration
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

	// Verify idempotency — running Seed again should not error
	if err := Seed(database); err != nil {
		t.Fatalf("second Seed call failed: %v", err)
	}
}
```

- [ ] **Step 5: Run tests**

```bash
go test ./db/ -v
```

Expected: PASS for both tests.

- [ ] **Step 6: Commit**

```bash
git add db/
git commit -m "feat: add SQLite database layer with schema, migrations, and seeding"
```

---

## Task 3: Model Types

**Files:**
- Create: `model/model.go`

- [ ] **Step 1: Define all model structs**

```go
// model/model.go
package model

import "time"

type Project struct {
	ID          int       `json:"id"`
	Name        string    `json:"name"`
	Description string    `json:"description,omitempty"`
	CreatedAt   time.Time `json:"created_at"`
	UpdatedAt   time.Time `json:"updated_at"`
}

type Agent struct {
	ID        int       `json:"id"`
	Name      string    `json:"name"`
	ProjectID int       `json:"project_id"`
	Project   string    `json:"project,omitempty"` // populated by joins
	Bio       string    `json:"bio,omitempty"`
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
}

type Card struct {
	ID           int            `json:"id"`
	Title        string         `json:"title"`
	Body         string         `json:"body,omitempty"`
	Status       string         `json:"status"`
	ProjectID    int            `json:"project_id"`
	Project      string         `json:"project,omitempty"` // populated by joins
	Assignees    []string       `json:"assignees,omitempty"`
	Tags         []string       `json:"tags,omitempty"`
	Relations    []CardRelation `json:"relations,omitempty"`
	CommentCount int            `json:"comment_count,omitempty"`
	CreatedAt    time.Time      `json:"created_at"`
	UpdatedAt    time.Time      `json:"updated_at"`
}

type CardRelation struct {
	RelatedCardID int    `json:"related_card_id"`
	RelationType  string `json:"relation_type"`
}

type Comment struct {
	ID        int       `json:"id"`
	CardID    int       `json:"card_id"`
	AgentID   int       `json:"agent_id"`
	Agent     string    `json:"agent,omitempty"` // populated by joins
	Body      string    `json:"body"`
	CreatedAt time.Time `json:"created_at"`
}

type ProjectAsset struct {
	ID          int       `json:"id"`
	ProjectID   int       `json:"project_id"`
	Project     string    `json:"project,omitempty"` // populated by joins
	Name        string    `json:"name"`
	Description string    `json:"description,omitempty"`
	URL         string    `json:"url,omitempty"`
	CreatedAt   time.Time `json:"created_at"`
	UpdatedAt   time.Time `json:"updated_at"`
}

// Valid card statuses
var ValidStatuses = map[string]bool{
	"considering": true,
	"todo":        true,
	"in_flight":   true,
	"completed":   true,
	"done":        true,
	"tabled":      true,
	"blocked":     true,
}

// ValidTransitions maps each status to the statuses it can transition to.
var ValidTransitions = map[string]map[string]bool{
	"considering": {"todo": true, "tabled": true},
	"todo":        {"in_flight": true, "blocked": true, "tabled": true},
	"in_flight":   {"completed": true, "blocked": true, "tabled": true},
	"completed":   {"done": true, "in_flight": true, "tabled": true},
	"blocked":     {"todo": true, "in_flight": true, "tabled": true},
	"tabled":      {"considering": true, "todo": true},
	"done":        {},
}

// CanTransition checks whether a status transition is valid.
func CanTransition(from, to string) bool {
	targets, ok := ValidTransitions[from]
	if !ok {
		return false
	}
	return targets[to]
}

// AllowedTransitions returns the list of valid target statuses from the given status.
func AllowedTransitions(from string) []string {
	targets, ok := ValidTransitions[from]
	if !ok {
		return nil
	}
	result := make([]string, 0, len(targets))
	for s := range targets {
		result = append(result, s)
	}
	return result
}
```

- [ ] **Step 2: Verify it compiles**

```bash
go build ./model/
```

Expected: no errors.

- [ ] **Step 3: Commit**

```bash
git add model/
git commit -m "feat: define model types with card status transition rules"
```

---

## Task 4: Project Store

**Files:**
- Create: `store/project.go`, `store/project_test.go`

- [ ] **Step 1: Write the failing tests**

```go
// store/project_test.go
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

	// Seeded orchestration project should be there
	projects, err := s.ListProjects()
	if err != nil {
		t.Fatalf("ListProjects: %v", err)
	}
	if len(projects) != 1 {
		t.Fatalf("got %d projects, want 1 (orchestration)", len(projects))
	}

	// Create a new project
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

	// List again — should have 2
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
```

- [ ] **Step 2: Run tests to verify they fail**

```bash
go test ./store/ -v
```

Expected: FAIL — `Store` type not defined.

- [ ] **Step 3: Implement the project store**

```go
// store/project.go
package store

import (
	"database/sql"
	"fmt"

	"github.com/joelhelbling/kkullm/model"
)

// Store wraps a database connection and provides query methods.
type Store struct {
	db *sql.DB
}

// New creates a new Store.
func New(db *sql.DB) *Store {
	return &Store{db: db}
}

func (s *Store) CreateProject(name, description string) (*model.Project, error) {
	result, err := s.db.Exec(
		"INSERT INTO projects (name, description) VALUES (?, ?)",
		name, description,
	)
	if err != nil {
		return nil, fmt.Errorf("insert project: %w", err)
	}

	id, err := result.LastInsertId()
	if err != nil {
		return nil, fmt.Errorf("last insert id: %w", err)
	}

	return s.GetProject(int(id))
}

func (s *Store) GetProject(id int) (*model.Project, error) {
	p := &model.Project{}
	err := s.db.QueryRow(
		"SELECT id, name, COALESCE(description, ''), created_at, updated_at FROM projects WHERE id = ?", id,
	).Scan(&p.ID, &p.Name, &p.Description, &p.CreatedAt, &p.UpdatedAt)
	if err != nil {
		return nil, fmt.Errorf("get project %d: %w", id, err)
	}
	return p, nil
}

func (s *Store) GetProjectByName(name string) (*model.Project, error) {
	p := &model.Project{}
	err := s.db.QueryRow(
		"SELECT id, name, COALESCE(description, ''), created_at, updated_at FROM projects WHERE name = ?", name,
	).Scan(&p.ID, &p.Name, &p.Description, &p.CreatedAt, &p.UpdatedAt)
	if err != nil {
		return nil, fmt.Errorf("get project %q: %w", name, err)
	}
	return p, nil
}

func (s *Store) ListProjects() ([]model.Project, error) {
	rows, err := s.db.Query("SELECT id, name, COALESCE(description, ''), created_at, updated_at FROM projects ORDER BY name")
	if err != nil {
		return nil, fmt.Errorf("list projects: %w", err)
	}
	defer rows.Close()

	var projects []model.Project
	for rows.Next() {
		var p model.Project
		if err := rows.Scan(&p.ID, &p.Name, &p.Description, &p.CreatedAt, &p.UpdatedAt); err != nil {
			return nil, fmt.Errorf("scan project: %w", err)
		}
		projects = append(projects, p)
	}
	return projects, rows.Err()
}
```

- [ ] **Step 4: Run tests to verify they pass**

```bash
go test ./store/ -v
```

Expected: PASS for all three tests.

- [ ] **Step 5: Commit**

```bash
git add store/project.go store/project_test.go
git commit -m "feat: add project store with CRUD operations"
```

---

## Task 5: Agent Store

**Files:**
- Create: `store/agent.go`, `store/agent_test.go`

- [ ] **Step 1: Write the failing tests**

```go
// store/agent_test.go
package store

import (
	"testing"
)

func TestCreateAndListAgents(t *testing.T) {
	s := setupTestDB(t)

	// Seeded user agent should exist
	agents, err := s.ListAgents("")
	if err != nil {
		t.Fatalf("ListAgents: %v", err)
	}
	if len(agents) != 1 {
		t.Fatalf("got %d agents, want 1 (user)", len(agents))
	}
	if agents[0].Name != "user" {
		t.Errorf("seeded agent name = %q, want 'user'", agents[0].Name)
	}

	// Create a project for the new agent
	proj, err := s.CreateProject("acme", "")
	if err != nil {
		t.Fatalf("CreateProject: %v", err)
	}

	// Create an agent
	agent, err := s.CreateAgent("dev-agent", proj.ID, "Writes Go code")
	if err != nil {
		t.Fatalf("CreateAgent: %v", err)
	}
	if agent.Name != "dev-agent" {
		t.Errorf("name = %q, want 'dev-agent'", agent.Name)
	}
	if agent.Bio != "Writes Go code" {
		t.Errorf("bio = %q, want 'Writes Go code'", agent.Bio)
	}

	// List all agents
	agents, err = s.ListAgents("")
	if err != nil {
		t.Fatalf("ListAgents: %v", err)
	}
	if len(agents) != 2 {
		t.Fatalf("got %d agents, want 2", len(agents))
	}

	// List agents filtered by project
	agents, err = s.ListAgents("acme")
	if err != nil {
		t.Fatalf("ListAgents(acme): %v", err)
	}
	if len(agents) != 1 {
		t.Fatalf("got %d agents for acme, want 1", len(agents))
	}
}

func TestGetAgentByName(t *testing.T) {
	s := setupTestDB(t)

	agent, err := s.GetAgentByName("user")
	if err != nil {
		t.Fatalf("GetAgentByName: %v", err)
	}
	if agent.Name != "user" {
		t.Errorf("name = %q, want 'user'", agent.Name)
	}
	if agent.Project != "orchestration" {
		t.Errorf("project = %q, want 'orchestration'", agent.Project)
	}
}
```

- [ ] **Step 2: Run tests to verify they fail**

```bash
go test ./store/ -v -run TestAgent
```

Expected: FAIL — methods not defined.

- [ ] **Step 3: Implement the agent store**

```go
// store/agent.go
package store

import (
	"fmt"

	"github.com/joelhelbling/kkullm/model"
)

func (s *Store) CreateAgent(name string, projectID int, bio string) (*model.Agent, error) {
	result, err := s.db.Exec(
		"INSERT INTO agents (name, project_id, bio) VALUES (?, ?, ?)",
		name, projectID, bio,
	)
	if err != nil {
		return nil, fmt.Errorf("insert agent: %w", err)
	}

	id, err := result.LastInsertId()
	if err != nil {
		return nil, fmt.Errorf("last insert id: %w", err)
	}

	return s.GetAgent(int(id))
}

func (s *Store) GetAgent(id int) (*model.Agent, error) {
	a := &model.Agent{}
	err := s.db.QueryRow(`
		SELECT a.id, a.name, a.project_id, p.name, COALESCE(a.bio, ''), a.created_at, a.updated_at
		FROM agents a JOIN projects p ON a.project_id = p.id
		WHERE a.id = ?
	`, id).Scan(&a.ID, &a.Name, &a.ProjectID, &a.Project, &a.Bio, &a.CreatedAt, &a.UpdatedAt)
	if err != nil {
		return nil, fmt.Errorf("get agent %d: %w", id, err)
	}
	return a, nil
}

func (s *Store) GetAgentByName(name string) (*model.Agent, error) {
	a := &model.Agent{}
	err := s.db.QueryRow(`
		SELECT a.id, a.name, a.project_id, p.name, COALESCE(a.bio, ''), a.created_at, a.updated_at
		FROM agents a JOIN projects p ON a.project_id = p.id
		WHERE a.name = ?
	`, name).Scan(&a.ID, &a.Name, &a.ProjectID, &a.Project, &a.Bio, &a.CreatedAt, &a.UpdatedAt)
	if err != nil {
		return nil, fmt.Errorf("get agent %q: %w", name, err)
	}
	return a, nil
}

func (s *Store) ListAgents(projectName string) ([]model.Agent, error) {
	query := `
		SELECT a.id, a.name, a.project_id, p.name, COALESCE(a.bio, ''), a.created_at, a.updated_at
		FROM agents a JOIN projects p ON a.project_id = p.id
	`
	var args []any
	if projectName != "" {
		query += " WHERE p.name = ?"
		args = append(args, projectName)
	}
	query += " ORDER BY a.name"

	rows, err := s.db.Query(query, args...)
	if err != nil {
		return nil, fmt.Errorf("list agents: %w", err)
	}
	defer rows.Close()

	var agents []model.Agent
	for rows.Next() {
		var a model.Agent
		if err := rows.Scan(&a.ID, &a.Name, &a.ProjectID, &a.Project, &a.Bio, &a.CreatedAt, &a.UpdatedAt); err != nil {
			return nil, fmt.Errorf("scan agent: %w", err)
		}
		agents = append(agents, a)
	}
	return agents, rows.Err()
}
```

- [ ] **Step 4: Run tests to verify they pass**

```bash
go test ./store/ -v
```

Expected: all tests PASS.

- [ ] **Step 5: Commit**

```bash
git add store/agent.go store/agent_test.go
git commit -m "feat: add agent store with CRUD and project-filtered listing"
```

---

## Task 6: Card Store — Core CRUD

**Files:**
- Create: `store/card.go`, `store/card_test.go`

- [ ] **Step 1: Write failing tests for card creation and listing**

```go
// store/card_test.go
package store

import (
	"testing"

	"github.com/joelhelbling/kkullm/model"
)

func createTestProject(t *testing.T, s *Store) *model.Project {
	t.Helper()
	p, err := s.CreateProject("test-project", "")
	if err != nil {
		t.Fatalf("CreateProject: %v", err)
	}
	return p
}

func createTestAgent(t *testing.T, s *Store, name string, projectID int) *model.Agent {
	t.Helper()
	a, err := s.CreateAgent(name, projectID, "")
	if err != nil {
		t.Fatalf("CreateAgent: %v", err)
	}
	return a
}

type CardCreateParams struct {
	Title     string
	Body      string
	Status    string
	ProjectID int
	Assignees []string
	Tags      []string
	Relations []model.CardRelation
}

func TestCreateAndGetCard(t *testing.T) {
	s := setupTestDB(t)
	proj := createTestProject(t, s)

	card, err := s.CreateCard(CardCreateParams{
		Title:     "Implement auth",
		Body:      "Add JWT middleware",
		Status:    "todo",
		ProjectID: proj.ID,
	})
	if err != nil {
		t.Fatalf("CreateCard: %v", err)
	}
	if card.Title != "Implement auth" {
		t.Errorf("title = %q, want 'Implement auth'", card.Title)
	}
	if card.Status != "todo" {
		t.Errorf("status = %q, want 'todo'", card.Status)
	}

	got, err := s.GetCard(card.ID)
	if err != nil {
		t.Fatalf("GetCard: %v", err)
	}
	if got.Title != "Implement auth" {
		t.Errorf("title = %q, want 'Implement auth'", got.Title)
	}
}

func TestCreateCardWithAssigneesAndTags(t *testing.T) {
	s := setupTestDB(t)
	proj := createTestProject(t, s)
	agent := createTestAgent(t, s, "dev-agent", proj.ID)
	_ = agent

	card, err := s.CreateCard(CardCreateParams{
		Title:     "Write tests",
		Status:    "todo",
		ProjectID: proj.ID,
		Assignees: []string{"dev-agent"},
		Tags:      []string{"testing", "backend"},
	})
	if err != nil {
		t.Fatalf("CreateCard: %v", err)
	}

	got, err := s.GetCard(card.ID)
	if err != nil {
		t.Fatalf("GetCard: %v", err)
	}
	if len(got.Assignees) != 1 || got.Assignees[0] != "dev-agent" {
		t.Errorf("assignees = %v, want [dev-agent]", got.Assignees)
	}
	if len(got.Tags) != 2 {
		t.Errorf("tags = %v, want [backend testing]", got.Tags)
	}
}

func TestCreateCardWithRelations(t *testing.T) {
	s := setupTestDB(t)
	proj := createTestProject(t, s)

	parent, err := s.CreateCard(CardCreateParams{
		Title: "Epic", Status: "todo", ProjectID: proj.ID,
	})
	if err != nil {
		t.Fatalf("CreateCard parent: %v", err)
	}

	child, err := s.CreateCard(CardCreateParams{
		Title:     "Sub-task",
		Status:    "todo",
		ProjectID: proj.ID,
		Relations: []model.CardRelation{
			{RelatedCardID: parent.ID, RelationType: "belongs_to"},
		},
	})
	if err != nil {
		t.Fatalf("CreateCard child: %v", err)
	}

	got, err := s.GetCard(child.ID)
	if err != nil {
		t.Fatalf("GetCard: %v", err)
	}
	if len(got.Relations) != 1 {
		t.Fatalf("relations = %v, want 1 relation", got.Relations)
	}
	if got.Relations[0].RelationType != "belongs_to" {
		t.Errorf("relation type = %q, want 'belongs_to'", got.Relations[0].RelationType)
	}
}

func TestListCardsFiltered(t *testing.T) {
	s := setupTestDB(t)
	proj := createTestProject(t, s)
	agent := createTestAgent(t, s, "dev-agent", proj.ID)
	_ = agent

	s.CreateCard(CardCreateParams{
		Title: "Card A", Status: "todo", ProjectID: proj.ID,
		Assignees: []string{"dev-agent"}, Tags: []string{"backend"},
	})
	s.CreateCard(CardCreateParams{
		Title: "Card B", Status: "in_flight", ProjectID: proj.ID,
		Assignees: []string{"dev-agent"},
	})
	s.CreateCard(CardCreateParams{
		Title: "Card C", Status: "todo", ProjectID: proj.ID,
		Assignees: []string{"user"}, Tags: []string{"docs"},
	})

	// Filter by status
	cards, err := s.ListCards(CardListParams{Status: "todo"})
	if err != nil {
		t.Fatalf("ListCards status=todo: %v", err)
	}
	if len(cards) != 2 {
		t.Errorf("got %d cards with status=todo, want 2", len(cards))
	}

	// Filter by assignee
	cards, err = s.ListCards(CardListParams{Assignee: "dev-agent"})
	if err != nil {
		t.Fatalf("ListCards assignee=dev-agent: %v", err)
	}
	if len(cards) != 2 {
		t.Errorf("got %d cards for dev-agent, want 2", len(cards))
	}

	// Filter by tag
	cards, err = s.ListCards(CardListParams{Tag: "backend"})
	if err != nil {
		t.Fatalf("ListCards tag=backend: %v", err)
	}
	if len(cards) != 1 {
		t.Errorf("got %d cards with tag=backend, want 1", len(cards))
	}

	// Filter by project
	cards, err = s.ListCards(CardListParams{Project: "test-project"})
	if err != nil {
		t.Fatalf("ListCards project=test-project: %v", err)
	}
	if len(cards) != 3 {
		t.Errorf("got %d cards for test-project, want 3", len(cards))
	}
}
```

- [ ] **Step 2: Run tests to verify they fail**

```bash
go test ./store/ -v -run TestCard
```

Expected: FAIL — `CreateCard`, `CardListParams` not defined.

- [ ] **Step 3: Implement card store**

```go
// store/card.go
package store

import (
	"fmt"
	"strings"

	"github.com/joelhelbling/kkullm/model"
)

type CardListParams struct {
	Project  string
	Assignee string
	Status   string
	Tag      string
}

func (s *Store) CreateCard(p CardCreateParams) (*model.Card, error) {
	tx, err := s.db.Begin()
	if err != nil {
		return nil, fmt.Errorf("begin tx: %w", err)
	}
	defer tx.Rollback()

	status := p.Status
	if status == "" {
		status = "considering"
	}

	result, err := tx.Exec(
		"INSERT INTO cards (title, body, status, project_id) VALUES (?, ?, ?, ?)",
		p.Title, p.Body, status, p.ProjectID,
	)
	if err != nil {
		return nil, fmt.Errorf("insert card: %w", err)
	}

	cardID, err := result.LastInsertId()
	if err != nil {
		return nil, fmt.Errorf("last insert id: %w", err)
	}

	// Add assignees
	for _, name := range p.Assignees {
		_, err := tx.Exec(
			"INSERT INTO card_assignees (card_id, agent_id) SELECT ?, id FROM agents WHERE name = ?",
			cardID, name,
		)
		if err != nil {
			return nil, fmt.Errorf("assign %q: %w", name, err)
		}
	}

	// Add tags
	for _, tag := range p.Tags {
		_, err := tx.Exec(
			"INSERT INTO card_tags (card_id, tag) VALUES (?, ?)",
			cardID, tag,
		)
		if err != nil {
			return nil, fmt.Errorf("add tag %q: %w", tag, err)
		}
	}

	// Add relations
	for _, rel := range p.Relations {
		_, err := tx.Exec(
			"INSERT INTO card_relations (card_id, related_card_id, relation_type) VALUES (?, ?, ?)",
			cardID, rel.RelatedCardID, rel.RelationType,
		)
		if err != nil {
			return nil, fmt.Errorf("add relation: %w", err)
		}
	}

	if err := tx.Commit(); err != nil {
		return nil, fmt.Errorf("commit: %w", err)
	}

	return s.GetCard(int(cardID))
}

func (s *Store) GetCard(id int) (*model.Card, error) {
	c := &model.Card{}
	err := s.db.QueryRow(`
		SELECT c.id, c.title, COALESCE(c.body, ''), c.status, c.project_id, p.name,
			c.created_at, c.updated_at,
			(SELECT COUNT(*) FROM comments WHERE card_id = c.id)
		FROM cards c
		JOIN projects p ON c.project_id = p.id
		WHERE c.id = ?
	`, id).Scan(&c.ID, &c.Title, &c.Body, &c.Status, &c.ProjectID, &c.Project,
		&c.CreatedAt, &c.UpdatedAt, &c.CommentCount)
	if err != nil {
		return nil, fmt.Errorf("get card %d: %w", id, err)
	}

	// Load assignees
	rows, err := s.db.Query(
		"SELECT a.name FROM card_assignees ca JOIN agents a ON ca.agent_id = a.id WHERE ca.card_id = ? ORDER BY a.name",
		id,
	)
	if err != nil {
		return nil, fmt.Errorf("load assignees: %w", err)
	}
	defer rows.Close()
	for rows.Next() {
		var name string
		if err := rows.Scan(&name); err != nil {
			return nil, fmt.Errorf("scan assignee: %w", err)
		}
		c.Assignees = append(c.Assignees, name)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("assignee rows: %w", err)
	}

	// Load tags
	tagRows, err := s.db.Query("SELECT tag FROM card_tags WHERE card_id = ? ORDER BY tag", id)
	if err != nil {
		return nil, fmt.Errorf("load tags: %w", err)
	}
	defer tagRows.Close()
	for tagRows.Next() {
		var tag string
		if err := tagRows.Scan(&tag); err != nil {
			return nil, fmt.Errorf("scan tag: %w", err)
		}
		c.Tags = append(c.Tags, tag)
	}
	if err := tagRows.Err(); err != nil {
		return nil, fmt.Errorf("tag rows: %w", err)
	}

	// Load relations
	relRows, err := s.db.Query(
		"SELECT related_card_id, relation_type FROM card_relations WHERE card_id = ? ORDER BY relation_type, related_card_id",
		id,
	)
	if err != nil {
		return nil, fmt.Errorf("load relations: %w", err)
	}
	defer relRows.Close()
	for relRows.Next() {
		var rel model.CardRelation
		if err := relRows.Scan(&rel.RelatedCardID, &rel.RelationType); err != nil {
			return nil, fmt.Errorf("scan relation: %w", err)
		}
		c.Relations = append(c.Relations, rel)
	}
	if err := relRows.Err(); err != nil {
		return nil, fmt.Errorf("relation rows: %w", err)
	}

	return c, nil
}

func (s *Store) ListCards(params CardListParams) ([]model.Card, error) {
	query := `
		SELECT DISTINCT c.id, c.title, COALESCE(c.body, ''), c.status, c.project_id, p.name,
			c.created_at, c.updated_at,
			(SELECT COUNT(*) FROM comments WHERE card_id = c.id)
		FROM cards c
		JOIN projects p ON c.project_id = p.id
	`
	var conditions []string
	var args []any

	if params.Assignee != "" {
		query += " JOIN card_assignees ca ON ca.card_id = c.id JOIN agents a ON ca.agent_id = a.id"
		conditions = append(conditions, "a.name = ?")
		args = append(args, params.Assignee)
	}

	if params.Tag != "" {
		query += " JOIN card_tags ct ON ct.card_id = c.id"
		conditions = append(conditions, "ct.tag = ?")
		args = append(args, params.Tag)
	}

	if params.Project != "" {
		conditions = append(conditions, "p.name = ?")
		args = append(args, params.Project)
	}

	if params.Status != "" {
		// Support comma-separated statuses
		statuses := strings.Split(params.Status, ",")
		placeholders := make([]string, len(statuses))
		for i, st := range statuses {
			placeholders[i] = "?"
			args = append(args, strings.TrimSpace(st))
		}
		conditions = append(conditions, "c.status IN ("+strings.Join(placeholders, ",")+")")
	}

	if len(conditions) > 0 {
		query += " WHERE " + strings.Join(conditions, " AND ")
	}

	query += " ORDER BY c.created_at ASC"

	rows, err := s.db.Query(query, args...)
	if err != nil {
		return nil, fmt.Errorf("list cards: %w", err)
	}
	defer rows.Close()

	var cards []model.Card
	for rows.Next() {
		var c model.Card
		if err := rows.Scan(&c.ID, &c.Title, &c.Body, &c.Status, &c.ProjectID, &c.Project,
			&c.CreatedAt, &c.UpdatedAt, &c.CommentCount); err != nil {
			return nil, fmt.Errorf("scan card: %w", err)
		}
		cards = append(cards, c)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("card rows: %w", err)
	}

	// Load assignees and tags for each card
	for i := range cards {
		aRows, err := s.db.Query(
			"SELECT a.name FROM card_assignees ca JOIN agents a ON ca.agent_id = a.id WHERE ca.card_id = ? ORDER BY a.name",
			cards[i].ID,
		)
		if err != nil {
			return nil, fmt.Errorf("load assignees for card %d: %w", cards[i].ID, err)
		}
		for aRows.Next() {
			var name string
			aRows.Scan(&name)
			cards[i].Assignees = append(cards[i].Assignees, name)
		}
		aRows.Close()

		tRows, err := s.db.Query("SELECT tag FROM card_tags WHERE card_id = ? ORDER BY tag", cards[i].ID)
		if err != nil {
			return nil, fmt.Errorf("load tags for card %d: %w", cards[i].ID, err)
		}
		for tRows.Next() {
			var tag string
			tRows.Scan(&tag)
			cards[i].Tags = append(cards[i].Tags, tag)
		}
		tRows.Close()
	}

	return cards, nil
}
```

- [ ] **Step 4: Run tests to verify they pass**

```bash
go test ./store/ -v
```

Expected: all tests PASS.

- [ ] **Step 5: Commit**

```bash
git add store/card.go store/card_test.go
git commit -m "feat: add card store with CRUD, assignees, tags, relations, and filtered listing"
```

---

## Task 7: Card Status Transitions and Update

**Files:**
- Modify: `store/card.go`
- Modify: `store/card_test.go`

- [ ] **Step 1: Write failing tests for status transitions and update**

Add to `store/card_test.go`:

```go
func TestUpdateCard(t *testing.T) {
	s := setupTestDB(t)
	proj := createTestProject(t, s)

	card, _ := s.CreateCard(CardCreateParams{
		Title: "Original", Status: "considering", ProjectID: proj.ID,
	})

	updated, err := s.UpdateCard(card.ID, CardUpdateParams{
		Title:  strPtr("Updated title"),
		Status: strPtr("todo"),
	})
	if err != nil {
		t.Fatalf("UpdateCard: %v", err)
	}
	if updated.Title != "Updated title" {
		t.Errorf("title = %q, want 'Updated title'", updated.Title)
	}
	if updated.Status != "todo" {
		t.Errorf("status = %q, want 'todo'", updated.Status)
	}
}

func TestUpdateCardInvalidTransition(t *testing.T) {
	s := setupTestDB(t)
	proj := createTestProject(t, s)

	card, _ := s.CreateCard(CardCreateParams{
		Title: "Test", Status: "considering", ProjectID: proj.ID,
	})

	// considering -> in_flight is not valid
	_, err := s.UpdateCard(card.ID, CardUpdateParams{
		Status: strPtr("in_flight"),
	})
	if err == nil {
		t.Error("expected error for invalid transition considering -> in_flight")
	}
}

func TestUpdateCardAddRelations(t *testing.T) {
	s := setupTestDB(t)
	proj := createTestProject(t, s)

	blocker, _ := s.CreateCard(CardCreateParams{
		Title: "Blocker", Status: "todo", ProjectID: proj.ID,
	})
	card, _ := s.CreateCard(CardCreateParams{
		Title: "Blocked", Status: "todo", ProjectID: proj.ID,
	})

	updated, err := s.UpdateCard(card.ID, CardUpdateParams{
		Relations: []model.CardRelation{
			{RelatedCardID: blocker.ID, RelationType: "blocked_by"},
		},
	})
	if err != nil {
		t.Fatalf("UpdateCard: %v", err)
	}
	if len(updated.Relations) != 1 {
		t.Fatalf("relations = %v, want 1 relation", updated.Relations)
	}
}

func strPtr(s string) *string { return &s }
```

- [ ] **Step 2: Run tests to verify they fail**

```bash
go test ./store/ -v -run TestUpdate
```

Expected: FAIL — `UpdateCard`, `CardUpdateParams` not defined.

- [ ] **Step 3: Implement UpdateCard**

Add to `store/card.go`:

```go
type CardUpdateParams struct {
	Title     *string
	Body      *string
	Status    *string
	Assignees []string           // replaces all assignees if non-nil
	Tags      []string           // replaces all tags if non-nil
	Relations []model.CardRelation // appends relations
}

func (s *Store) UpdateCard(id int, p CardUpdateParams) (*model.Card, error) {
	// Validate status transition if status is changing
	if p.Status != nil {
		var currentStatus string
		err := s.db.QueryRow("SELECT status FROM cards WHERE id = ?", id).Scan(&currentStatus)
		if err != nil {
			return nil, fmt.Errorf("get current status: %w", err)
		}
		if currentStatus != *p.Status {
			if !model.CanTransition(currentStatus, *p.Status) {
				allowed := model.AllowedTransitions(currentStatus)
				return nil, fmt.Errorf("invalid transition %q -> %q (allowed: %v)", currentStatus, *p.Status, allowed)
			}
		}
	}

	tx, err := s.db.Begin()
	if err != nil {
		return nil, fmt.Errorf("begin tx: %w", err)
	}
	defer tx.Rollback()

	// Build SET clause dynamically
	var setClauses []string
	var args []any

	if p.Title != nil {
		setClauses = append(setClauses, "title = ?")
		args = append(args, *p.Title)
	}
	if p.Body != nil {
		setClauses = append(setClauses, "body = ?")
		args = append(args, *p.Body)
	}
	if p.Status != nil {
		setClauses = append(setClauses, "status = ?")
		args = append(args, *p.Status)
	}

	if len(setClauses) > 0 {
		setClauses = append(setClauses, "updated_at = datetime('now')")
		args = append(args, id)
		query := "UPDATE cards SET " + strings.Join(setClauses, ", ") + " WHERE id = ?"
		if _, err := tx.Exec(query, args...); err != nil {
			return nil, fmt.Errorf("update card: %w", err)
		}
	}

	// Replace assignees if provided
	if p.Assignees != nil {
		if _, err := tx.Exec("DELETE FROM card_assignees WHERE card_id = ?", id); err != nil {
			return nil, fmt.Errorf("clear assignees: %w", err)
		}
		for _, name := range p.Assignees {
			_, err := tx.Exec(
				"INSERT INTO card_assignees (card_id, agent_id) SELECT ?, id FROM agents WHERE name = ?",
				id, name,
			)
			if err != nil {
				return nil, fmt.Errorf("assign %q: %w", name, err)
			}
		}
	}

	// Replace tags if provided
	if p.Tags != nil {
		if _, err := tx.Exec("DELETE FROM card_tags WHERE card_id = ?", id); err != nil {
			return nil, fmt.Errorf("clear tags: %w", err)
		}
		for _, tag := range p.Tags {
			if _, err := tx.Exec("INSERT INTO card_tags (card_id, tag) VALUES (?, ?)", id, tag); err != nil {
				return nil, fmt.Errorf("add tag %q: %w", tag, err)
			}
		}
	}

	// Append relations
	for _, rel := range p.Relations {
		_, err := tx.Exec(
			"INSERT OR IGNORE INTO card_relations (card_id, related_card_id, relation_type) VALUES (?, ?, ?)",
			id, rel.RelatedCardID, rel.RelationType,
		)
		if err != nil {
			return nil, fmt.Errorf("add relation: %w", err)
		}
	}

	if err := tx.Commit(); err != nil {
		return nil, fmt.Errorf("commit: %w", err)
	}

	return s.GetCard(id)
}
```

- [ ] **Step 4: Run tests to verify they pass**

```bash
go test ./store/ -v
```

Expected: all tests PASS.

- [ ] **Step 5: Commit**

```bash
git add store/card.go store/card_test.go
git commit -m "feat: add card update with status transition validation"
```

---

## Task 8: Comment Store

**Files:**
- Create: `store/comment.go`, `store/comment_test.go`

- [ ] **Step 1: Write failing tests**

```go
// store/comment_test.go
package store

import (
	"testing"
)

func TestCreateAndListComments(t *testing.T) {
	s := setupTestDB(t)
	proj := createTestProject(t, s)
	agent := createTestAgent(t, s, "dev-agent", proj.ID)

	card, _ := s.CreateCard(CardCreateParams{
		Title: "Test card", Status: "todo", ProjectID: proj.ID,
	})

	// Add a comment
	comment, err := s.CreateComment(card.ID, agent.ID, "Started working on this")
	if err != nil {
		t.Fatalf("CreateComment: %v", err)
	}
	if comment.Body != "Started working on this" {
		t.Errorf("body = %q, want 'Started working on this'", comment.Body)
	}
	if comment.Agent != "dev-agent" {
		t.Errorf("agent = %q, want 'dev-agent'", comment.Agent)
	}

	// Add another comment
	s.CreateComment(card.ID, agent.ID, "Making progress")

	// List comments
	comments, err := s.ListComments(card.ID)
	if err != nil {
		t.Fatalf("ListComments: %v", err)
	}
	if len(comments) != 2 {
		t.Fatalf("got %d comments, want 2", len(comments))
	}

	// Verify comment count on card
	got, _ := s.GetCard(card.ID)
	if got.CommentCount != 2 {
		t.Errorf("comment_count = %d, want 2", got.CommentCount)
	}
}
```

- [ ] **Step 2: Run tests to verify they fail**

```bash
go test ./store/ -v -run TestComment
```

Expected: FAIL — methods not defined.

- [ ] **Step 3: Implement comment store**

```go
// store/comment.go
package store

import (
	"fmt"

	"github.com/joelhelbling/kkullm/model"
)

func (s *Store) CreateComment(cardID, agentID int, body string) (*model.Comment, error) {
	result, err := s.db.Exec(
		"INSERT INTO comments (card_id, agent_id, body) VALUES (?, ?, ?)",
		cardID, agentID, body,
	)
	if err != nil {
		return nil, fmt.Errorf("insert comment: %w", err)
	}

	id, err := result.LastInsertId()
	if err != nil {
		return nil, fmt.Errorf("last insert id: %w", err)
	}

	c := &model.Comment{}
	err = s.db.QueryRow(`
		SELECT c.id, c.card_id, c.agent_id, a.name, c.body, c.created_at
		FROM comments c JOIN agents a ON c.agent_id = a.id
		WHERE c.id = ?
	`, id).Scan(&c.ID, &c.CardID, &c.AgentID, &c.Agent, &c.Body, &c.CreatedAt)
	if err != nil {
		return nil, fmt.Errorf("get comment %d: %w", id, err)
	}
	return c, nil
}

func (s *Store) ListComments(cardID int) ([]model.Comment, error) {
	rows, err := s.db.Query(`
		SELECT c.id, c.card_id, c.agent_id, a.name, c.body, c.created_at
		FROM comments c JOIN agents a ON c.agent_id = a.id
		WHERE c.card_id = ?
		ORDER BY c.created_at ASC
	`, cardID)
	if err != nil {
		return nil, fmt.Errorf("list comments: %w", err)
	}
	defer rows.Close()

	var comments []model.Comment
	for rows.Next() {
		var c model.Comment
		if err := rows.Scan(&c.ID, &c.CardID, &c.AgentID, &c.Agent, &c.Body, &c.CreatedAt); err != nil {
			return nil, fmt.Errorf("scan comment: %w", err)
		}
		comments = append(comments, c)
	}
	return comments, rows.Err()
}
```

- [ ] **Step 4: Run tests to verify they pass**

```bash
go test ./store/ -v
```

Expected: all tests PASS.

- [ ] **Step 5: Commit**

```bash
git add store/comment.go store/comment_test.go
git commit -m "feat: add comment store with create and list by card"
```

---

## Task 9: Asset Store with Glob Search

**Files:**
- Create: `store/asset.go`, `store/asset_test.go`

- [ ] **Step 1: Write failing tests**

```go
// store/asset_test.go
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

	// List all for project
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
```

- [ ] **Step 2: Run tests to verify they fail**

```bash
go test ./store/ -v -run TestAsset
```

Expected: FAIL — methods not defined.

- [ ] **Step 3: Implement asset store**

```go
// store/asset.go
package store

import (
	"fmt"
	"strings"

	"github.com/joelhelbling/kkullm/model"
)

type AssetListParams struct {
	Project  string
	NameGlob string
	URLGlob  string
}

func (s *Store) CreateAsset(projectID int, name, description, url string) (*model.ProjectAsset, error) {
	result, err := s.db.Exec(
		"INSERT INTO project_assets (project_id, name, description, url) VALUES (?, ?, ?, ?)",
		projectID, name, nilIfEmpty(description), nilIfEmpty(url),
	)
	if err != nil {
		return nil, fmt.Errorf("insert asset: %w", err)
	}

	id, err := result.LastInsertId()
	if err != nil {
		return nil, fmt.Errorf("last insert id: %w", err)
	}

	return s.GetAsset(int(id))
}

func (s *Store) GetAsset(id int) (*model.ProjectAsset, error) {
	a := &model.ProjectAsset{}
	err := s.db.QueryRow(`
		SELECT pa.id, pa.project_id, p.name, pa.name, COALESCE(pa.description, ''),
			COALESCE(pa.url, ''), pa.created_at, pa.updated_at
		FROM project_assets pa
		JOIN projects p ON pa.project_id = p.id
		WHERE pa.id = ?
	`, id).Scan(&a.ID, &a.ProjectID, &a.Project, &a.Name, &a.Description, &a.URL, &a.CreatedAt, &a.UpdatedAt)
	if err != nil {
		return nil, fmt.Errorf("get asset %d: %w", id, err)
	}
	return a, nil
}

func (s *Store) ListAssets(params AssetListParams) ([]model.ProjectAsset, error) {
	query := `
		SELECT pa.id, pa.project_id, p.name, pa.name, COALESCE(pa.description, ''),
			COALESCE(pa.url, ''), pa.created_at, pa.updated_at
		FROM project_assets pa
		JOIN projects p ON pa.project_id = p.id
	`
	var conditions []string
	var args []any

	if params.Project != "" {
		conditions = append(conditions, "p.name = ?")
		args = append(args, params.Project)
	}
	if params.NameGlob != "" {
		conditions = append(conditions, "pa.name GLOB ?")
		args = append(args, params.NameGlob)
	}
	if params.URLGlob != "" {
		conditions = append(conditions, "pa.url GLOB ?")
		args = append(args, params.URLGlob)
	}

	if len(conditions) > 0 {
		query += " WHERE " + strings.Join(conditions, " AND ")
	}
	query += " ORDER BY pa.name"

	rows, err := s.db.Query(query, args...)
	if err != nil {
		return nil, fmt.Errorf("list assets: %w", err)
	}
	defer rows.Close()

	var assets []model.ProjectAsset
	for rows.Next() {
		var a model.ProjectAsset
		if err := rows.Scan(&a.ID, &a.ProjectID, &a.Project, &a.Name, &a.Description,
			&a.URL, &a.CreatedAt, &a.UpdatedAt); err != nil {
			return nil, fmt.Errorf("scan asset: %w", err)
		}
		assets = append(assets, a)
	}
	return assets, rows.Err()
}

func nilIfEmpty(s string) *string {
	if s == "" {
		return nil
	}
	return &s
}
```

- [ ] **Step 4: Run tests to verify they pass**

```bash
go test ./store/ -v
```

Expected: all tests PASS.

- [ ] **Step 5: Commit**

```bash
git add store/asset.go store/asset_test.go
git commit -m "feat: add asset store with CRUD and glob pattern search"
```

---

## Task 10: REST API Server and Project/Agent Handlers

**Files:**
- Create: `api/server.go`, `api/projects.go`, `api/agents.go`, `api/server_test.go`

- [ ] **Step 1: Write failing integration test**

```go
// api/server_test.go
package api

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/joelhelbling/kkullm/db"
	"github.com/joelhelbling/kkullm/model"
	"github.com/joelhelbling/kkullm/store"
)

func setupTestServer(t *testing.T) *httptest.Server {
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

	s := store.New(database)
	srv := NewServer(s)
	return httptest.NewServer(srv.Handler())
}

func TestListProjects(t *testing.T) {
	ts := setupTestServer(t)
	defer ts.Close()

	resp, err := http.Get(ts.URL + "/api/projects")
	if err != nil {
		t.Fatalf("GET /api/projects: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != 200 {
		t.Fatalf("status = %d, want 200", resp.StatusCode)
	}

	var projects []model.Project
	if err := json.NewDecoder(resp.Body).Decode(&projects); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if len(projects) != 1 {
		t.Errorf("got %d projects, want 1 (orchestration)", len(projects))
	}
}

func TestCreateProject(t *testing.T) {
	ts := setupTestServer(t)
	defer ts.Close()

	body := `{"name":"acme-backend","description":"Backend service"}`
	resp, err := http.Post(ts.URL+"/api/projects", "application/json", strings.NewReader(body))
	if err != nil {
		t.Fatalf("POST /api/projects: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != 201 {
		t.Fatalf("status = %d, want 201", resp.StatusCode)
	}

	var project model.Project
	if err := json.NewDecoder(resp.Body).Decode(&project); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if project.Name != "acme-backend" {
		t.Errorf("name = %q, want 'acme-backend'", project.Name)
	}
}

func TestCreateAndListAgents(t *testing.T) {
	ts := setupTestServer(t)
	defer ts.Close()

	// Create a project first
	projBody := `{"name":"acme"}`
	resp, _ := http.Post(ts.URL+"/api/projects", "application/json", strings.NewReader(projBody))
	var proj model.Project
	json.NewDecoder(resp.Body).Decode(&proj)
	resp.Body.Close()

	// Create an agent
	agentBody := `{"name":"dev-agent","project":"acme","bio":"Writes Go"}`
	resp, err := http.Post(ts.URL+"/api/agents", "application/json", strings.NewReader(agentBody))
	if err != nil {
		t.Fatalf("POST /api/agents: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != 201 {
		t.Fatalf("status = %d, want 201", resp.StatusCode)
	}

	var agent model.Agent
	json.NewDecoder(resp.Body).Decode(&agent)
	if agent.Name != "dev-agent" {
		t.Errorf("name = %q, want 'dev-agent'", agent.Name)
	}

	// List agents filtered by project
	resp2, _ := http.Get(ts.URL + "/api/agents?project=acme")
	defer resp2.Body.Close()
	var agents []model.Agent
	json.NewDecoder(resp2.Body).Decode(&agents)
	if len(agents) != 1 {
		t.Errorf("got %d agents for acme, want 1", len(agents))
	}
}
```

- [ ] **Step 2: Run tests to verify they fail**

```bash
go test ./api/ -v
```

Expected: FAIL — `NewServer` not defined.

- [ ] **Step 3: Implement server and project handlers**

```go
// api/server.go
package api

import (
	"encoding/json"
	"net/http"

	"github.com/joelhelbling/kkullm/store"
)

type Server struct {
	store *store.Store
}

func NewServer(s *store.Store) *Server {
	return &Server{store: s}
}

func (s *Server) Handler() http.Handler {
	mux := http.NewServeMux()

	// Projects
	mux.HandleFunc("GET /api/projects", s.listProjects)
	mux.HandleFunc("POST /api/projects", s.createProject)
	mux.HandleFunc("GET /api/projects/{id}", s.getProject)

	// Agents
	mux.HandleFunc("GET /api/agents", s.listAgents)
	mux.HandleFunc("POST /api/agents", s.createAgent)
	mux.HandleFunc("GET /api/agents/{id}", s.getAgent)

	return mux
}

func writeJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(v)
}

func writeError(w http.ResponseWriter, status int, msg string) {
	writeJSON(w, status, map[string]string{"error": msg})
}
```

```go
// api/projects.go
package api

import (
	"encoding/json"
	"net/http"
	"strconv"
)

func (s *Server) listProjects(w http.ResponseWriter, r *http.Request) {
	projects, err := s.store.ListProjects()
	if err != nil {
		writeError(w, 500, err.Error())
		return
	}
	writeJSON(w, 200, projects)
}

func (s *Server) createProject(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Name        string `json:"name"`
		Description string `json:"description"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, 400, "invalid JSON")
		return
	}
	if req.Name == "" {
		writeError(w, 400, "name is required")
		return
	}

	project, err := s.store.CreateProject(req.Name, req.Description)
	if err != nil {
		writeError(w, 422, err.Error())
		return
	}
	writeJSON(w, 201, project)
}

func (s *Server) getProject(w http.ResponseWriter, r *http.Request) {
	id, err := strconv.Atoi(r.PathValue("id"))
	if err != nil {
		writeError(w, 400, "invalid id")
		return
	}

	project, err := s.store.GetProject(id)
	if err != nil {
		writeError(w, 404, "project not found")
		return
	}
	writeJSON(w, 200, project)
}
```

```go
// api/agents.go
package api

import (
	"encoding/json"
	"net/http"
	"strconv"
)

func (s *Server) listAgents(w http.ResponseWriter, r *http.Request) {
	project := r.URL.Query().Get("project")
	agents, err := s.store.ListAgents(project)
	if err != nil {
		writeError(w, 500, err.Error())
		return
	}
	writeJSON(w, 200, agents)
}

func (s *Server) createAgent(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Name    string `json:"name"`
		Project string `json:"project"`
		Bio     string `json:"bio"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, 400, "invalid JSON")
		return
	}
	if req.Name == "" || req.Project == "" {
		writeError(w, 400, "name and project are required")
		return
	}

	proj, err := s.store.GetProjectByName(req.Project)
	if err != nil {
		writeError(w, 404, "project not found")
		return
	}

	agent, err := s.store.CreateAgent(req.Name, proj.ID, req.Bio)
	if err != nil {
		writeError(w, 422, err.Error())
		return
	}
	writeJSON(w, 201, agent)
}

func (s *Server) getAgent(w http.ResponseWriter, r *http.Request) {
	id, err := strconv.Atoi(r.PathValue("id"))
	if err != nil {
		writeError(w, 400, "invalid id")
		return
	}

	agent, err := s.store.GetAgent(id)
	if err != nil {
		writeError(w, 404, "agent not found")
		return
	}
	writeJSON(w, 200, agent)
}
```

- [ ] **Step 4: Run tests to verify they pass**

```bash
go test ./api/ -v
```

Expected: all tests PASS.

- [ ] **Step 5: Commit**

```bash
git add api/
git commit -m "feat: add REST API server with project and agent handlers"
```

---

## Task 11: Card and Comment API Handlers

**Files:**
- Create: `api/cards.go`, `api/comments.go`
- Modify: `api/server.go` (add routes)
- Modify: `api/server_test.go` (add tests)

- [ ] **Step 1: Write failing tests for card and comment endpoints**

Add to `api/server_test.go`:

```go
func TestCardCRUD(t *testing.T) {
	ts := setupTestServer(t)
	defer ts.Close()

	// Create project
	resp, _ := http.Post(ts.URL+"/api/projects", "application/json", strings.NewReader(`{"name":"acme"}`))
	resp.Body.Close()

	// Create agent
	resp, _ = http.Post(ts.URL+"/api/agents", "application/json", strings.NewReader(`{"name":"dev","project":"acme"}`))
	resp.Body.Close()

	// Create card with assignees, tags, relations
	cardBody := `{
		"title":"Implement auth",
		"body":"Add JWT",
		"status":"todo",
		"project":"acme",
		"assignees":["dev"],
		"tags":["auth","backend"]
	}`
	resp, err := http.Post(ts.URL+"/api/cards", "application/json", strings.NewReader(cardBody))
	if err != nil {
		t.Fatalf("POST /api/cards: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != 201 {
		t.Fatalf("create status = %d, want 201", resp.StatusCode)
	}

	var card model.Card
	json.NewDecoder(resp.Body).Decode(&card)
	if card.Title != "Implement auth" {
		t.Errorf("title = %q, want 'Implement auth'", card.Title)
	}
	if len(card.Assignees) != 1 {
		t.Errorf("assignees = %v, want [dev]", card.Assignees)
	}
	if len(card.Tags) != 2 {
		t.Errorf("tags = %v, want [auth backend]", card.Tags)
	}

	// Update card status
	req, _ := http.NewRequest("PATCH", ts.URL+"/api/cards/"+strconv.Itoa(card.ID), strings.NewReader(`{"status":"in_flight"}`))
	req.Header.Set("Content-Type", "application/json")
	resp2, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("PATCH /api/cards: %v", err)
	}
	defer resp2.Body.Close()

	if resp2.StatusCode != 200 {
		t.Fatalf("update status = %d, want 200", resp2.StatusCode)
	}

	var updated model.Card
	json.NewDecoder(resp2.Body).Decode(&updated)
	if updated.Status != "in_flight" {
		t.Errorf("status = %q, want 'in_flight'", updated.Status)
	}

	// List cards filtered by status
	resp3, _ := http.Get(ts.URL + "/api/cards?status=in_flight")
	defer resp3.Body.Close()
	var cards []model.Card
	json.NewDecoder(resp3.Body).Decode(&cards)
	if len(cards) != 1 {
		t.Errorf("got %d in_flight cards, want 1", len(cards))
	}
}

func TestCardComments(t *testing.T) {
	ts := setupTestServer(t)
	defer ts.Close()

	// Setup: project + agent + card
	http.Post(ts.URL+"/api/projects", "application/json", strings.NewReader(`{"name":"acme"}`))
	http.Post(ts.URL+"/api/agents", "application/json", strings.NewReader(`{"name":"dev","project":"acme"}`))
	resp, _ := http.Post(ts.URL+"/api/cards", "application/json", strings.NewReader(`{"title":"Test","status":"todo","project":"acme"}`))
	var card model.Card
	json.NewDecoder(resp.Body).Decode(&card)
	resp.Body.Close()

	// Add comment
	commentBody := `{"agent":"dev","body":"Starting work"}`
	resp2, err := http.Post(ts.URL+"/api/cards/"+strconv.Itoa(card.ID)+"/comments", "application/json", strings.NewReader(commentBody))
	if err != nil {
		t.Fatalf("POST comments: %v", err)
	}
	defer resp2.Body.Close()

	if resp2.StatusCode != 201 {
		t.Fatalf("status = %d, want 201", resp2.StatusCode)
	}

	// List comments
	resp3, _ := http.Get(ts.URL + "/api/cards/" + strconv.Itoa(card.ID) + "/comments")
	defer resp3.Body.Close()
	var comments []model.Comment
	json.NewDecoder(resp3.Body).Decode(&comments)
	if len(comments) != 1 {
		t.Errorf("got %d comments, want 1", len(comments))
	}
}
```

- [ ] **Step 2: Run tests to verify they fail**

```bash
go test ./api/ -v -run TestCard
```

Expected: FAIL — card routes not registered.

- [ ] **Step 3: Implement card handlers**

```go
// api/cards.go
package api

import (
	"encoding/json"
	"net/http"
	"strconv"

	"github.com/joelhelbling/kkullm/model"
	"github.com/joelhelbling/kkullm/store"
)

func (s *Server) listCards(w http.ResponseWriter, r *http.Request) {
	params := store.CardListParams{
		Project:  r.URL.Query().Get("project"),
		Assignee: r.URL.Query().Get("assignee"),
		Status:   r.URL.Query().Get("status"),
		Tag:      r.URL.Query().Get("tag"),
	}
	cards, err := s.store.ListCards(params)
	if err != nil {
		writeError(w, 500, err.Error())
		return
	}
	if cards == nil {
		cards = []model.Card{}
	}
	writeJSON(w, 200, cards)
}

func (s *Server) createCard(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Title     string             `json:"title"`
		Body      string             `json:"body"`
		Status    string             `json:"status"`
		Project   string             `json:"project"`
		Assignees []string           `json:"assignees"`
		Tags      []string           `json:"tags"`
		Relations []model.CardRelation `json:"relations"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, 400, "invalid JSON")
		return
	}
	if req.Title == "" || req.Project == "" {
		writeError(w, 400, "title and project are required")
		return
	}

	proj, err := s.store.GetProjectByName(req.Project)
	if err != nil {
		writeError(w, 404, "project not found")
		return
	}

	card, err := s.store.CreateCard(store.CardCreateParams{
		Title:     req.Title,
		Body:      req.Body,
		Status:    req.Status,
		ProjectID: proj.ID,
		Assignees: req.Assignees,
		Tags:      req.Tags,
		Relations: req.Relations,
	})
	if err != nil {
		writeError(w, 422, err.Error())
		return
	}
	writeJSON(w, 201, card)
}

func (s *Server) getCard(w http.ResponseWriter, r *http.Request) {
	id, err := strconv.Atoi(r.PathValue("id"))
	if err != nil {
		writeError(w, 400, "invalid id")
		return
	}
	card, err := s.store.GetCard(id)
	if err != nil {
		writeError(w, 404, "card not found")
		return
	}
	writeJSON(w, 200, card)
}

func (s *Server) updateCard(w http.ResponseWriter, r *http.Request) {
	id, err := strconv.Atoi(r.PathValue("id"))
	if err != nil {
		writeError(w, 400, "invalid id")
		return
	}

	var req struct {
		Title     *string            `json:"title"`
		Body      *string            `json:"body"`
		Status    *string            `json:"status"`
		Assignees []string           `json:"assignees"`
		Tags      []string           `json:"tags"`
		Relations []model.CardRelation `json:"relations"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, 400, "invalid JSON")
		return
	}

	card, err := s.store.UpdateCard(id, store.CardUpdateParams{
		Title:     req.Title,
		Body:      req.Body,
		Status:    req.Status,
		Assignees: req.Assignees,
		Tags:      req.Tags,
		Relations: req.Relations,
	})
	if err != nil {
		writeError(w, 422, err.Error())
		return
	}
	writeJSON(w, 200, card)
}

func (s *Server) deleteCard(w http.ResponseWriter, r *http.Request) {
	id, err := strconv.Atoi(r.PathValue("id"))
	if err != nil {
		writeError(w, 400, "invalid id")
		return
	}
	// Simple delete — no soft-delete for v1
	_, err = s.store.GetCard(id)
	if err != nil {
		writeError(w, 404, "card not found")
		return
	}
	if err := s.store.DeleteCard(id); err != nil {
		writeError(w, 500, err.Error())
		return
	}
	w.WriteHeader(204)
}
```

```go
// api/comments.go
package api

import (
	"encoding/json"
	"net/http"
	"strconv"
)

func (s *Server) listComments(w http.ResponseWriter, r *http.Request) {
	cardID, err := strconv.Atoi(r.PathValue("id"))
	if err != nil {
		writeError(w, 400, "invalid card id")
		return
	}
	comments, err := s.store.ListComments(cardID)
	if err != nil {
		writeError(w, 500, err.Error())
		return
	}
	writeJSON(w, 200, comments)
}

func (s *Server) createComment(w http.ResponseWriter, r *http.Request) {
	cardID, err := strconv.Atoi(r.PathValue("id"))
	if err != nil {
		writeError(w, 400, "invalid card id")
		return
	}

	var req struct {
		Agent string `json:"agent"`
		Body  string `json:"body"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, 400, "invalid JSON")
		return
	}
	if req.Agent == "" || req.Body == "" {
		writeError(w, 400, "agent and body are required")
		return
	}

	agent, err := s.store.GetAgentByName(req.Agent)
	if err != nil {
		writeError(w, 404, "agent not found")
		return
	}

	comment, err := s.store.CreateComment(cardID, agent.ID, req.Body)
	if err != nil {
		writeError(w, 422, err.Error())
		return
	}
	writeJSON(w, 201, comment)
}
```

- [ ] **Step 4: Register card, comment, and asset routes in server.go**

Update `api/server.go` `Handler()` method to add the remaining routes:

```go
func (s *Server) Handler() http.Handler {
	mux := http.NewServeMux()

	// Projects
	mux.HandleFunc("GET /api/projects", s.listProjects)
	mux.HandleFunc("POST /api/projects", s.createProject)
	mux.HandleFunc("GET /api/projects/{id}", s.getProject)

	// Agents
	mux.HandleFunc("GET /api/agents", s.listAgents)
	mux.HandleFunc("POST /api/agents", s.createAgent)
	mux.HandleFunc("GET /api/agents/{id}", s.getAgent)

	// Cards
	mux.HandleFunc("GET /api/cards", s.listCards)
	mux.HandleFunc("POST /api/cards", s.createCard)
	mux.HandleFunc("GET /api/cards/{id}", s.getCard)
	mux.HandleFunc("PATCH /api/cards/{id}", s.updateCard)
	mux.HandleFunc("DELETE /api/cards/{id}", s.deleteCard)

	// Comments
	mux.HandleFunc("GET /api/cards/{id}/comments", s.listComments)
	mux.HandleFunc("POST /api/cards/{id}/comments", s.createComment)

	// Assets
	mux.HandleFunc("GET /api/assets", s.listAssets)
	mux.HandleFunc("POST /api/assets", s.createAsset)
	mux.HandleFunc("GET /api/assets/{id}", s.getAsset)

	return mux
}
```

- [ ] **Step 5: Implement asset handlers**

```go
// api/assets.go
package api

import (
	"encoding/json"
	"net/http"
	"strconv"

	"github.com/joelhelbling/kkullm/store"
)

func (s *Server) listAssets(w http.ResponseWriter, r *http.Request) {
	params := store.AssetListParams{
		Project:  r.URL.Query().Get("project"),
		NameGlob: r.URL.Query().Get("name"),
		URLGlob:  r.URL.Query().Get("url"),
	}
	assets, err := s.store.ListAssets(params)
	if err != nil {
		writeError(w, 500, err.Error())
		return
	}
	writeJSON(w, 200, assets)
}

func (s *Server) createAsset(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Project     string `json:"project"`
		Name        string `json:"name"`
		Description string `json:"description"`
		URL         string `json:"url"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, 400, "invalid JSON")
		return
	}
	if req.Name == "" || req.Project == "" {
		writeError(w, 400, "name and project are required")
		return
	}

	proj, err := s.store.GetProjectByName(req.Project)
	if err != nil {
		writeError(w, 404, "project not found")
		return
	}

	asset, err := s.store.CreateAsset(proj.ID, req.Name, req.Description, req.URL)
	if err != nil {
		writeError(w, 422, err.Error())
		return
	}
	writeJSON(w, 201, asset)
}

func (s *Server) getAsset(w http.ResponseWriter, r *http.Request) {
	id, err := strconv.Atoi(r.PathValue("id"))
	if err != nil {
		writeError(w, 400, "invalid id")
		return
	}
	asset, err := s.store.GetAsset(id)
	if err != nil {
		writeError(w, 404, "asset not found")
		return
	}
	writeJSON(w, 200, asset)
}
```

- [ ] **Step 6: Add DeleteCard to the store**

Add to `store/card.go`:

```go
func (s *Store) DeleteCard(id int) error {
	_, err := s.db.Exec("DELETE FROM cards WHERE id = ?", id)
	if err != nil {
		return fmt.Errorf("delete card %d: %w", id, err)
	}
	return nil
}
```

- [ ] **Step 7: Run all tests**

```bash
go test ./... -v
```

Expected: all tests PASS across all packages.

- [ ] **Step 8: Commit**

```bash
git add api/ store/card.go
git commit -m "feat: add REST API handlers for cards, comments, and assets"
```

---

## Task 12: SSE Event Stream

**Files:**
- Create: `api/sse.go`
- Modify: `api/server.go` (register route, add event bus)
- Modify: `api/cards.go`, `api/comments.go` (emit events on changes)
- Modify: `api/server_test.go`

- [ ] **Step 1: Write failing test for SSE**

Add to `api/server_test.go`:

```go
func TestSSEStream(t *testing.T) {
	ts := setupTestServer(t)
	defer ts.Close()

	// Connect to SSE stream
	resp, err := http.Get(ts.URL + "/api/events")
	if err != nil {
		t.Fatalf("GET /api/events: %v", err)
	}

	if resp.Header.Get("Content-Type") != "text/event-stream" {
		t.Errorf("content-type = %q, want 'text/event-stream'", resp.Header.Get("Content-Type"))
	}

	resp.Body.Close()
}
```

- [ ] **Step 2: Run test to verify it fails**

```bash
go test ./api/ -v -run TestSSE
```

Expected: FAIL — route not registered.

- [ ] **Step 3: Implement SSE event bus and handler**

```go
// api/sse.go
package api

import (
	"encoding/json"
	"fmt"
	"net/http"
	"sync"
)

type Event struct {
	Type string `json:"type"` // card_created, card_updated, comment_created
	Data any    `json:"data"`
}

type EventBus struct {
	mu      sync.RWMutex
	clients map[chan Event]struct{}
}

func NewEventBus() *EventBus {
	return &EventBus{
		clients: make(map[chan Event]struct{}),
	}
}

func (eb *EventBus) Subscribe() chan Event {
	ch := make(chan Event, 16)
	eb.mu.Lock()
	eb.clients[ch] = struct{}{}
	eb.mu.Unlock()
	return ch
}

func (eb *EventBus) Unsubscribe(ch chan Event) {
	eb.mu.Lock()
	delete(eb.clients, ch)
	eb.mu.Unlock()
	close(ch)
}

func (eb *EventBus) Publish(e Event) {
	eb.mu.RLock()
	defer eb.mu.RUnlock()
	for ch := range eb.clients {
		select {
		case ch <- e:
		default:
			// Drop event if client is slow
		}
	}
}

func (s *Server) handleSSE(w http.ResponseWriter, r *http.Request) {
	flusher, ok := w.(http.Flusher)
	if !ok {
		writeError(w, 500, "streaming not supported")
		return
	}

	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("Connection", "keep-alive")
	w.WriteHeader(200)
	flusher.Flush()

	ch := s.events.Subscribe()
	defer s.events.Unsubscribe(ch)

	for {
		select {
		case <-r.Context().Done():
			return
		case event := <-ch:
			data, _ := json.Marshal(event)
			fmt.Fprintf(w, "event: %s\ndata: %s\n\n", event.Type, data)
			flusher.Flush()
		}
	}
}
```

- [ ] **Step 4: Update Server struct and Handler to include EventBus**

Replace `api/server.go` with:

```go
// api/server.go
package api

import (
	"encoding/json"
	"net/http"

	"github.com/joelhelbling/kkullm/store"
)

type Server struct {
	store  *store.Store
	events *EventBus
}

func NewServer(s *store.Store) *Server {
	return &Server{
		store:  s,
		events: NewEventBus(),
	}
}

func (s *Server) Handler() http.Handler {
	mux := http.NewServeMux()

	// Projects
	mux.HandleFunc("GET /api/projects", s.listProjects)
	mux.HandleFunc("POST /api/projects", s.createProject)
	mux.HandleFunc("GET /api/projects/{id}", s.getProject)

	// Agents
	mux.HandleFunc("GET /api/agents", s.listAgents)
	mux.HandleFunc("POST /api/agents", s.createAgent)
	mux.HandleFunc("GET /api/agents/{id}", s.getAgent)

	// Cards
	mux.HandleFunc("GET /api/cards", s.listCards)
	mux.HandleFunc("POST /api/cards", s.createCard)
	mux.HandleFunc("GET /api/cards/{id}", s.getCard)
	mux.HandleFunc("PATCH /api/cards/{id}", s.updateCard)
	mux.HandleFunc("DELETE /api/cards/{id}", s.deleteCard)

	// Comments
	mux.HandleFunc("GET /api/cards/{id}/comments", s.listComments)
	mux.HandleFunc("POST /api/cards/{id}/comments", s.createComment)

	// Assets
	mux.HandleFunc("GET /api/assets", s.listAssets)
	mux.HandleFunc("POST /api/assets", s.createAsset)
	mux.HandleFunc("GET /api/assets/{id}", s.getAsset)

	// SSE
	mux.HandleFunc("GET /api/events", s.handleSSE)

	return mux
}

func writeJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(v)
}

func writeError(w http.ResponseWriter, status int, msg string) {
	writeJSON(w, status, map[string]string{"error": msg})
}
```

- [ ] **Step 5: Add event publishing to card and comment handlers**

In `api/cards.go`, add after successful create/update/delete:

```go
// In createCard, after writeJSON:
s.events.Publish(Event{Type: "card_created", Data: card})

// In updateCard, after writeJSON:
s.events.Publish(Event{Type: "card_updated", Data: card})

// In deleteCard, before w.WriteHeader(204):
s.events.Publish(Event{Type: "card_deleted", Data: map[string]int{"id": id}})
```

In `api/comments.go`, add after successful create:

```go
// In createComment, after writeJSON:
s.events.Publish(Event{Type: "comment_created", Data: comment})
```

- [ ] **Step 6: Run all tests**

```bash
go test ./... -v
```

Expected: all tests PASS.

- [ ] **Step 7: Commit**

```bash
git add api/
git commit -m "feat: add SSE event stream with pub/sub for real-time updates"
```

---

## Task 13: HTTP Client for CLI

**Files:**
- Create: `client/client.go`

- [ ] **Step 1: Implement the HTTP client**

```go
// client/client.go
package client

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"

	"github.com/joelhelbling/kkullm/model"
)

type Client struct {
	BaseURL    string
	HTTPClient *http.Client
}

func New(baseURL string) *Client {
	return &Client{
		BaseURL:    baseURL,
		HTTPClient: &http.Client{},
	}
}

func (c *Client) do(method, path string, body any, result any) error {
	var bodyReader io.Reader
	if body != nil {
		data, err := json.Marshal(body)
		if err != nil {
			return fmt.Errorf("marshal request: %w", err)
		}
		bodyReader = bytes.NewReader(data)
	}

	req, err := http.NewRequest(method, c.BaseURL+path, bodyReader)
	if err != nil {
		return fmt.Errorf("create request: %w", err)
	}
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}

	resp, err := c.HTTPClient.Do(req)
	if err != nil {
		return fmt.Errorf("request failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode >= 400 {
		var errResp struct {
			Error string `json:"error"`
		}
		json.NewDecoder(resp.Body).Decode(&errResp)
		return fmt.Errorf("server error (%d): %s", resp.StatusCode, errResp.Error)
	}

	if result != nil && resp.StatusCode != 204 {
		if err := json.NewDecoder(resp.Body).Decode(result); err != nil {
			return fmt.Errorf("decode response: %w", err)
		}
	}
	return nil
}

// Projects

func (c *Client) ListProjects() ([]model.Project, error) {
	var projects []model.Project
	err := c.do("GET", "/api/projects", nil, &projects)
	return projects, err
}

func (c *Client) CreateProject(name, description string) (*model.Project, error) {
	var project model.Project
	err := c.do("POST", "/api/projects", map[string]string{
		"name": name, "description": description,
	}, &project)
	return &project, err
}

// Agents

func (c *Client) ListAgents(project string) ([]model.Agent, error) {
	path := "/api/agents"
	if project != "" {
		path += "?project=" + url.QueryEscape(project)
	}
	var agents []model.Agent
	err := c.do("GET", path, nil, &agents)
	return agents, err
}

func (c *Client) CreateAgent(name, project, bio string) (*model.Agent, error) {
	var agent model.Agent
	err := c.do("POST", "/api/agents", map[string]string{
		"name": name, "project": project, "bio": bio,
	}, &agent)
	return &agent, err
}

func (c *Client) GetAgent(id int) (*model.Agent, error) {
	var agent model.Agent
	err := c.do("GET", fmt.Sprintf("/api/agents/%d", id), nil, &agent)
	return &agent, err
}

// Cards

type CardCreateRequest struct {
	Title     string             `json:"title"`
	Body      string             `json:"body,omitempty"`
	Status    string             `json:"status,omitempty"`
	Project   string             `json:"project"`
	Assignees []string           `json:"assignees,omitempty"`
	Tags      []string           `json:"tags,omitempty"`
	Relations []model.CardRelation `json:"relations,omitempty"`
}

type CardUpdateRequest struct {
	Title     *string            `json:"title,omitempty"`
	Body      *string            `json:"body,omitempty"`
	Status    *string            `json:"status,omitempty"`
	Assignees []string           `json:"assignees,omitempty"`
	Tags      []string           `json:"tags,omitempty"`
	Relations []model.CardRelation `json:"relations,omitempty"`
}

func (c *Client) ListCards(project, assignee, status, tag string) ([]model.Card, error) {
	params := url.Values{}
	if project != "" {
		params.Set("project", project)
	}
	if assignee != "" {
		params.Set("assignee", assignee)
	}
	if status != "" {
		params.Set("status", status)
	}
	if tag != "" {
		params.Set("tag", tag)
	}
	path := "/api/cards"
	if len(params) > 0 {
		path += "?" + params.Encode()
	}
	var cards []model.Card
	err := c.do("GET", path, nil, &cards)
	return cards, err
}

func (c *Client) GetCard(id int) (*model.Card, error) {
	var card model.Card
	err := c.do("GET", fmt.Sprintf("/api/cards/%d", id), nil, &card)
	return &card, err
}

func (c *Client) CreateCard(req CardCreateRequest) (*model.Card, error) {
	var card model.Card
	err := c.do("POST", "/api/cards", req, &card)
	return &card, err
}

func (c *Client) UpdateCard(id int, req CardUpdateRequest) (*model.Card, error) {
	var card model.Card
	err := c.do("PATCH", fmt.Sprintf("/api/cards/%d", id), req, &card)
	return &card, err
}

// Comments

func (c *Client) ListComments(cardID int) ([]model.Comment, error) {
	var comments []model.Comment
	err := c.do("GET", fmt.Sprintf("/api/cards/%d/comments", cardID), nil, &comments)
	return comments, err
}

func (c *Client) CreateComment(cardID int, agent, body string) (*model.Comment, error) {
	var comment model.Comment
	err := c.do("POST", fmt.Sprintf("/api/cards/%d/comments", cardID), map[string]string{
		"agent": agent, "body": body,
	}, &comment)
	return &comment, err
}

// Assets

func (c *Client) ListAssets(project, nameGlob, urlGlob string) ([]model.ProjectAsset, error) {
	params := url.Values{}
	if project != "" {
		params.Set("project", project)
	}
	if nameGlob != "" {
		params.Set("name", nameGlob)
	}
	if urlGlob != "" {
		params.Set("url", urlGlob)
	}
	path := "/api/assets"
	if len(params) > 0 {
		path += "?" + params.Encode()
	}
	var assets []model.ProjectAsset
	err := c.do("GET", path, nil, &assets)
	return assets, err
}

func (c *Client) CreateAsset(project, name, description, assetURL string) (*model.ProjectAsset, error) {
	var asset model.ProjectAsset
	err := c.do("POST", "/api/assets", map[string]string{
		"project": project, "name": name, "description": description, "url": assetURL,
	}, &asset)
	return &asset, err
}

func (c *Client) GetAsset(id int) (*model.ProjectAsset, error) {
	var asset model.ProjectAsset
	err := c.do("GET", fmt.Sprintf("/api/assets/%d", id), nil, &asset)
	return &asset, err
}
```

- [ ] **Step 2: Verify it compiles**

```bash
go build ./client/
```

Expected: no errors.

- [ ] **Step 3: Commit**

```bash
git add client/
git commit -m "feat: add HTTP client library for CLI commands"
```

---

## Task 14: CLI Commands — serve, project, agent

**Files:**
- Create: `cmd/serve.go`, `cmd/project.go`, `cmd/agent.go`

- [ ] **Step 1: Implement serve command**

```go
// cmd/serve.go
package cmd

import (
	"fmt"
	"net/http"
	"os"

	"github.com/joelhelbling/kkullm/api"
	"github.com/joelhelbling/kkullm/db"
	"github.com/joelhelbling/kkullm/store"
	"github.com/spf13/cobra"
)

var (
	serveAddr string
	dbPath    string
)

var serveCmd = &cobra.Command{
	Use:   "serve",
	Short: "Start the Kkullm server",
	RunE: func(cmd *cobra.Command, args []string) error {
		database, err := db.Open(dbPath)
		if err != nil {
			return fmt.Errorf("open database: %w", err)
		}
		defer database.Close()

		if err := db.Migrate(database); err != nil {
			return fmt.Errorf("migrate: %w", err)
		}
		if err := db.Seed(database); err != nil {
			return fmt.Errorf("seed: %w", err)
		}

		s := store.New(database)
		srv := api.NewServer(s)

		fmt.Fprintf(os.Stderr, "Kkullm server listening on %s\n", serveAddr)
		return http.ListenAndServe(serveAddr, srv.Handler())
	},
}

func init() {
	serveCmd.Flags().StringVar(&serveAddr, "addr", ":8080", "Listen address")
	serveCmd.Flags().StringVar(&dbPath, "db", "kkullm.db", "Database file path")
	rootCmd.AddCommand(serveCmd)
}
```

- [ ] **Step 2: Implement project commands**

```go
// cmd/project.go
package cmd

import (
	"fmt"

	"github.com/joelhelbling/kkullm/client"
	"github.com/spf13/cobra"
)

var projectCmd = &cobra.Command{
	Use:   "project",
	Short: "Manage projects",
}

var projectListCmd = &cobra.Command{
	Use:   "list",
	Short: "List projects",
	RunE: func(cmd *cobra.Command, args []string) error {
		c := client.New(serverURL)
		projects, err := c.ListProjects()
		if err != nil {
			return err
		}
		for _, p := range projects {
			if p.Description != "" {
				fmt.Printf("%s — %s\n", p.Name, p.Description)
			} else {
				fmt.Println(p.Name)
			}
		}
		return nil
	},
}

var projectCreateName string
var projectCreateDesc string

var projectCreateCmd = &cobra.Command{
	Use:   "create",
	Short: "Create a project",
	RunE: func(cmd *cobra.Command, args []string) error {
		c := client.New(serverURL)
		project, err := c.CreateProject(projectCreateName, projectCreateDesc)
		if err != nil {
			return err
		}
		fmt.Printf("Created project %q (id: %d)\n", project.Name, project.ID)
		return nil
	},
}

func init() {
	projectCreateCmd.Flags().StringVar(&projectCreateName, "name", "", "Project name")
	projectCreateCmd.Flags().StringVar(&projectCreateDesc, "description", "", "Project description")
	projectCreateCmd.MarkFlagRequired("name")

	projectCmd.AddCommand(projectListCmd, projectCreateCmd)
	rootCmd.AddCommand(projectCmd)
}
```

- [ ] **Step 3: Implement agent commands**

```go
// cmd/agent.go
package cmd

import (
	"fmt"

	"github.com/joelhelbling/kkullm/client"
	"github.com/spf13/cobra"
)

var agentCmd = &cobra.Command{
	Use:   "agent",
	Short: "Manage agents",
}

var agentListCmd = &cobra.Command{
	Use:   "list",
	Short: "List agents",
	RunE: func(cmd *cobra.Command, args []string) error {
		c := client.New(serverURL)
		agents, err := c.ListAgents(projectName)
		if err != nil {
			return err
		}
		for _, a := range agents {
			line := fmt.Sprintf("%s [%s]", a.Name, a.Project)
			if a.Bio != "" {
				line += " — " + a.Bio
			}
			fmt.Println(line)
		}
		return nil
	},
}

var agentCreateName string
var agentCreateProject string
var agentCreateBio string

var agentCreateCmd = &cobra.Command{
	Use:   "create",
	Short: "Create an agent",
	RunE: func(cmd *cobra.Command, args []string) error {
		c := client.New(serverURL)
		proj := agentCreateProject
		if proj == "" {
			proj = projectName
		}
		if proj == "" {
			return fmt.Errorf("project is required (use --project or KKULLM_PROJECT)")
		}
		agent, err := c.CreateAgent(agentCreateName, proj, agentCreateBio)
		if err != nil {
			return err
		}
		fmt.Printf("Created agent %q in project %q (id: %d)\n", agent.Name, agent.Project, agent.ID)
		return nil
	},
}

var agentShowCmd = &cobra.Command{
	Use:   "show <name>",
	Short: "Show agent details",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		// For now, list all and filter — a GetAgentByName endpoint would be cleaner
		c := client.New(serverURL)
		agents, err := c.ListAgents("")
		if err != nil {
			return err
		}
		for _, a := range agents {
			if a.Name == args[0] {
				fmt.Printf("Name:    %s\n", a.Name)
				fmt.Printf("Project: %s\n", a.Project)
				if a.Bio != "" {
					fmt.Printf("Bio:     %s\n", a.Bio)
				}
				fmt.Printf("ID:      %d\n", a.ID)
				return nil
			}
		}
		return fmt.Errorf("agent %q not found", args[0])
	},
}

func init() {
	agentCreateCmd.Flags().StringVar(&agentCreateName, "name", "", "Agent name")
	agentCreateCmd.Flags().StringVar(&agentCreateProject, "project", "", "Agent's home project")
	agentCreateCmd.Flags().StringVar(&agentCreateBio, "bio", "", "Agent bio")
	agentCreateCmd.MarkFlagRequired("name")

	agentCmd.AddCommand(agentListCmd, agentCreateCmd, agentShowCmd)
	rootCmd.AddCommand(agentCmd)
}
```

- [ ] **Step 4: Verify it compiles**

```bash
go build -o kkullm . && ./kkullm project --help && ./kkullm agent --help && ./kkullm serve --help
```

Expected: help output for each subcommand.

- [ ] **Step 5: Commit**

```bash
git add cmd/serve.go cmd/project.go cmd/agent.go
git commit -m "feat: add CLI commands for serve, project, and agent"
```

---

## Task 15: CLI Commands — card, comment, asset

**Files:**
- Create: `cmd/card.go`, `cmd/comment.go`, `cmd/asset.go`

- [ ] **Step 1: Implement card commands**

```go
// cmd/card.go
package cmd

import (
	"encoding/json"
	"fmt"
	"strconv"
	"strings"

	"github.com/joelhelbling/kkullm/client"
	"github.com/joelhelbling/kkullm/model"
	"github.com/spf13/cobra"
)

var cardCmd = &cobra.Command{
	Use:   "card",
	Short: "Manage cards",
}

var cardListStatus string
var cardListAssignee string
var cardListTag string
var cardListFormat string
var cardListJSON bool

var cardListCmd = &cobra.Command{
	Use:   "list",
	Short: "List cards",
	RunE: func(cmd *cobra.Command, args []string) error {
		c := client.New(serverURL)
		cards, err := c.ListCards(projectName, cardListAssignee, cardListStatus, cardListTag)
		if err != nil {
			return err
		}

		if cardListJSON {
			data, _ := json.MarshalIndent(cards, "", "  ")
			fmt.Println(string(data))
			return nil
		}

		for _, card := range cards {
			if cardListFormat == "full" {
				printCardFull(card)
			} else {
				printCardBrief(card)
			}
		}
		return nil
	},
}

func printCardBrief(c model.Card) {
	assignees := strings.Join(c.Assignees, ",")
	if assignees == "" {
		assignees = "-"
	}
	tags := ""
	if len(c.Tags) > 0 {
		tags = " [" + strings.Join(c.Tags, ",") + "]"
	}
	fmt.Printf("#%-4d %-12s %-12s %s%s\n", c.ID, c.Status, assignees, c.Title, tags)
}

func printCardFull(c model.Card) {
	fmt.Printf("--- Card #%d ---\n", c.ID)
	fmt.Printf("Title:    %s\n", c.Title)
	fmt.Printf("Status:   %s\n", c.Status)
	fmt.Printf("Project:  %s\n", c.Project)
	if len(c.Assignees) > 0 {
		fmt.Printf("Assigned: %s\n", strings.Join(c.Assignees, ", "))
	}
	if len(c.Tags) > 0 {
		fmt.Printf("Tags:     %s\n", strings.Join(c.Tags, ", "))
	}
	if c.Body != "" {
		fmt.Printf("Body:     %s\n", c.Body)
	}
	for _, r := range c.Relations {
		fmt.Printf("Relation: %s #%d\n", r.RelationType, r.RelatedCardID)
	}
	if c.CommentCount > 0 {
		fmt.Printf("Comments: %d\n", c.CommentCount)
	}
	fmt.Printf("Age:      %s\n", c.CreatedAt.Format("2006-01-02"))
	fmt.Println()
}

var cardShowCmd = &cobra.Command{
	Use:   "show <id>",
	Short: "Show card details",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		id, err := strconv.Atoi(args[0])
		if err != nil {
			return fmt.Errorf("invalid card id: %s", args[0])
		}
		c := client.New(serverURL)
		card, err := c.GetCard(id)
		if err != nil {
			return err
		}
		printCardFull(*card)
		return nil
	},
}

var cardCreateTitle string
var cardCreateBody string
var cardCreateStatus string
var cardCreateAssignee []string
var cardCreateTag []string
var cardCreateBlockedBy []int
var cardCreateBelongsTo []int
var cardCreateInterestedIn []int

var cardCreateCmd = &cobra.Command{
	Use:   "create",
	Short: "Create a card",
	RunE: func(cmd *cobra.Command, args []string) error {
		requireAgent()

		c := client.New(serverURL)
		proj := projectName
		if proj == "" {
			return fmt.Errorf("project is required (use --project or KKULLM_PROJECT)")
		}

		var relations []model.CardRelation
		for _, id := range cardCreateBlockedBy {
			relations = append(relations, model.CardRelation{RelatedCardID: id, RelationType: "blocked_by"})
		}
		for _, id := range cardCreateBelongsTo {
			relations = append(relations, model.CardRelation{RelatedCardID: id, RelationType: "belongs_to"})
		}
		for _, id := range cardCreateInterestedIn {
			relations = append(relations, model.CardRelation{RelatedCardID: id, RelationType: "interested_in"})
		}

		status := cardCreateStatus
		if status == "" {
			status = "considering"
		}

		card, err := c.CreateCard(client.CardCreateRequest{
			Title:     cardCreateTitle,
			Body:      cardCreateBody,
			Status:    status,
			Project:   proj,
			Assignees: cardCreateAssignee,
			Tags:      cardCreateTag,
			Relations: relations,
		})
		if err != nil {
			return err
		}
		fmt.Printf("Created card #%d: %s\n", card.ID, card.Title)
		return nil
	},
}

var cardUpdateStatus string
var cardUpdateTitle string
var cardUpdateBody string
var cardUpdateAssignee []string
var cardUpdateTag []string
var cardUpdateBlockedBy []int
var cardUpdateBelongsTo []int
var cardUpdateInterestedIn []int

var cardUpdateCmd = &cobra.Command{
	Use:   "update <id>",
	Short: "Update a card",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		requireAgent()

		id, err := strconv.Atoi(args[0])
		if err != nil {
			return fmt.Errorf("invalid card id: %s", args[0])
		}

		c := client.New(serverURL)
		req := client.CardUpdateRequest{}

		if cmd.Flags().Changed("status") {
			req.Status = &cardUpdateStatus
		}
		if cmd.Flags().Changed("title") {
			req.Title = &cardUpdateTitle
		}
		if cmd.Flags().Changed("body") {
			req.Body = &cardUpdateBody
		}
		if cmd.Flags().Changed("assignee") {
			req.Assignees = cardUpdateAssignee
		}
		if cmd.Flags().Changed("tag") {
			req.Tags = cardUpdateTag
		}

		var relations []model.CardRelation
		for _, rid := range cardUpdateBlockedBy {
			relations = append(relations, model.CardRelation{RelatedCardID: rid, RelationType: "blocked_by"})
		}
		for _, rid := range cardUpdateBelongsTo {
			relations = append(relations, model.CardRelation{RelatedCardID: rid, RelationType: "belongs_to"})
		}
		for _, rid := range cardUpdateInterestedIn {
			relations = append(relations, model.CardRelation{RelatedCardID: rid, RelationType: "interested_in"})
		}
		if len(relations) > 0 {
			req.Relations = relations
		}

		card, err := c.UpdateCard(id, req)
		if err != nil {
			return err
		}
		fmt.Printf("Updated card #%d: %s [%s]\n", card.ID, card.Title, card.Status)
		return nil
	},
}

func init() {
	cardListCmd.Flags().StringVar(&cardListStatus, "status", "", "Filter by status (comma-separated)")
	cardListCmd.Flags().StringVar(&cardListAssignee, "assignee", "", "Filter by assignee name")
	cardListCmd.Flags().StringVar(&cardListTag, "tag", "", "Filter by tag")
	cardListCmd.Flags().StringVar(&cardListFormat, "format", "brief", "Output format: brief or full")
	cardListCmd.Flags().BoolVar(&cardListJSON, "json", false, "Output as JSON")

	cardCreateCmd.Flags().StringVar(&cardCreateTitle, "title", "", "Card title")
	cardCreateCmd.Flags().StringVar(&cardCreateBody, "body", "", "Card body")
	cardCreateCmd.Flags().StringVar(&cardCreateStatus, "status", "", "Card status (default: considering)")
	cardCreateCmd.Flags().StringSliceVar(&cardCreateAssignee, "assignee", nil, "Assignee name (repeatable)")
	cardCreateCmd.Flags().StringSliceVar(&cardCreateTag, "tag", nil, "Tag (repeatable)")
	cardCreateCmd.Flags().IntSliceVar(&cardCreateBlockedBy, "blocked-by", nil, "Blocked by card ID (repeatable)")
	cardCreateCmd.Flags().IntSliceVar(&cardCreateBelongsTo, "belongs-to", nil, "Belongs to card ID (repeatable)")
	cardCreateCmd.Flags().IntSliceVar(&cardCreateInterestedIn, "interested-in", nil, "Interested in card ID (repeatable)")
	cardCreateCmd.MarkFlagRequired("title")

	cardUpdateCmd.Flags().StringVar(&cardUpdateStatus, "status", "", "New status")
	cardUpdateCmd.Flags().StringVar(&cardUpdateTitle, "title", "", "New title")
	cardUpdateCmd.Flags().StringVar(&cardUpdateBody, "body", "", "New body")
	cardUpdateCmd.Flags().StringSliceVar(&cardUpdateAssignee, "assignee", nil, "New assignee(s)")
	cardUpdateCmd.Flags().StringSliceVar(&cardUpdateTag, "tag", nil, "New tag(s)")
	cardUpdateCmd.Flags().IntSliceVar(&cardUpdateBlockedBy, "blocked-by", nil, "Add blocked_by relation")
	cardUpdateCmd.Flags().IntSliceVar(&cardUpdateBelongsTo, "belongs-to", nil, "Add belongs_to relation")
	cardUpdateCmd.Flags().IntSliceVar(&cardUpdateInterestedIn, "interested-in", nil, "Add interested_in relation")

	cardCmd.AddCommand(cardListCmd, cardShowCmd, cardCreateCmd, cardUpdateCmd)
	rootCmd.AddCommand(cardCmd)
}
```

- [ ] **Step 2: Implement comment commands**

```go
// cmd/comment.go
package cmd

import (
	"fmt"
	"strconv"

	"github.com/joelhelbling/kkullm/client"
	"github.com/spf13/cobra"
)

var commentCmd = &cobra.Command{
	Use:   "comment",
	Short: "Manage comments on cards",
}

var commentListCmd = &cobra.Command{
	Use:   "list <card-id>",
	Short: "List comments on a card",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		cardID, err := strconv.Atoi(args[0])
		if err != nil {
			return fmt.Errorf("invalid card id: %s", args[0])
		}
		c := client.New(serverURL)
		comments, err := c.ListComments(cardID)
		if err != nil {
			return err
		}
		for _, comment := range comments {
			fmt.Printf("[%s] %s: %s\n", comment.CreatedAt.Format("2006-01-02 15:04"), comment.Agent, comment.Body)
		}
		return nil
	},
}

var commentAddBody string

var commentAddCmd = &cobra.Command{
	Use:   "add <card-id>",
	Short: "Add a comment to a card",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		agent := requireAgent()

		cardID, err := strconv.Atoi(args[0])
		if err != nil {
			return fmt.Errorf("invalid card id: %s", args[0])
		}
		c := client.New(serverURL)
		comment, err := c.CreateComment(cardID, agent, commentAddBody)
		if err != nil {
			return err
		}
		fmt.Printf("Comment added to card #%d (id: %d)\n", cardID, comment.ID)
		return nil
	},
}

func init() {
	commentAddCmd.Flags().StringVar(&commentAddBody, "body", "", "Comment text")
	commentAddCmd.MarkFlagRequired("body")

	commentCmd.AddCommand(commentListCmd, commentAddCmd)
	rootCmd.AddCommand(commentCmd)
}
```

- [ ] **Step 3: Implement asset commands**

```go
// cmd/asset.go
package cmd

import (
	"fmt"
	"strconv"

	"github.com/joelhelbling/kkullm/client"
	"github.com/spf13/cobra"
)

var assetCmd = &cobra.Command{
	Use:   "asset",
	Short: "Manage project assets",
}

var assetListName string
var assetListURL string

var assetListCmd = &cobra.Command{
	Use:   "list",
	Short: "List assets",
	RunE: func(cmd *cobra.Command, args []string) error {
		c := client.New(serverURL)
		assets, err := c.ListAssets(projectName, assetListName, assetListURL)
		if err != nil {
			return err
		}
		for _, a := range assets {
			line := fmt.Sprintf("%s [%s]", a.Name, a.Project)
			if a.URL != "" {
				line += " " + a.URL
			}
			fmt.Println(line)
		}
		return nil
	},
}

var assetCreateName string
var assetCreateDesc string
var assetCreateURL string

var assetCreateCmd = &cobra.Command{
	Use:   "create",
	Short: "Create an asset",
	RunE: func(cmd *cobra.Command, args []string) error {
		proj := projectName
		if proj == "" {
			return fmt.Errorf("project is required (use --project or KKULLM_PROJECT)")
		}
		c := client.New(serverURL)
		asset, err := c.CreateAsset(proj, assetCreateName, assetCreateDesc, assetCreateURL)
		if err != nil {
			return err
		}
		fmt.Printf("Created asset %q in project %q (id: %d)\n", asset.Name, asset.Project, asset.ID)
		return nil
	},
}

var assetShowCmd = &cobra.Command{
	Use:   "show <id>",
	Short: "Show asset details",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		id, err := strconv.Atoi(args[0])
		if err != nil {
			return fmt.Errorf("invalid asset id: %s", args[0])
		}
		c := client.New(serverURL)
		asset, err := c.GetAsset(id)
		if err != nil {
			return err
		}
		fmt.Printf("Name:        %s\n", asset.Name)
		fmt.Printf("Project:     %s\n", asset.Project)
		if asset.Description != "" {
			fmt.Printf("Description: %s\n", asset.Description)
		}
		if asset.URL != "" {
			fmt.Printf("URL:         %s\n", asset.URL)
		}
		fmt.Printf("ID:          %d\n", asset.ID)
		return nil
	},
}

func init() {
	assetListCmd.Flags().StringVar(&assetListName, "name", "", "Filter by name glob pattern")
	assetListCmd.Flags().StringVar(&assetListURL, "url", "", "Filter by URL glob pattern")

	assetCreateCmd.Flags().StringVar(&assetCreateName, "name", "", "Asset name")
	assetCreateCmd.Flags().StringVar(&assetCreateDesc, "description", "", "Asset description")
	assetCreateCmd.Flags().StringVar(&assetCreateURL, "url", "", "Asset URL")
	assetCreateCmd.MarkFlagRequired("name")

	assetCmd.AddCommand(assetListCmd, assetCreateCmd, assetShowCmd)
	rootCmd.AddCommand(assetCmd)
}
```

- [ ] **Step 4: Build and verify all commands**

```bash
go build -o kkullm . && ./kkullm --help
```

Expected: help output showing all subcommands: `serve`, `card`, `comment`, `project`, `agent`, `asset`.

- [ ] **Step 5: Run all tests**

```bash
go test ./... -v
```

Expected: all tests PASS.

- [ ] **Step 6: Commit**

```bash
git add cmd/card.go cmd/comment.go cmd/asset.go
git commit -m "feat: add CLI commands for card, comment, and asset management"
```

---

## Task 16: End-to-End Smoke Test

**Files:**
- Create: `test/e2e_test.go`

This test starts the server, runs CLI-equivalent HTTP operations, and verifies the full workflow.

- [ ] **Step 1: Write the e2e test**

```go
// test/e2e_test.go
package test

import (
	"net/http/httptest"
	"testing"

	"github.com/joelhelbling/kkullm/api"
	"github.com/joelhelbling/kkullm/client"
	"github.com/joelhelbling/kkullm/db"
	"github.com/joelhelbling/kkullm/model"
	"github.com/joelhelbling/kkullm/store"
)

func TestFullWorkflow(t *testing.T) {
	// Setup
	database, _ := db.Open(":memory:")
	defer database.Close()
	db.Migrate(database)
	db.Seed(database)

	s := store.New(database)
	srv := api.NewServer(s)
	ts := httptest.NewServer(srv.Handler())
	defer ts.Close()

	c := client.New(ts.URL)

	// 1. Create project
	proj, err := c.CreateProject("acme-backend", "The ACME backend")
	if err != nil {
		t.Fatalf("create project: %v", err)
	}
	if proj.Name != "acme-backend" {
		t.Fatalf("project name = %q", proj.Name)
	}

	// 2. Create agent
	agent, err := c.CreateAgent("dev-agent", "acme-backend", "Writes Go code")
	if err != nil {
		t.Fatalf("create agent: %v", err)
	}

	// 3. Create asset
	_, err = c.CreateAsset("acme-backend", "GitHub repo", "Main repo", "https://github.com/acme/backend")
	if err != nil {
		t.Fatalf("create asset: %v", err)
	}

	// 4. Create card with assignee and tags
	card, err := c.CreateCard(client.CardCreateRequest{
		Title:     "Implement JWT auth",
		Body:      "Add JWT middleware to all API routes",
		Status:    "todo",
		Project:   "acme-backend",
		Assignees: []string{agent.Name},
		Tags:      []string{"auth", "security"},
	})
	if err != nil {
		t.Fatalf("create card: %v", err)
	}

	// 5. Simulate agent workflow: claim card
	status := "in_flight"
	claimed, err := c.UpdateCard(card.ID, client.CardUpdateRequest{Status: &status})
	if err != nil {
		t.Fatalf("claim card: %v", err)
	}
	if claimed.Status != "in_flight" {
		t.Fatalf("status = %q, want 'in_flight'", claimed.Status)
	}

	// 6. Agent adds comment
	_, err = c.CreateComment(card.ID, "dev-agent", "Started implementing JWT middleware")
	if err != nil {
		t.Fatalf("add comment: %v", err)
	}

	// 7. Agent creates sub-task
	subtask, err := c.CreateCard(client.CardCreateRequest{
		Title:     "Write JWT tests",
		Status:    "todo",
		Project:   "acme-backend",
		Assignees: []string{"dev-agent"},
		Relations: []model.CardRelation{{RelatedCardID: card.ID, RelationType: "belongs_to"}},
	})
	if err != nil {
		t.Fatalf("create subtask: %v", err)
	}
	if len(subtask.Relations) != 1 {
		t.Fatalf("subtask relations = %d, want 1", len(subtask.Relations))
	}

	// 8. Agent completes card
	completed := "completed"
	_, err = c.UpdateCard(card.ID, client.CardUpdateRequest{Status: &completed})
	if err != nil {
		t.Fatalf("complete card: %v", err)
	}

	// 9. Verify listing works with filters
	cards, err := c.ListCards("acme-backend", "dev-agent", "todo", "")
	if err != nil {
		t.Fatalf("list cards: %v", err)
	}
	if len(cards) != 1 {
		t.Fatalf("got %d todo cards for dev-agent, want 1 (subtask)", len(cards))
	}

	// 10. Verify asset discovery
	assets, err := c.ListAssets("", "", "*github*acme*")
	if err != nil {
		t.Fatalf("list assets: %v", err)
	}
	if len(assets) != 1 {
		t.Fatalf("got %d assets matching url glob, want 1", len(assets))
	}
	if assets[0].Project != "acme-backend" {
		t.Fatalf("asset project = %q, want 'acme-backend'", assets[0].Project)
	}

	// 11. Verify invalid transition is rejected
	_, err = c.UpdateCard(card.ID, client.CardUpdateRequest{Status: &status}) // completed -> in_flight is valid
	if err != nil {
		t.Fatalf("completed -> in_flight should be valid: %v", err)
	}

	// done is terminal — can't go back
	done := "done"
	c.UpdateCard(card.ID, client.CardUpdateRequest{Status: &completed})
	c.UpdateCard(card.ID, client.CardUpdateRequest{Status: &done})
	_, err = c.UpdateCard(card.ID, client.CardUpdateRequest{Status: &status}) // done -> in_flight invalid
	if err == nil {
		t.Fatal("expected error for done -> in_flight transition")
	}
}
```

- [ ] **Step 2: Run the e2e test**

```bash
go test ./test/ -v
```

Expected: PASS — full workflow from project creation through card lifecycle to asset discovery.

- [ ] **Step 3: Run all tests one final time**

```bash
go test ./... -v
```

Expected: all tests PASS across all packages.

- [ ] **Step 4: Commit**

```bash
git add test/
git commit -m "feat: add end-to-end smoke test covering full agent workflow"
```

- [ ] **Step 5: Final build verification**

```bash
go build -o kkullm . && ls -lh kkullm
```

Expected: binary builds successfully. Note the file size (should be ~15-25MB).
