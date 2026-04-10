package web

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/joelhelbling/kkullm/api"
	"github.com/joelhelbling/kkullm/db"
	"github.com/joelhelbling/kkullm/store"
)

func setupTestMuxWithStore(t *testing.T) (*http.ServeMux, *store.Store) {
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
	srv := api.NewServer(s)
	mux := http.NewServeMux()
	RegisterRoutes(mux, s, srv.EventBus())
	return mux, s
}

func setupTestMux(t *testing.T) *http.ServeMux {
	mux, _ := setupTestMuxWithStore(t)
	return mux
}

func TestStaticFileServing(t *testing.T) {
	mux := setupTestMux(t)
	ts := httptest.NewServer(mux)
	defer ts.Close()

	resp, err := http.Get(ts.URL + "/static/css/app.css")
	if err != nil {
		t.Fatalf("GET /static/css/app.css: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != 200 {
		t.Fatalf("expected 200, got %d", resp.StatusCode)
	}

	ct := resp.Header.Get("Content-Type")
	if !strings.Contains(ct, "text/css") {
		t.Errorf("expected Content-Type containing 'text/css', got %q", ct)
	}
}

func TestStaticVendorJS(t *testing.T) {
	mux := setupTestMux(t)
	ts := httptest.NewServer(mux)
	defer ts.Close()

	for _, file := range []string{"htmx.min.js", "alpine.min.js", "sortable.min.js"} {
		resp, err := http.Get(ts.URL + "/static/vendor/" + file)
		if err != nil {
			t.Fatalf("GET /static/vendor/%s: %v", file, err)
		}
		resp.Body.Close()

		if resp.StatusCode != 200 {
			t.Errorf("expected 200 for %s, got %d", file, resp.StatusCode)
		}

		ct := resp.Header.Get("Content-Type")
		if !strings.Contains(ct, "javascript") {
			t.Errorf("expected JS Content-Type for %s, got %q", file, ct)
		}
	}
}
