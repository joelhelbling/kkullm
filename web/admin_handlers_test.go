package web

import (
	"net/http"
	"net/http/httptest"
	"net/url"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/joelhelbling/kkullm/api"
)

func TestAdminRoot_RedirectsToProjects(t *testing.T) {
	mux := setupTestMux(t)
	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/admin", nil)
	mux.ServeHTTP(rec, req)

	if rec.Code != http.StatusSeeOther && rec.Code != http.StatusFound {
		t.Fatalf("expected 303 or 302, got %d", rec.Code)
	}
	if loc := rec.Header().Get("Location"); loc != "/admin/projects" {
		t.Errorf("expected Location /admin/projects, got %q", loc)
	}
}

func TestAdminProjects_Renders(t *testing.T) {
	mux := setupTestMux(t)
	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/admin/projects", nil)
	mux.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d (body: %s)", rec.Code, rec.Body.String())
	}
	body := rec.Body.String()
	for _, want := range []string{"Projects", "Agents", "Danger Zone"} {
		if !strings.Contains(body, want) {
			t.Errorf("expected body to contain %q", want)
		}
	}
}

func TestAdminAgents_Renders(t *testing.T) {
	mux := setupTestMux(t)
	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/admin/agents", nil)
	mux.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d (body: %s)", rec.Code, rec.Body.String())
	}
	body := rec.Body.String()
	if !strings.Contains(body, "Agents") {
		t.Errorf("expected body to contain 'Agents'")
	}
	if !strings.Contains(body, "user") {
		t.Errorf("expected body to contain seeded agent name 'user'")
	}
}

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

func TestAdminDeleteProject_RejectsMismatchedConfirm(t *testing.T) {
	mux, st := setupTestMuxWithStore(t)
	p, err := st.CreateProject("alpha", "")
	if err != nil {
		t.Fatalf("CreateProject: %v", err)
	}

	form := url.Values{"confirm": {"WRONG"}}
	req := httptest.NewRequest(http.MethodPost, "/admin/projects/"+strconv.Itoa(p.ID)+"/delete",
		strings.NewReader(form.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d (body: %s)", rec.Code, rec.Body.String())
	}
	got, err := st.GetProject(p.ID)
	if err != nil {
		t.Fatalf("expected project to still exist: %v", err)
	}
	if got.Name != "alpha" {
		t.Errorf("expected name 'alpha', got %q", got.Name)
	}
}

func TestAdminDeleteProject_AcceptsMatchedConfirm(t *testing.T) {
	mux, st := setupTestMuxWithStore(t)
	p, err := st.CreateProject("alpha", "")
	if err != nil {
		t.Fatalf("CreateProject: %v", err)
	}

	form := url.Values{"confirm": {"alpha"}}
	req := httptest.NewRequest(http.MethodPost, "/admin/projects/"+strconv.Itoa(p.ID)+"/delete",
		strings.NewReader(form.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)

	if rec.Code != http.StatusSeeOther && rec.Code != http.StatusFound {
		t.Fatalf("expected redirect, got %d (body: %s)", rec.Code, rec.Body.String())
	}
	if got, err := st.GetProject(p.ID); err == nil {
		t.Errorf("expected project gone, got %+v", got)
	}
}

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

func TestAdminDeleteAgent_OK(t *testing.T) {
	mux, st := setupTestMuxWithStore(t)
	p, err := st.CreateProject("p1", "")
	if err != nil {
		t.Fatalf("CreateProject: %v", err)
	}
	a, err := st.CreateAgent("doomed", p.ID, "")
	if err != nil {
		t.Fatalf("CreateAgent: %v", err)
	}

	req := httptest.NewRequest(http.MethodPost, "/admin/agents/"+strconv.Itoa(a.ID)+"/delete", nil)
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)

	if rec.Code != http.StatusSeeOther && rec.Code != http.StatusFound {
		t.Fatalf("expected redirect, got %d (body: %s)", rec.Code, rec.Body.String())
	}
	if got, err := st.GetAgent(a.ID); err == nil {
		t.Errorf("expected agent gone, got %+v", got)
	}
}

func TestAdminPurge_RejectsWrongPhrase(t *testing.T) {
	mux, st := setupTestMuxWithStore(t)
	if _, err := st.CreateProject("alpha", ""); err != nil {
		t.Fatalf("CreateProject: %v", err)
	}

	form := url.Values{"confirm": {"purge database"}}
	req := httptest.NewRequest(http.MethodPost, "/admin/danger/purge",
		strings.NewReader(form.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d (body: %s)", rec.Code, rec.Body.String())
	}
	projects, err := st.ListProjects()
	if err != nil {
		t.Fatalf("ListProjects: %v", err)
	}
	found := false
	for _, p := range projects {
		if p.Name == "alpha" {
			found = true
			break
		}
	}
	if !found {
		t.Errorf("expected 'alpha' project to still exist after rejected purge")
	}
}

func TestAdminPurge_AcceptsExactPhrase(t *testing.T) {
	mux, st := setupTestMuxWithStore(t)
	if _, err := st.CreateProject("alpha", ""); err != nil {
		t.Fatalf("CreateProject: %v", err)
	}

	form := url.Values{"confirm": {"PURGE DATABASE"}}
	req := httptest.NewRequest(http.MethodPost, "/admin/danger/purge",
		strings.NewReader(form.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)

	if rec.Code != http.StatusSeeOther && rec.Code != http.StatusFound {
		t.Fatalf("expected redirect, got %d (body: %s)", rec.Code, rec.Body.String())
	}
	projects, err := st.ListProjects()
	if err != nil {
		t.Fatalf("ListProjects: %v", err)
	}
	for _, p := range projects {
		if p.Name == "alpha" {
			t.Errorf("expected 'alpha' project to be gone after purge, found %+v", p)
		}
	}
}

func TestBroadcastDatasetReset_PublishesEvent(t *testing.T) {
	bus := api.NewEventBus()
	ws := &WebServer{events: bus}
	ch := bus.Subscribe()
	defer bus.Unsubscribe(ch)

	ws.broadcastDatasetReset()

	select {
	case ev := <-ch:
		if ev.Type != "dataset_reset" {
			t.Errorf("expected type 'dataset_reset', got %q", ev.Type)
		}
	case <-time.After(time.Second):
		t.Fatal("timed out waiting for dataset_reset event")
	}
}

func TestBroadcastProjectRenamed_PublishesEvent(t *testing.T) {
	bus := api.NewEventBus()
	ws := &WebServer{events: bus}
	ch := bus.Subscribe()
	defer bus.Unsubscribe(ch)

	ws.broadcastProjectRenamed(7, "alpha")

	select {
	case ev := <-ch:
		if ev.Type != "project_renamed" {
			t.Errorf("expected type 'project_renamed', got %q", ev.Type)
		}
	case <-time.After(time.Second):
		t.Fatal("timed out waiting for project_renamed event")
	}
}

func TestBroadcastProjectDeleted_PublishesEvent(t *testing.T) {
	bus := api.NewEventBus()
	ws := &WebServer{events: bus}
	ch := bus.Subscribe()
	defer bus.Unsubscribe(ch)

	ws.broadcastProjectDeleted(9)

	select {
	case ev := <-ch:
		if ev.Type != "project_deleted" {
			t.Errorf("expected type 'project_deleted', got %q", ev.Type)
		}
	case <-time.After(time.Second):
		t.Fatal("timed out waiting for project_deleted event")
	}
}

func TestBroadcastAgentRenamed_PublishesEvent(t *testing.T) {
	bus := api.NewEventBus()
	ws := &WebServer{events: bus}
	ch := bus.Subscribe()
	defer bus.Unsubscribe(ch)

	ws.broadcastAgentRenamed(3, "rosie")

	select {
	case ev := <-ch:
		if ev.Type != "agent_renamed" {
			t.Errorf("expected type 'agent_renamed', got %q", ev.Type)
		}
	case <-time.After(time.Second):
		t.Fatal("timed out waiting for agent_renamed event")
	}
}

func TestBroadcastAgentDeleted_PublishesEvent(t *testing.T) {
	bus := api.NewEventBus()
	ws := &WebServer{events: bus}
	ch := bus.Subscribe()
	defer bus.Unsubscribe(ch)

	ws.broadcastAgentDeleted(11)

	select {
	case ev := <-ch:
		if ev.Type != "agent_deleted" {
			t.Errorf("expected type 'agent_deleted', got %q", ev.Type)
		}
	case <-time.After(time.Second):
		t.Fatal("timed out waiting for agent_deleted event")
	}
}

func TestBroadcastCardDeleted_PublishesEvent(t *testing.T) {
	bus := api.NewEventBus()
	ws := &WebServer{events: bus}
	ch := bus.Subscribe()
	defer bus.Unsubscribe(ch)

	ws.broadcastCardDeleted(42)

	select {
	case ev := <-ch:
		if ev.Type != "card_deleted" {
			t.Errorf("expected type 'card_deleted', got %q", ev.Type)
		}
	case <-time.After(time.Second):
		t.Fatal("timed out waiting for card_deleted event")
	}
}

func TestAdminDanger_Renders(t *testing.T) {
	mux := setupTestMux(t)
	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/admin/danger", nil)
	mux.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d (body: %s)", rec.Code, rec.Body.String())
	}
	body := rec.Body.String()
	if !strings.Contains(body, "Purge database") {
		t.Errorf("expected body to contain 'Purge database'")
	}
	if !strings.Contains(body, "PURGE DATABASE") {
		t.Errorf("expected body to contain 'PURGE DATABASE'")
	}
}
