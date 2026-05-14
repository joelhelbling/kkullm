package web

import (
	"net/http"
	"net/http/httptest"
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
