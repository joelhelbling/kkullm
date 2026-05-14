package web

import (
	"net/http"
	"net/http/httptest"
	"net/url"
	"strconv"
	"strings"
	"testing"
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

func TestAdminRenameProject_OK(t *testing.T) {
	mux, st := setupTestMuxWithStore(t)
	p, err := st.CreateProject("orig", "")
	if err != nil {
		t.Fatalf("CreateProject: %v", err)
	}

	form := url.Values{"name": {"new"}}
	req := httptest.NewRequest(http.MethodPost, "/admin/projects/"+strconv.Itoa(p.ID)+"/rename",
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
	if got.Name != "new" {
		t.Errorf("expected name 'new', got %q", got.Name)
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
