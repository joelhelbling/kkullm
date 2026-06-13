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
	"github.com/joelhelbling/kkullm/store"
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

func TestAdminUpdateProject_EmptyName(t *testing.T) {
	mux, st := setupTestMuxWithStore(t)
	p, err := st.CreateProject("orig", "")
	if err != nil {
		t.Fatalf("CreateProject: %v", err)
	}

	form := url.Values{"name": {"  "}, "description": {"x"}}
	req := httptest.NewRequest(http.MethodPost, "/admin/projects/"+strconv.Itoa(p.ID)+"/update",
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

func TestAdminUpdateProject_DuplicateName(t *testing.T) {
	mux, st := setupTestMuxWithStore(t)
	if _, err := st.CreateProject("taken", ""); err != nil {
		t.Fatalf("CreateProject taken: %v", err)
	}
	p, err := st.CreateProject("orig", "")
	if err != nil {
		t.Fatalf("CreateProject orig: %v", err)
	}

	form := url.Values{"name": {"taken"}, "description": {""}}
	req := httptest.NewRequest(http.MethodPost, "/admin/projects/"+strconv.Itoa(p.ID)+"/update",
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

func TestAdminUpdateAgent_EmptyName(t *testing.T) {
	mux, st := setupTestMuxWithStore(t)
	p, err := st.CreateProject("p1", "")
	if err != nil {
		t.Fatalf("CreateProject: %v", err)
	}
	a, err := st.CreateAgent("old", p.ID, "")
	if err != nil {
		t.Fatalf("CreateAgent: %v", err)
	}

	form := url.Values{"name": {"  "}, "bio": {"x"}}
	req := httptest.NewRequest(http.MethodPost, "/admin/agents/"+strconv.Itoa(a.ID)+"/update",
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

func TestAdminUpdateAgent_DuplicateName(t *testing.T) {
	mux, st := setupTestMuxWithStore(t)
	p, err := st.CreateProject("p1", "")
	if err != nil {
		t.Fatalf("CreateProject: %v", err)
	}
	if _, err := st.CreateAgent("taken", p.ID, ""); err != nil {
		t.Fatalf("CreateAgent taken: %v", err)
	}
	a, err := st.CreateAgent("old", p.ID, "")
	if err != nil {
		t.Fatalf("CreateAgent old: %v", err)
	}

	form := url.Values{"name": {"taken"}, "bio": {""}}
	req := httptest.NewRequest(http.MethodPost, "/admin/agents/"+strconv.Itoa(a.ID)+"/update",
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

func TestAdminAssets_Renders(t *testing.T) {
	mux := setupTestMux(t)
	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/admin/assets", nil)
	mux.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d (body: %s)", rec.Code, rec.Body.String())
	}
	body := rec.Body.String()
	if !strings.Contains(body, "Assets") {
		t.Errorf("expected body to contain 'Assets'")
	}
}

func TestAdminShell_LinksToAssets(t *testing.T) {
	mux := setupTestMux(t)
	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/admin/projects", nil)
	mux.ServeHTTP(rec, req)
	if !strings.Contains(rec.Body.String(), "/admin/assets") {
		t.Errorf("expected admin nav to link to /admin/assets")
	}
}

func TestAdminCreateAsset_OK(t *testing.T) {
	mux, st := setupTestMuxWithStore(t)
	if _, err := st.CreateProject("p1", ""); err != nil {
		t.Fatalf("CreateProject: %v", err)
	}

	form := url.Values{"name": {"GitHub repo"}, "project": {"p1"},
		"description": {"main repo"}, "url": {"https://github.com/acme/repo"}}
	req := httptest.NewRequest(http.MethodPost, "/admin/assets/create",
		strings.NewReader(form.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)

	if rec.Code != http.StatusSeeOther && rec.Code != http.StatusFound {
		t.Fatalf("expected redirect, got %d (body: %s)", rec.Code, rec.Body.String())
	}
	assets, err := st.ListAssets(store.AssetListParams{Project: "p1"})
	if err != nil {
		t.Fatalf("ListAssets: %v", err)
	}
	if len(assets) != 1 {
		t.Fatalf("got %d assets, want 1", len(assets))
	}
	if assets[0].Name != "GitHub repo" || assets[0].URL != "https://github.com/acme/repo" {
		t.Errorf("got name=%q url=%q", assets[0].Name, assets[0].URL)
	}
}

func TestAdminCreateAsset_EmptyName(t *testing.T) {
	mux, _ := setupTestMuxWithStore(t)

	form := url.Values{"name": {"  "}, "project": {"orchestration"}}
	req := httptest.NewRequest(http.MethodPost, "/admin/assets/create",
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

func TestAdminCreateAsset_MissingProject(t *testing.T) {
	mux, _ := setupTestMuxWithStore(t)

	form := url.Values{"name": {"thing"}, "project": {""}}
	req := httptest.NewRequest(http.MethodPost, "/admin/assets/create",
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

func TestAdminUpdateAsset_OK(t *testing.T) {
	mux, st := setupTestMuxWithStore(t)
	p, err := st.CreateProject("p1", "")
	if err != nil {
		t.Fatalf("CreateProject: %v", err)
	}
	a, err := st.CreateAsset(p.ID, "old", "old desc", "https://old.example.com")
	if err != nil {
		t.Fatalf("CreateAsset: %v", err)
	}

	form := url.Values{"name": {"new"}, "description": {"new desc"}, "url": {"https://new.example.com"}}
	req := httptest.NewRequest(http.MethodPost, "/admin/assets/"+strconv.Itoa(a.ID)+"/update",
		strings.NewReader(form.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)

	if rec.Code != http.StatusSeeOther && rec.Code != http.StatusFound {
		t.Fatalf("expected redirect, got %d (body: %s)", rec.Code, rec.Body.String())
	}
	got, err := st.GetAsset(a.ID)
	if err != nil {
		t.Fatalf("GetAsset: %v", err)
	}
	if got.Name != "new" || got.Description != "new desc" || got.URL != "https://new.example.com" {
		t.Errorf("got name=%q desc=%q url=%q", got.Name, got.Description, got.URL)
	}
}

func TestAdminUpdateAsset_EmptyName(t *testing.T) {
	mux, st := setupTestMuxWithStore(t)
	p, _ := st.CreateProject("p1", "")
	a, _ := st.CreateAsset(p.ID, "old", "", "")

	form := url.Values{"name": {"  "}}
	req := httptest.NewRequest(http.MethodPost, "/admin/assets/"+strconv.Itoa(a.ID)+"/update",
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

func TestAdminDeleteAsset_OK(t *testing.T) {
	mux, st := setupTestMuxWithStore(t)
	p, _ := st.CreateProject("p1", "")
	a, _ := st.CreateAsset(p.ID, "doomed", "", "")

	req := httptest.NewRequest(http.MethodPost, "/admin/assets/"+strconv.Itoa(a.ID)+"/delete", nil)
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)

	if rec.Code != http.StatusSeeOther && rec.Code != http.StatusFound {
		t.Fatalf("expected redirect, got %d (body: %s)", rec.Code, rec.Body.String())
	}
	if got, err := st.GetAsset(a.ID); err == nil {
		t.Errorf("expected asset gone, got %+v", got)
	}
}

func TestAdminAssets_RoundTrip(t *testing.T) {
	mux, st := setupTestMuxWithStore(t)
	if _, err := st.CreateProject("p1", ""); err != nil {
		t.Fatalf("CreateProject: %v", err)
	}

	// Create
	createForm := url.Values{"name": {"Repo"}, "project": {"p1"}, "url": {"https://x.example.com"}}
	doForm(t, mux, "/admin/assets/create", createForm)

	assets, _ := st.ListAssets(store.AssetListParams{Project: "p1"})
	if len(assets) != 1 {
		t.Fatalf("after create: got %d assets, want 1", len(assets))
	}
	id := assets[0].ID

	// List shows it
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/admin/assets", nil))
	if !strings.Contains(rec.Body.String(), "Repo") {
		t.Errorf("expected list to contain 'Repo'")
	}

	// Edit
	doForm(t, mux, "/admin/assets/"+strconv.Itoa(id)+"/update",
		url.Values{"name": {"Repo2"}, "url": {""}})
	got, _ := st.GetAsset(id)
	if got.Name != "Repo2" || got.URL != "" {
		t.Errorf("after edit: name=%q url=%q", got.Name, got.URL)
	}

	// Delete
	doForm(t, mux, "/admin/assets/"+strconv.Itoa(id)+"/delete", url.Values{})
	if remaining, _ := st.ListAssets(store.AssetListParams{Project: "p1"}); len(remaining) != 0 {
		t.Errorf("after delete: got %d assets, want 0", len(remaining))
	}
}

func doForm(t *testing.T, mux *http.ServeMux, path string, form url.Values) {
	t.Helper()
	req := httptest.NewRequest(http.MethodPost, path, strings.NewReader(form.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)
	if rec.Code != http.StatusSeeOther && rec.Code != http.StatusFound {
		t.Fatalf("POST %s: expected redirect, got %d (body: %s)", path, rec.Code, rec.Body.String())
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
