# Create & Manage Projects and Agents in the Web Admin UI — Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Let an admin create projects and agents from the web admin UI, and edit their name plus the previously web-uneditable fields (project `description`, agent `bio`).

**Architecture:** The admin section is server-rendered Go `html/template` pages with plain `POST` forms. Each of the Projects and Agents pages gains a `+ New …` button and a per-row `Edit` button, both opening a single shared modal whose title/action/fields are set by a small inline script (same style as the existing `openDeleteProjectModal`). Handlers validate, call the store, and on success `303`-redirect; on a validation error they re-render the same page with an error banner, the submitted values, and a flag that reopens the modal. The `RenameProject`/`RenameAgent` store methods are superseded by `UpdateProject`/`UpdateAgent`.

**Tech Stack:** Go (`net/http`, `html/template`), SQLite via `modernc.org/sqlite`, server-rendered HTML + plain CSS. No new dependencies.

**Reference spec:** `docs/superpowers/specs/2026-05-16-web-create-projects-agents-design.md`

---

## File Structure

Files created or modified:

- `store/project.go` — add `UpdateProject`; later remove `RenameProject`.
- `store/agent.go` — add `UpdateAgent`; later remove `RenameAgent`.
- `store/project_test.go` — add `UpdateProject` tests; later remove `RenameProject` tests.
- `store/agent_test.go` — add `UpdateAgent` tests; later remove `RenameAgent` tests.
- `web/web.go` — swap the two `/rename` routes for `create` + `update` routes.
- `web/admin_handlers.go` — new create/update handlers, page-data helpers, `isUniqueViolation`; remove the two rename handlers.
- `web/admin_handlers_test.go` — replace rename handler tests with create/update tests.
- `web/templates/admin_projects.html` — header button, row Edit button, shared create/edit modal, error banner.
- `web/templates/admin_agents.html` — same shape, with a project `<select>` on create.
- `web/static/css/app.css` — styles for the header row, row actions, error banner, modal form fields.

No database migration is needed — `projects.description` and `agents.bio` already exist.

---

## Task 1: Store — `UpdateProject`

Adds a store method that updates a project's name **and** description in one call. `RenameProject` is left in place for now (still used by the rename handler until Task 3).

**Files:**
- Modify: `store/project.go`
- Test: `store/project_test.go`

- [ ] **Step 1: Write the failing tests**

Add these three functions to `store/project_test.go` (append after `TestRenameProject_Empty`, before `TestDeleteProject_CascadesAllChildren`):

```go
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
```

- [ ] **Step 2: Run the tests to verify they fail**

Run: `go test ./store/ -run TestUpdateProject -v`
Expected: FAIL — compile error, `s.UpdateProject undefined`.

- [ ] **Step 3: Implement `UpdateProject`**

In `store/project.go`, add this function immediately after `RenameProject`:

```go
func (s *Store) UpdateProject(id int, name, description string) error {
	if name == "" {
		return fmt.Errorf("project name cannot be empty")
	}
	_, err := s.db.Exec(
		"UPDATE projects SET name = ?, description = ?, updated_at = datetime('now') WHERE id = ?",
		name, description, id,
	)
	if err != nil {
		return fmt.Errorf("update project %d: %w", id, err)
	}
	return nil
}
```

- [ ] **Step 4: Run the tests to verify they pass**

Run: `go test ./store/ -run TestUpdateProject -v`
Expected: PASS — all three tests pass.

- [ ] **Step 5: Commit**

```bash
git add store/project.go store/project_test.go
git commit -m "feat(store): add UpdateProject for name + description"
```

---

## Task 2: Store — `UpdateAgent`

Adds a store method that updates an agent's name **and** bio, preserving `RenameAgent`'s `comments.author_name` backfill so historical comment attribution stays correct. `RenameAgent` is left in place for now.

**Files:**
- Modify: `store/agent.go`
- Test: `store/agent_test.go`

- [ ] **Step 1: Write the failing tests**

Add these three functions to `store/agent_test.go` (append after `TestRenameAgent_EmptyName`, before `TestDeleteAgent_UnassignsCards_PreservesComments`):

```go
func TestUpdateAgent_OK_BackfillsHistoricalComments(t *testing.T) {
	s := setupTestDB(t)
	proj := createTestProject(t, s)
	agent := createTestAgent(t, s, "old-name", proj.ID)

	card, err := s.CreateCard(CardCreateParams{
		Title:     "test card",
		ProjectID: proj.ID,
	})
	if err != nil {
		t.Fatalf("CreateCard: %v", err)
	}
	if _, err := s.CreateComment(card.ID, agent.ID, "a comment"); err != nil {
		t.Fatalf("CreateComment: %v", err)
	}

	if err := s.UpdateAgent(agent.ID, "new-name", "new bio"); err != nil {
		t.Fatalf("UpdateAgent: %v", err)
	}

	var authorName string
	if err := s.db.QueryRow(
		"SELECT author_name FROM comments WHERE agent_id = ?", agent.ID,
	).Scan(&authorName); err != nil {
		t.Fatalf("query author_name: %v", err)
	}
	if authorName != "new-name" {
		t.Errorf("author_name = %q, want 'new-name'", authorName)
	}

	reloaded, err := s.GetAgent(agent.ID)
	if err != nil {
		t.Fatalf("GetAgent: %v", err)
	}
	if reloaded.Name != "new-name" {
		t.Errorf("agent name = %q, want 'new-name'", reloaded.Name)
	}
	if reloaded.Bio != "new bio" {
		t.Errorf("agent bio = %q, want 'new bio'", reloaded.Bio)
	}
}

func TestUpdateAgent_DuplicateName(t *testing.T) {
	s := setupTestDB(t)
	proj := createTestProject(t, s)
	a1 := createTestAgent(t, s, "alpha", proj.ID)
	a2 := createTestAgent(t, s, "beta", proj.ID)

	if err := s.UpdateAgent(a2.ID, "alpha", ""); err == nil {
		t.Fatalf("expected error updating to duplicate name, got nil")
	}

	reloaded, err := s.GetAgent(a1.ID)
	if err != nil {
		t.Fatalf("GetAgent: %v", err)
	}
	if reloaded.Name != "alpha" {
		t.Errorf("alpha agent name = %q, want 'alpha'", reloaded.Name)
	}
}

func TestUpdateAgent_EmptyName(t *testing.T) {
	s := setupTestDB(t)
	proj := createTestProject(t, s)
	agent := createTestAgent(t, s, "alpha", proj.ID)

	if err := s.UpdateAgent(agent.ID, "", "bio"); err == nil {
		t.Fatalf("expected error for empty name, got nil")
	}
}
```

- [ ] **Step 2: Run the tests to verify they fail**

Run: `go test ./store/ -run TestUpdateAgent -v`
Expected: FAIL — compile error, `s.UpdateAgent undefined`.

- [ ] **Step 3: Implement `UpdateAgent`**

In `store/agent.go`, add this function immediately after `RenameAgent`:

```go
func (s *Store) UpdateAgent(id int, name, bio string) error {
	if name == "" {
		return fmt.Errorf("agent name cannot be empty")
	}
	tx, err := s.db.Begin()
	if err != nil {
		return fmt.Errorf("begin: %w", err)
	}
	defer tx.Rollback()

	if _, err := tx.Exec(
		"UPDATE agents SET name = ?, bio = ?, updated_at = datetime('now') WHERE id = ?",
		name, bio, id,
	); err != nil {
		return fmt.Errorf("update agent %d: %w", id, err)
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

- [ ] **Step 4: Run the tests to verify they pass**

Run: `go test ./store/ -run TestUpdateAgent -v`
Expected: PASS — all three tests pass.

- [ ] **Step 5: Commit**

```bash
git add store/agent.go store/agent_test.go
git commit -m "feat(store): add UpdateAgent for name + bio"
```

---

## Task 3: Projects — create & edit in the admin UI

Replaces the project rename route/handler with `create` and `update` routes, adds the shared modal and error-banner UI, and removes the now-superseded `RenameProject` store method.

**Files:**
- Modify: `web/web.go` (route registration)
- Modify: `web/admin_handlers.go`
- Modify: `web/templates/admin_projects.html`
- Modify: `web/static/css/app.css`
- Modify: `store/project.go` (remove `RenameProject`)
- Test: `web/admin_handlers_test.go`
- Test: `store/project_test.go` (remove `RenameProject` tests)

- [ ] **Step 1: Write the failing handler tests**

In `web/admin_handlers_test.go`, **delete** the function `TestAdminRenameProject_OK` (lines covering `func TestAdminRenameProject_OK`). Replace it with the following four functions:

```go
func TestAdminCreateProject_OK(t *testing.T) {
	mux, st := setupTestMuxWithStore(t)

	form := url.Values{"name": {"newproj"}, "description": {"a fresh project"}}
	req := httptest.NewRequest(http.MethodPost, "/admin/projects/create",
		strings.NewReader(form.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)

	if rec.Code != http.StatusSeeOther && rec.Code != http.StatusFound {
		t.Fatalf("expected redirect, got %d (body: %s)", rec.Code, rec.Body.String())
	}
	p, err := st.GetProjectByName("newproj")
	if err != nil {
		t.Fatalf("GetProjectByName: %v", err)
	}
	if p.Description != "a fresh project" {
		t.Errorf("description = %q, want 'a fresh project'", p.Description)
	}
}

func TestAdminCreateProject_EmptyName(t *testing.T) {
	mux, _ := setupTestMuxWithStore(t)

	form := url.Values{"name": {"  "}, "description": {"x"}}
	req := httptest.NewRequest(http.MethodPost, "/admin/projects/create",
		strings.NewReader(form.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d", rec.Code)
	}
	if !strings.Contains(rec.Body.String(), "name is required") {
		t.Errorf("expected error message in body, got: %s", rec.Body.String())
	}
}

func TestAdminCreateProject_DuplicateName(t *testing.T) {
	mux, st := setupTestMuxWithStore(t)
	if _, err := st.CreateProject("dup", ""); err != nil {
		t.Fatalf("CreateProject: %v", err)
	}

	form := url.Values{"name": {"dup"}, "description": {""}}
	req := httptest.NewRequest(http.MethodPost, "/admin/projects/create",
		strings.NewReader(form.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d", rec.Code)
	}
	if !strings.Contains(rec.Body.String(), "already exists") {
		t.Errorf("expected 'already exists' in body, got: %s", rec.Body.String())
	}
}

func TestAdminUpdateProject_OK(t *testing.T) {
	mux, st := setupTestMuxWithStore(t)
	p, err := st.CreateProject("orig", "orig desc")
	if err != nil {
		t.Fatalf("CreateProject: %v", err)
	}

	form := url.Values{"name": {"renamed"}, "description": {"updated desc"}}
	req := httptest.NewRequest(http.MethodPost, "/admin/projects/"+strconv.Itoa(p.ID)+"/update",
		strings.NewReader(form.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)

	if rec.Code != http.StatusSeeOther && rec.Code != http.StatusFound {
		t.Fatalf("expected redirect, got %d (body: %s)", rec.Code, rec.Body.String())
	}
	got, err := st.GetProject(p.ID)
	if err != nil {
		t.Fatalf("GetProject: %v", err)
	}
	if got.Name != "renamed" || got.Description != "updated desc" {
		t.Errorf("got name=%q desc=%q, want 'renamed'/'updated desc'", got.Name, got.Description)
	}
}
```

- [ ] **Step 2: Run the tests to verify they fail**

Run: `go test ./web/ -run 'TestAdminCreateProject|TestAdminUpdateProject' -v`
Expected: FAIL — the `/admin/projects/create` and `/update` routes return 404 (no matching route), so the status assertions fail.

- [ ] **Step 3: Update the routes in `web/web.go`**

In `web/web.go`, replace this line:

```go
	mux.Handle("POST /admin/projects/{id}/rename", RequireAdmin(http.HandlerFunc(ws.handleAdminRenameProject)))
```

with these two lines:

```go
	mux.Handle("POST /admin/projects/create", RequireAdmin(http.HandlerFunc(ws.handleAdminCreateProject)))
	mux.Handle("POST /admin/projects/{id}/update", RequireAdmin(http.HandlerFunc(ws.handleAdminUpdateProject)))
```

- [ ] **Step 4: Rework the project handlers in `web/admin_handlers.go`**

First, replace the `adminProjectRow` and `adminProjectsData` type declarations with:

```go
type adminProjectRow struct {
	ID          int
	Name        string
	Description string
	CardCount   int
	AgentCount  int
}

// adminProjectForm carries the values a user submitted, so an error
// re-render can repopulate the reopened modal.
type adminProjectForm struct {
	ID          int
	Name        string
	Description string
}

type adminProjectsData struct {
	Section  string
	Projects []adminProjectRow
	Error    string
	Form     adminProjectForm
	Reopen   string // "", "create", or "edit"
}
```

Next, replace the entire `handleAdminProjects` function and the entire `handleAdminRenameProject` function with the following. (The `handleAdminRenameProject` function is removed; the create/update handlers and helpers below take its place.)

```go
func (ws *WebServer) handleAdminProjects(w http.ResponseWriter, r *http.Request) {
	data, err := ws.projectsPageData()
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	ws.renderProjectsPage(w, http.StatusOK, data)
}

// projectsPageData builds the full Projects admin page model from the store.
func (ws *WebServer) projectsPageData() (adminProjectsData, error) {
	projects, err := ws.store.ListProjects()
	if err != nil {
		return adminProjectsData{}, err
	}
	rows := make([]adminProjectRow, 0, len(projects))
	for _, p := range projects {
		cc, err := ws.store.CountCardsForProject(p.ID)
		if err != nil {
			return adminProjectsData{}, err
		}
		ac, err := ws.store.CountAgentsForProject(p.ID)
		if err != nil {
			return adminProjectsData{}, err
		}
		rows = append(rows, adminProjectRow{
			ID: p.ID, Name: p.Name, Description: p.Description,
			CardCount: cc, AgentCount: ac,
		})
	}
	return adminProjectsData{Section: "projects", Projects: rows}, nil
}

func (ws *WebServer) renderProjectsPage(w http.ResponseWriter, status int, data adminProjectsData) {
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.WriteHeader(status)
	if err := tmpl.ExecuteTemplate(w, "admin_projects", data); err != nil {
		// Headers are already sent; nothing more we can do but log via the error.
		http.Error(w, err.Error(), http.StatusInternalServerError)
	}
}

// renderProjectError re-renders the Projects page with an error banner and the
// submitted values, reopening the modal named by reopen ("create" or "edit").
func (ws *WebServer) renderProjectError(w http.ResponseWriter, msg string, form adminProjectForm, reopen string) {
	data, err := ws.projectsPageData()
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	data.Error = msg
	data.Form = form
	data.Reopen = reopen
	ws.renderProjectsPage(w, http.StatusBadRequest, data)
}

func (ws *WebServer) handleAdminCreateProject(w http.ResponseWriter, r *http.Request) {
	if err := r.ParseForm(); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	name := strings.TrimSpace(r.FormValue("name"))
	description := strings.TrimSpace(r.FormValue("description"))
	form := adminProjectForm{Name: name, Description: description}

	if name == "" {
		ws.renderProjectError(w, "Project name is required.", form, "create")
		return
	}
	if _, err := ws.store.CreateProject(name, description); err != nil {
		if isUniqueViolation(err) {
			ws.renderProjectError(w, fmt.Sprintf("A project named %q already exists.", name), form, "create")
			return
		}
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	http.Redirect(w, r, "/admin/projects", http.StatusSeeOther)
}

func (ws *WebServer) handleAdminUpdateProject(w http.ResponseWriter, r *http.Request) {
	id, err := strconv.Atoi(r.PathValue("id"))
	if err != nil {
		http.Error(w, "bad id", http.StatusBadRequest)
		return
	}
	if err := r.ParseForm(); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	name := strings.TrimSpace(r.FormValue("name"))
	description := strings.TrimSpace(r.FormValue("description"))
	form := adminProjectForm{ID: id, Name: name, Description: description}

	if name == "" {
		ws.renderProjectError(w, "Project name is required.", form, "edit")
		return
	}
	if err := ws.store.UpdateProject(id, name, description); err != nil {
		if isUniqueViolation(err) {
			ws.renderProjectError(w, fmt.Sprintf("A project named %q already exists.", name), form, "edit")
			return
		}
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	ws.broadcastProjectRenamed(id, name)
	http.Redirect(w, r, "/admin/projects", http.StatusSeeOther)
}

// isUniqueViolation reports whether err is a SQLite UNIQUE constraint failure.
// The modernc.org/sqlite driver reports these with the text
// "UNIQUE constraint failed" in the error message.
func isUniqueViolation(err error) bool {
	return err != nil && strings.Contains(err.Error(), "UNIQUE constraint failed")
}
```

The error message text the test expects (`"name is required"`) is a substring of `"Project name is required."` — keep that wording.

`fmt` is already imported in `admin_handlers.go`? It is not — add `"fmt"` to the import block in `web/admin_handlers.go`.

- [ ] **Step 5: Rewrite the projects template**

Replace the entire contents of `web/templates/admin_projects.html` with:

```html
{{define "admin_projects"}}
{{template "admin_shell_top" .}}
<div class="admin-header">
  <h1>Projects</h1>
  <button type="button" class="btn-small" onclick="openProjectModal('create')">+ New project</button>
</div>
<p class="admin-desc">Create, edit, or delete projects. Deleting a project permanently removes its cards, comments, agents, and assets.</p>
{{if .Error}}<div class="admin-error" role="alert">{{.Error}}</div>{{end}}
<ul class="admin-list">
{{range .Projects}}
  <li class="admin-row" data-project-id="{{.ID}}" data-project-name="{{.Name}}" data-project-description="{{.Description}}">
    <span class="admin-name">{{.Name}}</span>
    <span class="admin-meta">{{.CardCount}} cards · {{.AgentCount}} agents</span>
    <span class="admin-row-actions">
      <button type="button" class="btn-small" onclick="openProjectModal('edit', this.closest('.admin-row'))">Edit</button>
      <button type="button" class="btn-outlined-danger" onclick="openDeleteProjectModal(this)">Delete</button>
    </span>
  </li>
{{end}}
</ul>

<div id="project-modal" class="modal" hidden>
  <div class="modal-content">
    <h2 id="pm-title">New project</h2>
    <form method="post" id="project-form">
      <label for="pm-name">Name</label>
      <input id="pm-name" type="text" name="name" autocomplete="off" required>
      <label for="pm-description">Description</label>
      <textarea id="pm-description" name="description" rows="3"></textarea>
      <div class="modal-actions">
        <button type="button" onclick="document.getElementById('project-modal').hidden = true">Cancel</button>
        <button type="submit" class="btn-small">Save</button>
      </div>
    </form>
  </div>
</div>

<div id="delete-project-modal" class="modal" hidden>
  <div class="modal-content danger-panel">
    <h2>Delete project</h2>
    <p>This cannot be undone. All cards, comments, agents, and assets in this project will be permanently deleted.</p>
    <form method="post" id="delete-project-form">
      <label for="dp-confirm">Type the project name <code id="dp-name"></code> to enable:</label>
      <input id="dp-confirm" type="text" name="confirm" autocomplete="off" class="mono"
             oninput="document.getElementById('dp-btn').disabled = this.value !== document.getElementById('dp-name').textContent">
      <button type="button" onclick="document.getElementById('delete-project-modal').hidden = true">Cancel</button>
      <button id="dp-btn" type="submit" class="btn-danger" disabled>Delete</button>
    </form>
  </div>
</div>

<script>
function openProjectModal(mode, row) {
  const form = document.getElementById('project-form');
  const name = document.getElementById('pm-name');
  const desc = document.getElementById('pm-description');
  if (mode === 'edit') {
    document.getElementById('pm-title').textContent = 'Edit project';
    form.action = '/admin/projects/' + row.dataset.projectId + '/update';
    name.value = row.dataset.projectName;
    desc.value = row.dataset.projectDescription;
  } else {
    document.getElementById('pm-title').textContent = 'New project';
    form.action = '/admin/projects/create';
    name.value = '';
    desc.value = '';
  }
  document.getElementById('project-modal').hidden = false;
}
function openDeleteProjectModal(btn) {
  const row = btn.closest('.admin-row');
  document.getElementById('dp-name').textContent = row.dataset.projectName;
  document.getElementById('delete-project-form').action = '/admin/projects/' + row.dataset.projectId + '/delete';
  document.getElementById('dp-confirm').value = '';
  document.getElementById('dp-btn').disabled = true;
  document.getElementById('delete-project-modal').hidden = false;
}
</script>
{{if .Reopen}}
<script>
(function () {
  document.getElementById('pm-name').value = {{.Form.Name}};
  document.getElementById('pm-description').value = {{.Form.Description}};
  {{if eq .Reopen "edit"}}
  document.getElementById('pm-title').textContent = 'Edit project';
  document.getElementById('project-form').action = '/admin/projects/{{.Form.ID}}/update';
  {{else}}
  document.getElementById('pm-title').textContent = 'New project';
  document.getElementById('project-form').action = '/admin/projects/create';
  {{end}}
  document.getElementById('project-modal').hidden = false;
})();
</script>
{{end}}
{{template "admin_shell_bottom" .}}
{{end}}
```

- [ ] **Step 6: Add the supporting CSS**

In `web/static/css/app.css`, add the following block immediately before the `/* Modal overlay (admin delete confirmations) */` comment (around line 2184):

```css
/* Admin create/edit affordances */
.admin-header {
  display: flex;
  align-items: baseline;
  justify-content: space-between;
  gap: 16px;
}

.admin-row-actions {
  display: flex;
  gap: 8px;
  align-items: center;
}

.admin-error {
  font-family: var(--font-body);
  font-size: 13px;
  color: var(--danger-strong);
  background: var(--danger-tint);
  border: 1px solid var(--danger);
  border-radius: var(--radius-md);
  padding: 10px 14px;
  margin-bottom: 20px;
}

.admin-readonly {
  font-family: var(--font-mono);
  font-size: 13px;
  color: var(--text-secondary);
  padding: 9px 12px;
  background: var(--bg-surface-2);
  border: 1px solid var(--border-light);
  border-radius: var(--radius-md);
}

.modal-actions {
  display: flex;
  gap: 10px;
  justify-content: flex-end;
}

.modal-content form input[type="text"],
.modal-content form textarea,
.modal-content form select {
  font-family: var(--font-body);
  font-size: 14px;
  color: var(--text-primary);
  background: var(--bg-input);
  border: 1px solid var(--border);
  border-radius: var(--radius-md);
  padding: 9px 12px;
  outline: none;
  width: 100%;
  transition: border-color 0.15s, box-shadow 0.15s;
}

.modal-content form textarea { resize: vertical; }

.modal-content form input[type="text"]:focus,
.modal-content form textarea:focus,
.modal-content form select:focus {
  border-color: var(--accent);
  box-shadow: 0 0 0 3px var(--accent-tint);
}
```

(The `.admin-readonly` rule is used by the agents page in Task 4; defining it here keeps all the new admin CSS in one block.)

- [ ] **Step 7: Remove `RenameProject` and its tests**

`RenameProject` is now unused. In `store/project.go`, delete the entire `RenameProject` function. In `store/project_test.go`, delete the three functions `TestRenameProject_OK`, `TestRenameProject_DuplicateName`, and `TestRenameProject_Empty`.

- [ ] **Step 8: Run the project test suites to verify they pass**

Run: `go test ./web/ ./store/ -run 'Project' -v`
Expected: PASS — `TestAdminCreateProject_*`, `TestAdminUpdateProject_OK`, `TestUpdateProject_*`, and the existing project delete/render tests all pass. No references to `RenameProject` remain.

- [ ] **Step 9: Verify the whole package still builds and passes**

Run: `go build ./... && go test ./web/ ./store/`
Expected: PASS — build succeeds, both packages green.

- [ ] **Step 10: Commit**

```bash
git add web/web.go web/admin_handlers.go web/admin_handlers_test.go \
  web/templates/admin_projects.html web/static/css/app.css \
  store/project.go store/project_test.go
git commit -m "feat(web): create and edit projects in the admin UI"
```

---

## Task 4: Agents — create & edit in the admin UI

Mirrors Task 3 for agents. The create modal includes a project `<select>`; the edit modal shows the project read-only (agents are not reassignable). Removes the now-superseded `RenameAgent` store method.

**Files:**
- Modify: `web/web.go` (route registration)
- Modify: `web/admin_handlers.go`
- Modify: `web/templates/admin_agents.html`
- Modify: `store/agent.go` (remove `RenameAgent`)
- Test: `web/admin_handlers_test.go`
- Test: `store/agent_test.go` (remove `RenameAgent` tests)

- [ ] **Step 1: Write the failing handler tests**

In `web/admin_handlers_test.go`, **delete** the function `TestAdminRenameAgent_OK`. Replace it with the following five functions:

```go
func TestAdminCreateAgent_OK(t *testing.T) {
	mux, st := setupTestMuxWithStore(t)
	p, err := st.CreateProject("p1", "")
	if err != nil {
		t.Fatalf("CreateProject: %v", err)
	}

	form := url.Values{"name": {"rosie"}, "project": {"p1"}, "bio": {"helper bot"}}
	req := httptest.NewRequest(http.MethodPost, "/admin/agents/create",
		strings.NewReader(form.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)

	if rec.Code != http.StatusSeeOther && rec.Code != http.StatusFound {
		t.Fatalf("expected redirect, got %d (body: %s)", rec.Code, rec.Body.String())
	}
	a, err := st.GetAgentByName("rosie")
	if err != nil {
		t.Fatalf("GetAgentByName: %v", err)
	}
	if a.ProjectID != p.ID {
		t.Errorf("ProjectID = %d, want %d", a.ProjectID, p.ID)
	}
	if a.Bio != "helper bot" {
		t.Errorf("bio = %q, want 'helper bot'", a.Bio)
	}
}

func TestAdminCreateAgent_EmptyName(t *testing.T) {
	mux, _ := setupTestMuxWithStore(t)

	form := url.Values{"name": {"  "}, "project": {"orchestration"}}
	req := httptest.NewRequest(http.MethodPost, "/admin/agents/create",
		strings.NewReader(form.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d", rec.Code)
	}
	if !strings.Contains(rec.Body.String(), "name is required") {
		t.Errorf("expected error message in body, got: %s", rec.Body.String())
	}
}

func TestAdminCreateAgent_MissingProject(t *testing.T) {
	mux, _ := setupTestMuxWithStore(t)

	form := url.Values{"name": {"rosie"}, "project": {""}}
	req := httptest.NewRequest(http.MethodPost, "/admin/agents/create",
		strings.NewReader(form.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d", rec.Code)
	}
	if !strings.Contains(rec.Body.String(), "project") {
		t.Errorf("expected a project-related error, got: %s", rec.Body.String())
	}
}

func TestAdminCreateAgent_DuplicateName(t *testing.T) {
	mux, st := setupTestMuxWithStore(t)
	p, err := st.CreateProject("p1", "")
	if err != nil {
		t.Fatalf("CreateProject: %v", err)
	}
	if _, err := st.CreateAgent("dupe", p.ID, ""); err != nil {
		t.Fatalf("CreateAgent: %v", err)
	}

	form := url.Values{"name": {"dupe"}, "project": {"p1"}}
	req := httptest.NewRequest(http.MethodPost, "/admin/agents/create",
		strings.NewReader(form.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d", rec.Code)
	}
	if !strings.Contains(rec.Body.String(), "already exists") {
		t.Errorf("expected 'already exists' in body, got: %s", rec.Body.String())
	}
}

func TestAdminUpdateAgent_OK(t *testing.T) {
	mux, st := setupTestMuxWithStore(t)
	p, err := st.CreateProject("p1", "")
	if err != nil {
		t.Fatalf("CreateProject: %v", err)
	}
	a, err := st.CreateAgent("old", p.ID, "old bio")
	if err != nil {
		t.Fatalf("CreateAgent: %v", err)
	}

	form := url.Values{"name": {"new"}, "bio": {"new bio"}}
	req := httptest.NewRequest(http.MethodPost, "/admin/agents/"+strconv.Itoa(a.ID)+"/update",
		strings.NewReader(form.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)

	if rec.Code != http.StatusSeeOther && rec.Code != http.StatusFound {
		t.Fatalf("expected redirect, got %d (body: %s)", rec.Code, rec.Body.String())
	}
	got, err := st.GetAgent(a.ID)
	if err != nil {
		t.Fatalf("GetAgent: %v", err)
	}
	if got.Name != "new" || got.Bio != "new bio" {
		t.Errorf("got name=%q bio=%q, want 'new'/'new bio'", got.Name, got.Bio)
	}
}
```

- [ ] **Step 2: Run the tests to verify they fail**

Run: `go test ./web/ -run 'TestAdminCreateAgent|TestAdminUpdateAgent' -v`
Expected: FAIL — the `/admin/agents/create` and `/update` routes return 404.

- [ ] **Step 3: Update the routes in `web/web.go`**

In `web/web.go`, replace this line:

```go
	mux.Handle("POST /admin/agents/{id}/rename", RequireAdmin(http.HandlerFunc(ws.handleAdminRenameAgent)))
```

with these two lines:

```go
	mux.Handle("POST /admin/agents/create", RequireAdmin(http.HandlerFunc(ws.handleAdminCreateAgent)))
	mux.Handle("POST /admin/agents/{id}/update", RequireAdmin(http.HandlerFunc(ws.handleAdminUpdateAgent)))
```

- [ ] **Step 4: Rework the agent handlers in `web/admin_handlers.go`**

Replace the `adminAgentsData` type declaration with:

```go
// adminAgentForm carries submitted values for an error re-render.
type adminAgentForm struct {
	ID      int
	Name    string
	Project string
	Bio     string
}

type adminAgentsData struct {
	Section  string
	Agents   []model.Agent
	Projects []model.Project // populates the create modal's project <select>
	Error    string
	Form     adminAgentForm
	Reopen   string // "", "create", or "edit"
}
```

Replace the entire `handleAdminAgents` function and the entire `handleAdminRenameAgent` function with the following. (`handleAdminRenameAgent` is removed.)

```go
func (ws *WebServer) handleAdminAgents(w http.ResponseWriter, r *http.Request) {
	data, err := ws.agentsPageData()
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	ws.renderAgentsPage(w, http.StatusOK, data)
}

func (ws *WebServer) agentsPageData() (adminAgentsData, error) {
	agents, err := ws.store.ListAgents("")
	if err != nil {
		return adminAgentsData{}, err
	}
	projects, err := ws.store.ListProjects()
	if err != nil {
		return adminAgentsData{}, err
	}
	return adminAgentsData{Section: "agents", Agents: agents, Projects: projects}, nil
}

func (ws *WebServer) renderAgentsPage(w http.ResponseWriter, status int, data adminAgentsData) {
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.WriteHeader(status)
	if err := tmpl.ExecuteTemplate(w, "admin_agents", data); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
	}
}

func (ws *WebServer) renderAgentError(w http.ResponseWriter, msg string, form adminAgentForm, reopen string) {
	data, err := ws.agentsPageData()
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	data.Error = msg
	data.Form = form
	data.Reopen = reopen
	ws.renderAgentsPage(w, http.StatusBadRequest, data)
}

func (ws *WebServer) handleAdminCreateAgent(w http.ResponseWriter, r *http.Request) {
	if err := r.ParseForm(); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	name := strings.TrimSpace(r.FormValue("name"))
	projectName := strings.TrimSpace(r.FormValue("project"))
	bio := strings.TrimSpace(r.FormValue("bio"))
	form := adminAgentForm{Name: name, Project: projectName, Bio: bio}

	if name == "" {
		ws.renderAgentError(w, "Agent name is required.", form, "create")
		return
	}
	if projectName == "" {
		ws.renderAgentError(w, "Please choose a project for the agent.", form, "create")
		return
	}
	project, err := ws.store.GetProjectByName(projectName)
	if err != nil {
		ws.renderAgentError(w, fmt.Sprintf("Project %q was not found.", projectName), form, "create")
		return
	}
	if _, err := ws.store.CreateAgent(name, project.ID, bio); err != nil {
		if isUniqueViolation(err) {
			ws.renderAgentError(w, fmt.Sprintf("An agent named %q already exists.", name), form, "create")
			return
		}
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	http.Redirect(w, r, "/admin/agents", http.StatusSeeOther)
}

func (ws *WebServer) handleAdminUpdateAgent(w http.ResponseWriter, r *http.Request) {
	id, err := strconv.Atoi(r.PathValue("id"))
	if err != nil {
		http.Error(w, "bad id", http.StatusBadRequest)
		return
	}
	if err := r.ParseForm(); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	name := strings.TrimSpace(r.FormValue("name"))
	bio := strings.TrimSpace(r.FormValue("bio"))
	form := adminAgentForm{ID: id, Name: name, Bio: bio}
	// The agent's project is fixed; look it up so an error re-render can show
	// it read-only in the reopened edit modal.
	if agent, err := ws.store.GetAgent(id); err == nil {
		form.Project = agent.Project
	}

	if name == "" {
		ws.renderAgentError(w, "Agent name is required.", form, "edit")
		return
	}
	if err := ws.store.UpdateAgent(id, name, bio); err != nil {
		if isUniqueViolation(err) {
			ws.renderAgentError(w, fmt.Sprintf("An agent named %q already exists.", name), form, "edit")
			return
		}
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	ws.broadcastAgentRenamed(id, name)
	http.Redirect(w, r, "/admin/agents", http.StatusSeeOther)
}
```

- [ ] **Step 5: Rewrite the agents template**

Replace the entire contents of `web/templates/admin_agents.html` with:

```html
{{define "admin_agents"}}
{{template "admin_shell_top" .}}
<div class="admin-header">
  <h1>Agents</h1>
  <button type="button" class="btn-small" onclick="openAgentModal('create')">+ New agent</button>
</div>
<p class="admin-desc">Create, edit, or delete agents. Deleting an agent unassigns their cards; their comments are preserved with attribution.</p>
{{if .Error}}<div class="admin-error" role="alert">{{.Error}}</div>{{end}}
<ul class="admin-list">
{{range .Agents}}
  <li class="admin-row" data-agent-id="{{.ID}}" data-agent-name="{{.Name}}" data-agent-bio="{{.Bio}}" data-agent-project="{{.Project}}">
    <span class="admin-name">{{.Name}}</span>
    <span class="admin-meta">project: {{.Project}}</span>
    <span class="admin-row-actions">
      <button type="button" class="btn-small" onclick="openAgentModal('edit', this.closest('.admin-row'))">Edit</button>
      <form method="post" action="/admin/agents/{{.ID}}/delete" class="admin-delete-form">
        <button type="submit" class="btn-outlined-danger"
                onclick="return confirm('Delete agent {{.Name}}? Their cards will be unassigned; comments preserved.')">Delete</button>
      </form>
    </span>
  </li>
{{end}}
</ul>

<div id="agent-modal" class="modal" hidden>
  <div class="modal-content">
    <h2 id="am-title">New agent</h2>
    <form method="post" id="agent-form">
      <label for="am-name">Name</label>
      <input id="am-name" type="text" name="name" autocomplete="off" required>
      <div id="am-project-create">
        <label for="am-project">Project</label>
        <select id="am-project" name="project">
          <option value="">— choose a project —</option>
          {{range .Projects}}<option value="{{.Name}}">{{.Name}}</option>{{end}}
        </select>
      </div>
      <div id="am-project-edit" hidden>
        <label>Project</label>
        <p id="am-project-readonly" class="admin-readonly"></p>
      </div>
      <label for="am-bio">Bio</label>
      <textarea id="am-bio" name="bio" rows="3"></textarea>
      <div class="modal-actions">
        <button type="button" onclick="document.getElementById('agent-modal').hidden = true">Cancel</button>
        <button type="submit" class="btn-small">Save</button>
      </div>
    </form>
  </div>
</div>

<script>
function openAgentModal(mode, row) {
  const form = document.getElementById('agent-form');
  const createBlock = document.getElementById('am-project-create');
  const editBlock = document.getElementById('am-project-edit');
  const select = document.getElementById('am-project');
  if (mode === 'edit') {
    document.getElementById('am-title').textContent = 'Edit agent';
    form.action = '/admin/agents/' + row.dataset.agentId + '/update';
    document.getElementById('am-name').value = row.dataset.agentName;
    document.getElementById('am-bio').value = row.dataset.agentBio;
    document.getElementById('am-project-readonly').textContent = row.dataset.agentProject;
    createBlock.hidden = true;
    editBlock.hidden = false;
    select.disabled = true;
  } else {
    document.getElementById('am-title').textContent = 'New agent';
    form.action = '/admin/agents/create';
    document.getElementById('am-name').value = '';
    document.getElementById('am-bio').value = '';
    select.value = '';
    createBlock.hidden = false;
    editBlock.hidden = true;
    select.disabled = false;
  }
  document.getElementById('agent-modal').hidden = false;
}
</script>
{{if .Reopen}}
<script>
(function () {
  var select = document.getElementById('am-project');
  document.getElementById('am-name').value = {{.Form.Name}};
  document.getElementById('am-bio').value = {{.Form.Bio}};
  {{if eq .Reopen "edit"}}
  document.getElementById('am-title').textContent = 'Edit agent';
  document.getElementById('agent-form').action = '/admin/agents/{{.Form.ID}}/update';
  document.getElementById('am-project-readonly').textContent = {{.Form.Project}};
  document.getElementById('am-project-create').hidden = true;
  document.getElementById('am-project-edit').hidden = false;
  select.disabled = true;
  {{else}}
  document.getElementById('am-title').textContent = 'New agent';
  document.getElementById('agent-form').action = '/admin/agents/create';
  select.value = {{.Form.Project}};
  {{end}}
  document.getElementById('agent-modal').hidden = false;
})();
</script>
{{end}}
{{template "admin_shell_bottom" .}}
{{end}}
```

- [ ] **Step 6: Remove `RenameAgent` and its tests**

`RenameAgent` is now unused. In `store/agent.go`, delete the entire `RenameAgent` function. In `store/agent_test.go`, delete the three functions `TestRenameAgent_OK_BackfillsHistoricalComments`, `TestRenameAgent_DuplicateName`, and `TestRenameAgent_EmptyName`.

- [ ] **Step 7: Run the agent test suites to verify they pass**

Run: `go test ./web/ ./store/ -run 'Agent' -v`
Expected: PASS — `TestAdminCreateAgent_*`, `TestAdminUpdateAgent_OK`, `TestUpdateAgent_*`, and the existing agent delete/render tests all pass.

- [ ] **Step 8: Verify the whole project builds and passes**

Run: `go build ./... && go test ./...`
Expected: PASS — build succeeds, every package green. No references to `RenameProject` or `RenameAgent` remain.

- [ ] **Step 9: Check formatting**

Run: `gofmt -l web/ store/`
Expected: no output (all files already formatted). If any file is listed, run `gofmt -w` on it.

- [ ] **Step 10: Commit**

```bash
git add web/web.go web/admin_handlers.go web/admin_handlers_test.go \
  web/templates/admin_agents.html store/agent.go store/agent_test.go
git commit -m "feat(web): create and edit agents in the admin UI"
```

---

## Task 5: Manual smoke test

Automated tests cover the handlers and store; this task confirms the modal UX works in a real browser.

**Files:** none (verification only).

- [ ] **Step 1: Build and run the server**

Run: `go run . serve`
Expected: server starts and prints its listen address.

- [ ] **Step 2: Exercise the Projects page**

In a browser, open `/admin/projects` and verify:
- `+ New project` opens the modal; submitting with a name creates the project and the list shows it.
- Submitting a blank name, or a name that already exists, re-renders the page with a red error banner and the modal reopened with the typed values preserved.
- `Edit` on a row opens the modal pre-filled with the current name and description; saving updates both.

- [ ] **Step 3: Exercise the Agents page**

In a browser, open `/admin/agents` and verify:
- `+ New agent` opens the modal with a populated project dropdown; creating an agent works.
- A blank name, a missing project selection, or a duplicate name each re-render with the error banner and reopened modal.
- `Edit` on a row opens the modal with name and bio editable and the project shown read-only; saving updates name and bio.

- [ ] **Step 4: Stop the server**

Press `Ctrl+C` to stop the `go run . serve` process.

---

## Self-Review Notes

- **Spec coverage:** create project (Task 3), create agent (Task 4), edit project name+description (Task 3), edit agent name+bio (Task 4), agent project fixed/read-only (Task 4 template + handler), modal pattern (Tasks 3–4 templates), plain-form + re-render error handling (Tasks 3–4 handlers), `UpdateProject`/`UpdateAgent` replacing `Rename*` (Tasks 1–4), reuse of `project_renamed`/`agent_renamed` broadcasts (Tasks 3–4 handlers), no new SSE events, no migration — all covered.
- **Type consistency:** `adminProjectForm`/`adminAgentForm`, `projectsPageData`/`agentsPageData`, `renderProjectsPage`/`renderAgentsPage`, `renderProjectError`/`renderAgentError`, and `isUniqueViolation` are defined once (Task 3 / Task 4) and referenced consistently. `isUniqueViolation` is defined in Task 3 and reused in Task 4.
- **No placeholders:** every step contains the exact code or command to run.
