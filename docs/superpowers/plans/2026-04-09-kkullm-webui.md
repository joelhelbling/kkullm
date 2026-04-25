# Kkullm Web UI Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Build a kanban board web UI embedded in the existing Go binary, with project/agent board views, a card detail drawer, a blockers column, drag-and-drop status changes, SSE real-time updates, and light/dark theming.

**Architecture:** A new `web/` package handles HTML fragment rendering via Go templates. The existing `/api/` JSON endpoints remain unchanged. A new `/ui/` route namespace returns HTML fragments consumed by htmx. Static assets (CSS, JS, vendor libs) are embedded via `//go:embed` and served at `/static/`. The `api.EventBus` is shared with the web package so status changes publish SSE events.

**Tech Stack:** Go `html/template`, htmx 2.x, Alpine.js 3.x, SortableJS, CSS custom properties

**Spec:** `docs/superpowers/specs/2026-04-09-kkullm-webui-design.md`

---

## File Structure

```
web/
├── web.go              # //go:embed, RegisterRoutes(mux, store, eventBus)
├── handlers.go         # /ui/ route handlers (board, drawer, blockers, status patch)
├── handlers_test.go    # Integration tests for /ui/ endpoints
├── templates/
│   ├── layout.html     # Full page shell (nav, board container, drawer, scripts)
│   ├── board.html      # Kanban columns with card tiles
│   ├── card.html       # Single card tile template
│   ├── drawer.html     # Card detail drawer
│   └── blockers.html   # Blocked column fragment
├── static/
│   ├── css/
│   │   └── app.css     # All styles + CSS custom properties for light/dark
│   ├── js/
│   │   └── app.js      # Alpine components, SortableJS init, SSE, dark mode, FLIP
│   └── vendor/
│       ├── htmx.min.js
│       ├── alpine.min.js
│       └── sortable.min.js
```

**Modified files:**
- `cmd/serve.go` — import `web` package, call `RegisterRoutes`
- `api/server.go` — export `EventBus` via a getter on `Server`

---

## Task 1: Expose EventBus from API Server

The web package needs access to the same `EventBus` the API handlers publish to. Add a getter method.

**Files:**
- Modify: `api/server.go`
- Test: `api/server_test.go`

- [x] **Step 1: Write the test**

Add to `api/server_test.go`:

```go
func TestServerEventBus(t *testing.T) {
	database, err := db.Open(":memory:")
	if err != nil {
		t.Fatalf("Open: %v", err)
	}
	defer database.Close()
	if err := db.Migrate(database); err != nil {
		t.Fatalf("Migrate: %v", err)
	}

	s := NewServer(store.New(database))
	eb := s.EventBus()
	if eb == nil {
		t.Fatal("expected non-nil EventBus")
	}

	// Verify it's functional
	ch := eb.Subscribe()
	defer eb.Unsubscribe(ch)

	eb.Publish(Event{Type: "test", Data: "hello"})

	select {
	case e := <-ch:
		if e.Type != "test" {
			t.Errorf("expected event type 'test', got %q", e.Type)
		}
	default:
		t.Fatal("expected to receive event")
	}
}
```

- [x] **Step 2: Run test to verify it fails**

Run: `cd /Users/joelhelbling/code/ai/kkullm && go test ./api/ -run TestServerEventBus -v`
Expected: FAIL with "s.EventBus undefined"

- [x] **Step 3: Add the EventBus getter**

Add to `api/server.go`, after the `NewServer` function:

```go
func (s *Server) EventBus() *EventBus {
	return s.events
}
```

- [x] **Step 4: Run test to verify it passes**

Run: `cd /Users/joelhelbling/code/ai/kkullm && go test ./api/ -run TestServerEventBus -v`
Expected: PASS

- [x] **Step 5: Run all existing tests to confirm no regressions**

Run: `cd /Users/joelhelbling/code/ai/kkullm && go test ./...`
Expected: All tests pass

- [x] **Step 6: Commit**

```bash
git add api/server.go api/server_test.go
git commit -m "feat: expose EventBus getter on API server for web package"
```

---

## Task 2: Web Package Scaffolding and Static File Serving

Create the `web/` package with `//go:embed`, vendor JS files, and static file serving. No templates yet — just the package structure and the ability to serve `/static/*`.

**Files:**
- Create: `web/web.go`
- Create: `web/static/css/app.css` (minimal placeholder)
- Create: `web/static/js/app.js` (minimal placeholder)
- Create: `web/static/vendor/htmx.min.js`
- Create: `web/static/vendor/alpine.min.js`
- Create: `web/static/vendor/sortable.min.js`
- Modify: `cmd/serve.go`
- Test: `web/web_test.go`

- [x] **Step 1: Download vendor JS files**

```bash
cd /Users/joelhelbling/code/ai/kkullm
mkdir -p web/static/vendor web/static/css web/static/js web/templates
curl -sL https://unpkg.com/htmx.org@2.0.4/dist/htmx.min.js -o web/static/vendor/htmx.min.js
curl -sL https://unpkg.com/alpinejs@3.14.8/dist/cdn.min.js -o web/static/vendor/alpine.min.js
curl -sL https://unpkg.com/sortablejs@1.15.6/Sortable.min.js -o web/static/vendor/sortable.min.js
```

Verify each file is non-empty:

```bash
wc -c web/static/vendor/*.js
```

Expected: Each file should be several KB (htmx ~50KB, Alpine ~45KB, Sortable ~40KB).

- [x] **Step 2: Create placeholder CSS**

Create `web/static/css/app.css`:

```css
/* Kkullm Web UI Styles */
:root {
  --bg-page: #f0f2f5;
  --bg-surface: #ffffff;
  --bg-card: #f8f9fa;
  --border: #e1e4e8;
  --text-primary: #24292f;
  --text-secondary: #656d76;
}
```

- [x] **Step 3: Create placeholder JS**

Create `web/static/js/app.js`:

```js
// Kkullm Web UI
document.addEventListener('DOMContentLoaded', function() {
  console.log('Kkullm UI loaded');
});
```

- [x] **Step 4: Create web.go with embed and RegisterRoutes**

Create `web/web.go`:

```go
package web

import (
	"embed"
	"io/fs"
	"net/http"

	"github.com/joelhelbling/kkullm/api"
	"github.com/joelhelbling/kkullm/store"
)

//go:embed static templates
var content embed.FS

type WebServer struct {
	store  *store.Store
	events *api.EventBus
}

func RegisterRoutes(mux *http.ServeMux, s *store.Store, events *api.EventBus) {
	ws := &WebServer{store: s, events: events}
	_ = ws // handlers added in later tasks

	// Static files
	staticFS, _ := fs.Sub(content, "static")
	mux.Handle("GET /static/", http.StripPrefix("/static/", http.FileServer(http.FS(staticFS))))
}
```

- [x] **Step 5: Create a placeholder template file so embed works**

Create `web/templates/layout.html`:

```html
<!DOCTYPE html>
<html lang="en">
<head>
  <meta charset="UTF-8">
  <meta name="viewport" content="width=device-width, initial-scale=1.0">
  <title>Kkullm</title>
  <link rel="stylesheet" href="/static/css/app.css">
</head>
<body>
  <p>Kkullm — coming soon</p>
  <script src="/static/vendor/htmx.min.js"></script>
  <script src="/static/vendor/alpine.min.js"></script>
  <script src="/static/vendor/sortable.min.js"></script>
  <script src="/static/js/app.js"></script>
</body>
</html>
```

- [x] **Step 6: Write the test**

Create `web/web_test.go`:

```go
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

func setupTestMux(t *testing.T) *http.ServeMux {
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
	}
}
```

- [x] **Step 7: Run test to verify it passes**

Run: `cd /Users/joelhelbling/code/ai/kkullm && go test ./web/ -v`
Expected: PASS

- [x] **Step 8: Update cmd/serve.go to register web routes**

Modify `cmd/serve.go` to import the web package and register routes on the same mux:

```go
package cmd

import (
	"fmt"
	"log"
	"net/http"

	"github.com/joelhelbling/kkullm/api"
	"github.com/joelhelbling/kkullm/db"
	"github.com/joelhelbling/kkullm/store"
	"github.com/joelhelbling/kkullm/web"
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

		s := api.NewServer(store.New(database))
		mux := http.NewServeMux()

		// Mount API routes
		apiHandler := s.Handler()
		mux.Handle("/api/", apiHandler)

		// Mount web UI routes
		web.RegisterRoutes(mux, store.New(database), s.EventBus())

		log.Printf("Kkullm server listening on %s", serveAddr)
		return http.ListenAndServe(serveAddr, mux)
	},
}

func init() {
	serveCmd.Flags().StringVar(&serveAddr, "addr", ":8080", "Listen address")
	serveCmd.Flags().StringVar(&dbPath, "db", "kkullm.db", "Database file path")
	rootCmd.AddCommand(serveCmd)
}
```

Note: The API server's `Handler()` currently returns its own mux. We need the API routes mounted under the top-level mux. The existing `Handler()` returns an `http.Handler` which we can mount at `/api/`. But since the API routes already have `/api/` prefixes in their patterns, we need to delegate directly. Replace the approach:

```go
		// Get the API handler (which is its own mux with /api/ prefixed routes)
		apiHandler := s.Handler()

		// Mount web UI routes on a new top-level mux
		mux := http.NewServeMux()
		web.RegisterRoutes(mux, store.New(database), s.EventBus())

		// Create a combined handler that tries web routes first, falls back to API
		combined := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if strings.HasPrefix(r.URL.Path, "/api/") {
				apiHandler.ServeHTTP(w, r)
				return
			}
			mux.ServeHTTP(w, r)
		})

		log.Printf("Kkullm server listening on %s", serveAddr)
		return http.ListenAndServe(serveAddr, combined)
```

Actually, simpler approach: have `api.Server.RegisterRoutes(mux)` register on an external mux instead of creating its own. But that changes the API package. Let's keep it simple and just use the combined handler approach above.

Final `cmd/serve.go`:

```go
package cmd

import (
	"fmt"
	"log"
	"net/http"
	"strings"

	"github.com/joelhelbling/kkullm/api"
	"github.com/joelhelbling/kkullm/db"
	"github.com/joelhelbling/kkullm/store"
	"github.com/joelhelbling/kkullm/web"
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

		st := store.New(database)
		apiSrv := api.NewServer(st)
		apiHandler := apiSrv.Handler()

		webMux := http.NewServeMux()
		web.RegisterRoutes(webMux, st, apiSrv.EventBus())

		handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if strings.HasPrefix(r.URL.Path, "/api/") {
				apiHandler.ServeHTTP(w, r)
				return
			}
			webMux.ServeHTTP(w, r)
		})

		log.Printf("Kkullm server listening on %s", serveAddr)
		return http.ListenAndServe(serveAddr, handler)
	},
}

func init() {
	serveCmd.Flags().StringVar(&serveAddr, "addr", ":8080", "Listen address")
	serveCmd.Flags().StringVar(&dbPath, "db", "kkullm.db", "Database file path")
	rootCmd.AddCommand(serveCmd)
}
```

- [x] **Step 9: Run all tests**

Run: `cd /Users/joelhelbling/code/ai/kkullm && go test ./...`
Expected: All tests pass

- [x] **Step 10: Commit**

```bash
git add web/ cmd/serve.go
git commit -m "feat: add web package scaffolding with static file serving"
```

---

## Task 3: CSS Styles with Light/Dark Theming

Write the complete CSS with all styles for the board, cards, columns, drawer, blockers, nav bar, and dark mode.

**Files:**
- Modify: `web/static/css/app.css`

- [x] **Step 1: Write the complete CSS**

Replace `web/static/css/app.css` with the full stylesheet. The CSS uses custom properties on `:root` (light mode) and `[data-theme="dark"]` (dark mode).

```css
/* ===== Reset & Base ===== */
*, *::before, *::after {
  box-sizing: border-box;
  margin: 0;
  padding: 0;
}

body {
  font-family: -apple-system, BlinkMacSystemFont, "Segoe UI", Helvetica, Arial, sans-serif;
  background: var(--bg-page);
  color: var(--text-primary);
  line-height: 1.5;
  overflow: hidden;
  height: 100vh;
}

/* ===== Light Theme (default) ===== */
:root {
  --bg-page: #f0f2f5;
  --bg-surface: #ffffff;
  --bg-card: #f8f9fa;
  --bg-card-hover: #f1f3f5;
  --border: #e1e4e8;
  --border-light: #eef0f2;
  --text-primary: #24292f;
  --text-secondary: #656d76;
  --text-link: #0969da;

  --accent-blocked: #dc3545;
  --accent-blocked-bg: #fff5f5;
  --accent-blocked-border: #f5c6cb;
  --accent-inflight: #1a7f37;
  --accent-inflight-bg: #dafbe1;
  --accent-completed: #8250df;

  --badge-blue-bg: #ddf4ff;
  --badge-blue-text: #0969da;
  --badge-green-bg: #dafbe1;
  --badge-green-text: #1a7f37;
  --badge-yellow-bg: #fff8c5;
  --badge-yellow-text: #9a6700;
  --badge-red-bg: #ffebe9;
  --badge-red-text: #cf222e;
  --badge-purple-bg: #fbefff;
  --badge-purple-text: #8250df;

  --highlight-glow: rgba(255, 213, 79, 0.5);
  --shadow-sm: 0 1px 2px rgba(0, 0, 0, 0.05);
  --shadow-md: 0 4px 12px rgba(0, 0, 0, 0.1);
  --drawer-width: 440px;
}

/* ===== Dark Theme ===== */
[data-theme="dark"] {
  --bg-page: #0d1117;
  --bg-surface: #161b22;
  --bg-card: #1c2128;
  --bg-card-hover: #22272e;
  --border: #30363d;
  --border-light: #21262d;
  --text-primary: #e6edf3;
  --text-secondary: #7d8590;
  --text-link: #58a6ff;

  --accent-blocked: #f85149;
  --accent-blocked-bg: #1a0d0d;
  --accent-blocked-border: #4d2020;
  --accent-inflight: #3fb950;
  --accent-inflight-bg: #0d1a0f;
  --accent-completed: #bc8cff;

  --badge-blue-bg: #1f6feb33;
  --badge-blue-text: #58a6ff;
  --badge-green-bg: #23883333;
  --badge-green-text: #3fb950;
  --badge-yellow-bg: #9e6a0333;
  --badge-yellow-text: #d29922;
  --badge-red-bg: #da363333;
  --badge-red-text: #f85149;
  --badge-purple-bg: #8957e533;
  --badge-purple-text: #bc8cff;

  --highlight-glow: rgba(255, 213, 79, 0.25);
  --shadow-sm: 0 1px 2px rgba(0, 0, 0, 0.2);
  --shadow-md: 0 4px 12px rgba(0, 0, 0, 0.3);
}

/* ===== Nav Bar ===== */
.nav {
  background: var(--bg-surface);
  border-bottom: 1px solid var(--border);
  padding: 8px 16px;
  display: flex;
  align-items: center;
  justify-content: space-between;
  height: 48px;
  position: fixed;
  top: 0;
  left: 0;
  right: 0;
  z-index: 100;
}

.nav-left {
  display: flex;
  align-items: center;
  gap: 16px;
}

.nav-right {
  display: flex;
  align-items: center;
  gap: 12px;
}

.nav-logo {
  font-weight: 700;
  font-size: 15px;
  color: var(--text-primary);
  text-decoration: none;
}

.nav-select {
  border: 1px solid var(--border);
  border-radius: 6px;
  padding: 4px 8px;
  font-size: 12px;
  background: var(--bg-surface);
  color: var(--text-primary);
  cursor: pointer;
}

.nav-blockers {
  background: var(--accent-blocked);
  color: white;
  padding: 3px 10px;
  border-radius: 12px;
  font-size: 12px;
  font-weight: 600;
  cursor: pointer;
  border: none;
  transition: opacity 0.2s;
}

.nav-blockers:hover {
  opacity: 0.9;
}

.nav-blockers.hide-mode {
  background: #856404;
}

.nav-theme-toggle {
  cursor: pointer;
  font-size: 16px;
  background: none;
  border: none;
  padding: 4px;
  line-height: 1;
}

/* ===== Board ===== */
.board {
  display: flex;
  gap: 12px;
  padding: 16px;
  padding-top: 64px; /* below fixed nav */
  height: 100vh;
  overflow-x: auto;
  align-items: flex-start;
}

/* ===== Columns ===== */
.column {
  background: var(--bg-surface);
  border-radius: 8px;
  border: 1px solid var(--border);
  min-width: 220px;
  flex: 1;
  max-height: calc(100vh - 80px);
  display: flex;
  flex-direction: column;
  transition: flex 0.3s ease, opacity 0.3s ease, max-width 0.3s ease;
}

.column-header {
  padding: 10px 12px;
  border-bottom: 1px solid var(--border);
  font-size: 12px;
  font-weight: 600;
  color: var(--text-secondary);
  text-transform: uppercase;
  letter-spacing: 0.5px;
  display: flex;
  justify-content: space-between;
  align-items: center;
  flex-shrink: 0;
}

.column-count {
  background: var(--border);
  border-radius: 10px;
  padding: 0 6px;
  font-size: 11px;
  min-width: 18px;
  text-align: center;
}

.column-cards {
  padding: 8px;
  overflow-y: auto;
  flex: 1;
  min-height: 40px;
}

/* ===== Blocked Column ===== */
.column-blocked {
  background: var(--accent-blocked-bg);
  border: 2px solid var(--accent-blocked);
  box-shadow: 0 0 8px rgba(220, 53, 69, 0.15);
}

[data-theme="dark"] .column-blocked {
  box-shadow: 0 0 8px rgba(248, 81, 73, 0.15);
}

.column-blocked .column-header {
  color: var(--accent-blocked);
  font-weight: 700;
  border-bottom-color: var(--accent-blocked-border);
}

.column-blocked .card-tile {
  background: var(--bg-surface);
  border-color: var(--accent-blocked-border);
}

/* Blocked column slide animation */
.column-blocked-enter {
  max-width: 0;
  min-width: 0;
  opacity: 0;
  padding: 0;
  border-width: 0;
  overflow: hidden;
}

.column-blocked-active {
  max-width: 300px;
  min-width: 220px;
  opacity: 1;
}

/* ===== Card Tiles ===== */
.card-tile {
  background: var(--bg-card);
  border: 1px solid var(--border);
  border-radius: 6px;
  padding: 10px;
  margin-bottom: 8px;
  cursor: pointer;
  transition: background 0.15s, box-shadow 0.15s;
}

.card-tile:hover {
  background: var(--bg-card-hover);
  box-shadow: var(--shadow-sm);
}

.card-tile:last-child {
  margin-bottom: 0;
}

.card-tile.in-flight {
  border-left: 3px solid var(--accent-inflight);
}

.card-tile-title {
  font-size: 13px;
  font-weight: 500;
  color: var(--text-primary);
  margin-bottom: 6px;
}

.card-tile-project {
  margin-bottom: 5px;
}

.card-tile-tags {
  display: flex;
  gap: 4px;
  flex-wrap: wrap;
  margin-bottom: 6px;
}

.card-tile-footer {
  display: flex;
  justify-content: space-between;
  align-items: center;
  font-size: 11px;
  color: var(--text-secondary);
}

.card-tile-meta {
  display: flex;
  gap: 8px;
  align-items: center;
}

/* ===== Badges & Pills ===== */
.tag {
  font-size: 10px;
  padding: 1px 6px;
  border-radius: 10px;
  font-weight: 500;
}

.project-badge {
  font-size: 9px;
  padding: 1px 5px;
  border-radius: 3px;
  font-weight: 600;
}

.status-pill {
  font-size: 11px;
  padding: 3px 10px;
  border-radius: 12px;
  cursor: pointer;
  border: 1px solid var(--border);
  background: var(--bg-card);
  color: var(--text-secondary);
  transition: background 0.15s;
}

.status-pill:hover {
  background: var(--bg-card-hover);
}

.status-pill.active {
  font-weight: 600;
  border-width: 2px;
}

.status-pill.active-considering {
  background: var(--badge-yellow-bg);
  color: var(--badge-yellow-text);
  border-color: var(--badge-yellow-text);
}

.status-pill.active-todo {
  background: var(--badge-blue-bg);
  color: var(--badge-blue-text);
  border-color: var(--badge-blue-text);
}

.status-pill.active-in_flight {
  background: var(--badge-green-bg);
  color: var(--badge-green-text);
  border-color: var(--badge-green-text);
}

.status-pill.active-completed {
  background: var(--badge-purple-bg);
  color: var(--badge-purple-text);
  border-color: var(--badge-purple-text);
}

.status-pill.active-blocked {
  background: var(--badge-red-bg);
  color: var(--badge-red-text);
  border-color: var(--badge-red-text);
}

.status-pill.active-tabled {
  background: var(--bg-card);
  color: var(--text-secondary);
  border-color: var(--text-secondary);
}

/* ===== Drawer ===== */
.drawer-overlay {
  position: fixed;
  top: 0;
  left: 0;
  right: 0;
  bottom: 0;
  background: rgba(0, 0, 0, 0.3);
  z-index: 200;
  opacity: 0;
  pointer-events: none;
  transition: opacity 0.2s;
}

.drawer-overlay.open {
  opacity: 1;
  pointer-events: auto;
}

.drawer {
  position: fixed;
  top: 0;
  right: 0;
  bottom: 0;
  width: var(--drawer-width);
  background: var(--bg-surface);
  border-left: 1px solid var(--border);
  z-index: 201;
  overflow-y: auto;
  transform: translateX(100%);
  transition: transform 0.25s ease;
}

.drawer.open {
  transform: translateX(0);
}

.drawer-header {
  padding: 16px 20px;
  border-bottom: 1px solid var(--border);
  display: flex;
  justify-content: space-between;
  align-items: flex-start;
}

.drawer-card-id {
  font-size: 11px;
  color: var(--text-secondary);
  margin-bottom: 4px;
}

.drawer-title {
  font-size: 17px;
  font-weight: 600;
  color: var(--text-primary);
}

.drawer-close {
  cursor: pointer;
  color: var(--text-secondary);
  font-size: 18px;
  background: none;
  border: none;
  padding: 0 4px;
  line-height: 1;
}

.drawer-close:hover {
  color: var(--text-primary);
}

.drawer-section {
  padding: 12px 20px;
  border-bottom: 1px solid var(--border);
}

.drawer-section-label {
  font-size: 11px;
  font-weight: 600;
  color: var(--text-secondary);
  text-transform: uppercase;
  letter-spacing: 0.5px;
  margin-bottom: 6px;
}

.drawer-status-pills {
  display: flex;
  gap: 6px;
  flex-wrap: wrap;
}

.drawer-meta {
  display: flex;
  gap: 24px;
}

.drawer-meta-item {
  font-size: 13px;
  color: var(--text-primary);
}

.drawer-relation {
  margin-bottom: 4px;
  font-size: 13px;
}

.drawer-relation-type {
  color: var(--text-secondary);
  font-size: 11px;
}

.drawer-relation-link {
  color: var(--text-link);
  text-decoration: none;
  cursor: pointer;
}

.drawer-relation-link:hover {
  text-decoration: underline;
}

.drawer-body {
  font-size: 13px;
  color: var(--text-primary);
  line-height: 1.5;
}

/* ===== Comments ===== */
.comment {
  margin-bottom: 12px;
}

.comment:last-child {
  margin-bottom: 0;
}

.comment-header {
  display: flex;
  justify-content: space-between;
  margin-bottom: 2px;
}

.comment-agent {
  font-size: 12px;
  font-weight: 600;
  color: var(--text-primary);
}

.comment-agent.user-agent {
  color: var(--text-link);
}

.comment-time {
  font-size: 11px;
  color: var(--text-secondary);
}

.comment-body {
  font-size: 13px;
  color: var(--text-primary);
  line-height: 1.4;
}

/* ===== Closed Status Toggle ===== */
.closed-toggle {
  padding: 0 16px 12px;
  font-size: 11px;
  color: var(--text-secondary);
  cursor: pointer;
  position: fixed;
  bottom: 0;
  left: 0;
  right: 0;
  background: var(--bg-page);
  padding: 8px 16px;
}

.closed-toggle:hover {
  color: var(--text-primary);
}

/* ===== Animations ===== */
@keyframes highlight-glow {
  0% { box-shadow: 0 0 8px var(--highlight-glow); }
  100% { box-shadow: none; }
}

.card-tile.highlight {
  animation: highlight-glow 1.5s ease-out;
}

@keyframes slide-in-down {
  from {
    opacity: 0;
    transform: translateY(-10px);
  }
  to {
    opacity: 1;
    transform: translateY(0);
  }
}

.card-tile.slide-in {
  animation: slide-in-down 0.3s ease-out;
}

@keyframes fade-out {
  from { opacity: 1; }
  to { opacity: 0; transform: scale(0.95); }
}

.card-tile.fade-out {
  animation: fade-out 0.3s ease-out forwards;
}

/* ===== Toast ===== */
.toast {
  position: fixed;
  bottom: 20px;
  right: 20px;
  background: var(--accent-blocked);
  color: white;
  padding: 10px 16px;
  border-radius: 6px;
  font-size: 13px;
  z-index: 300;
  box-shadow: var(--shadow-md);
  animation: slide-in-down 0.2s ease-out;
}

/* ===== SortableJS ghost/drag ===== */
.sortable-ghost {
  opacity: 0.4;
}

.sortable-chosen {
  box-shadow: var(--shadow-md);
}

/* ===== Utilities ===== */
.hidden {
  display: none !important;
}
```

- [x] **Step 2: Verify CSS loads correctly**

Run: `cd /Users/joelhelbling/code/ai/kkullm && go test ./web/ -run TestStaticFileServing -v`
Expected: PASS

- [x] **Step 3: Commit**

```bash
git add web/static/css/app.css
git commit -m "feat: add complete CSS with light/dark theming"
```

---

## Task 4: Layout Template and Root Handler

Create the full page shell template and the `GET /` handler.

**Files:**
- Modify: `web/templates/layout.html`
- Create: `web/handlers.go`
- Modify: `web/web.go`
- Test: `web/handlers_test.go`

- [x] **Step 1: Write the test**

Create `web/handlers_test.go`:

```go
package web

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestRootHandler(t *testing.T) {
	mux := setupTestMux(t)
	ts := httptest.NewServer(mux)
	defer ts.Close()

	resp, err := http.Get(ts.URL + "/")
	if err != nil {
		t.Fatalf("GET /: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != 200 {
		t.Fatalf("expected 200, got %d", resp.StatusCode)
	}

	ct := resp.Header.Get("Content-Type")
	if !strings.Contains(ct, "text/html") {
		t.Errorf("expected text/html, got %q", ct)
	}

	buf := new(strings.Builder)
	buf.ReadFrom(resp.Body)
	body := buf.String()

	// Check key elements are present
	checks := []string{
		"kkullm",                    // logo text
		"/static/css/app.css",       // CSS link
		"/static/vendor/htmx.min.js", // htmx
		"x-data",                     // Alpine.js binding
		"hx-get",                     // htmx attribute
	}
	for _, check := range checks {
		if !strings.Contains(body, check) {
			t.Errorf("expected body to contain %q", check)
		}
	}
}
```

- [x] **Step 2: Run test to verify it fails**

Run: `cd /Users/joelhelbling/code/ai/kkullm && go test ./web/ -run TestRootHandler -v`
Expected: FAIL (404 — no root handler registered yet)

- [x] **Step 3: Write the layout template**

Replace `web/templates/layout.html`:

```html
<!DOCTYPE html>
<html lang="en">
<head>
  <meta charset="UTF-8">
  <meta name="viewport" content="width=device-width, initial-scale=1.0">
  <title>Kkullm</title>
  <link rel="stylesheet" href="/static/css/app.css">
  <script src="/static/vendor/htmx.min.js"></script>
  <script defer src="/static/vendor/alpine.min.js"></script>
  <script src="/static/vendor/sortable.min.js"></script>
</head>
<body x-data="kkullm()" x-init="init()">

  <!-- Nav Bar -->
  <nav class="nav">
    <div class="nav-left">
      <a href="/" class="nav-logo">끌림 kkullm</a>
      <template x-if="viewMode === 'project'">
        <select class="nav-select" x-model="currentProject" @change="loadBoard()">
          {{range .Projects}}
          <option value="{{.ID}}">Project: {{.Name}}</option>
          {{end}}
        </select>
      </template>
      <template x-if="viewMode === 'agent'">
        <select class="nav-select" x-model="currentAgent" @change="loadBoard()">
          {{range .Agents}}
          <option value="{{.ID}}">Agent: {{.Name}}</option>
          {{end}}
        </select>
      </template>
      <select class="nav-select" x-model="viewMode" @change="loadBoard()">
        <option value="project">View: Project Board</option>
        <option value="agent">View: Agent Board</option>
      </select>
    </div>
    <div class="nav-right">
      <button
        class="nav-blockers"
        :class="{ 'hide-mode': blockersOpen }"
        x-show="blockerCount > 0"
        @click="toggleBlockers()"
        x-text="blockersOpen ? 'Hide Blocked ✕' : 'Blocked! (' + blockerCount + ')'"
      ></button>
      <button class="nav-theme-toggle" @click="toggleTheme()" x-text="theme === 'dark' ? '☀' : '🌙'"></button>
    </div>
  </nav>

  <!-- Board container (populated by htmx) -->
  <div id="board-container"
       hx-get="/ui/board?project={{.DefaultProjectID}}"
       hx-trigger="load"
       hx-target="#board-container"
       hx-swap="innerHTML">
  </div>

  <!-- Drawer overlay -->
  <div class="drawer-overlay" :class="{ open: drawerOpen }" @click="closeDrawer()"></div>

  <!-- Drawer -->
  <div class="drawer" :class="{ open: drawerOpen }" id="drawer-container">
  </div>

  <!-- Toast container -->
  <div id="toast-container"></div>

  <script src="/static/js/app.js"></script>
</body>
</html>
```

- [x] **Step 4: Write the handlers.go**

Create `web/handlers.go`:

```go
package web

import (
	"html/template"
	"log"
	"net/http"

	"github.com/joelhelbling/kkullm/model"
)

var tmpl *template.Template

func initTemplates() {
	var err error
	tmpl, err = template.ParseFS(content, "templates/*.html")
	if err != nil {
		log.Fatalf("parse templates: %v", err)
	}
}

type layoutData struct {
	Projects         []model.Project
	Agents           []model.Agent
	DefaultProjectID int
}

func (ws *WebServer) handleRoot(w http.ResponseWriter, r *http.Request) {
	if r.URL.Path != "/" {
		http.NotFound(w, r)
		return
	}

	projects, err := ws.store.ListProjects()
	if err != nil {
		http.Error(w, err.Error(), 500)
		return
	}

	agents, err := ws.store.ListAgents("")
	if err != nil {
		http.Error(w, err.Error(), 500)
		return
	}

	defaultProjectID := 0
	if len(projects) > 0 {
		defaultProjectID = projects[0].ID
	}

	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	if err := tmpl.ExecuteTemplate(w, "layout.html", layoutData{
		Projects:         projects,
		Agents:           agents,
		DefaultProjectID: defaultProjectID,
	}); err != nil {
		log.Printf("render layout: %v", err)
	}
}
```

- [x] **Step 5: Update web.go to register the root handler and init templates**

Modify `web/web.go` — add `initTemplates()` call and root route:

```go
package web

import (
	"embed"
	"io/fs"
	"net/http"

	"github.com/joelhelbling/kkullm/api"
	"github.com/joelhelbling/kkullm/store"
)

//go:embed static templates
var content embed.FS

type WebServer struct {
	store  *store.Store
	events *api.EventBus
}

func RegisterRoutes(mux *http.ServeMux, s *store.Store, events *api.EventBus) {
	initTemplates()

	ws := &WebServer{store: s, events: events}

	// Root page
	mux.HandleFunc("GET /", ws.handleRoot)

	// Static files
	staticFS, _ := fs.Sub(content, "static")
	mux.Handle("GET /static/", http.StripPrefix("/static/", http.FileServer(http.FS(staticFS))))
}
```

- [x] **Step 6: Run test to verify it passes**

Run: `cd /Users/joelhelbling/code/ai/kkullm && go test ./web/ -run TestRootHandler -v`
Expected: PASS

- [x] **Step 7: Run all tests**

Run: `cd /Users/joelhelbling/code/ai/kkullm && go test ./...`
Expected: All pass

- [x] **Step 8: Commit**

```bash
git add web/templates/layout.html web/handlers.go web/web.go web/handlers_test.go
git commit -m "feat: add layout template and root handler"
```

---

## Task 5: Card Tile Template and Board Handler

Implement the board view with kanban columns and card tiles for both project-scoped and agent-scoped views.

**Files:**
- Create: `web/templates/board.html`
- Create: `web/templates/card.html`
- Modify: `web/handlers.go`
- Modify: `web/web.go`
- Test: `web/handlers_test.go`

- [x] **Step 1: Write the tests**

Add to `web/handlers_test.go`:

```go
func seedTestData(t *testing.T, mux *http.ServeMux) {
	t.Helper()
	// We need to seed via the API. Create a test server that has both API and web routes.
	// Instead, use the store directly from setupTestMux.
	// Since setupTestMux uses db.Seed which creates "orchestration" project and "user" agent,
	// we can use those. But we need additional data for board tests.
	// Let's create a helper that returns the store too.
}

// Refactor: expose store from setupTestMux
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

func TestBoardProjectScoped(t *testing.T) {
	mux, st := setupTestMuxWithStore(t)

	// Create a card in the orchestration project
	_, err := st.CreateCard(store.CardCreateParams{
		Title:     "Test card",
		Status:    "todo",
		ProjectID: 1, // orchestration project from seed
	})
	if err != nil {
		t.Fatalf("create card: %v", err)
	}

	ts := httptest.NewServer(mux)
	defer ts.Close()

	resp, err := http.Get(ts.URL + "/ui/board?project=1")
	if err != nil {
		t.Fatalf("GET /ui/board: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != 200 {
		t.Fatalf("expected 200, got %d", resp.StatusCode)
	}

	buf := new(strings.Builder)
	buf.ReadFrom(resp.Body)
	body := buf.String()

	if !strings.Contains(body, "Test card") {
		t.Error("expected board to contain card title")
	}
	if !strings.Contains(body, "data-status=\"todo\"") {
		t.Error("expected board to contain todo column")
	}
}

func TestBoardAgentScoped(t *testing.T) {
	mux, st := setupTestMuxWithStore(t)

	// Create a card assigned to the seeded "user" agent
	_, err := st.CreateCard(store.CardCreateParams{
		Title:     "Agent card",
		Status:    "in_flight",
		ProjectID: 1,
		Assignees: []string{"user"},
	})
	if err != nil {
		t.Fatalf("create card: %v", err)
	}

	ts := httptest.NewServer(mux)
	defer ts.Close()

	resp, err := http.Get(ts.URL + "/ui/board?agent=1")
	if err != nil {
		t.Fatalf("GET /ui/board?agent=1: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != 200 {
		t.Fatalf("expected 200, got %d", resp.StatusCode)
	}

	buf := new(strings.Builder)
	buf.ReadFrom(resp.Body)
	body := buf.String()

	if !strings.Contains(body, "Agent card") {
		t.Error("expected board to contain agent's card")
	}
}
```

Also update `setupTestMux` to use the new helper:

```go
func setupTestMux(t *testing.T) *http.ServeMux {
	mux, _ := setupTestMuxWithStore(t)
	return mux
}
```

Remove the old `setupTestMux` body and replace with the delegation above.

- [x] **Step 2: Run tests to verify they fail**

Run: `cd /Users/joelhelbling/code/ai/kkullm && go test ./web/ -run TestBoard -v`
Expected: FAIL (404 — no /ui/board handler)

- [x] **Step 3: Create the card tile template**

Create `web/templates/card.html`:

```html
{{define "card"}}
<div class="card-tile{{if eq .Status "in_flight"}} in-flight{{end}}"
     data-card-id="{{.ID}}"
     hx-get="/ui/cards/{{.ID}}/drawer"
     hx-target="#drawer-container"
     hx-swap="innerHTML"
     @htmx:after-swap.window="if ($event.detail.target.id === 'drawer-container') { drawerOpen = true }">
  {{if .ShowProject}}
  <div class="card-tile-project">
    <span class="project-badge" style="background:{{projectColor .Project}};color:white;">{{.Project}}</span>
  </div>
  {{end}}
  <div class="card-tile-title">{{.Title}}</div>
  {{if .Tags}}
  <div class="card-tile-tags">
    {{range .Tags}}
    <span class="tag" style="background:{{tagBg .}};color:{{tagColor .}};">{{.}}</span>
    {{end}}
  </div>
  {{end}}
  <div class="card-tile-footer">
    <span>{{if .Assignees}}{{joinStrings .Assignees ", "}}{{else}}unassigned{{end}}</span>
    <span class="card-tile-meta">
      {{if gt .CommentCount 0}}<span>💬 {{.CommentCount}}</span>{{end}}
      {{if .Relations}}<span>🔗 {{len .Relations}}</span>{{end}}
    </span>
  </div>
</div>
{{end}}
```

- [x] **Step 4: Create the board template**

Create `web/templates/board.html`:

```html
{{define "board"}}
<div class="board" id="board" x-ref="board">
  {{$blocked := .BlockedCards}}
  {{$showProject := .ShowProject}}

  <div class="column" data-status="considering">
    <div class="column-header">
      <span>Considering</span>
      <span class="column-count">{{len .Considering}}</span>
    </div>
    <div class="column-cards" data-status="considering">
      {{range .Considering}}{{template "card" .}}{{end}}
    </div>
  </div>

  <div class="column" data-status="todo">
    <div class="column-header">
      <span>Todo</span>
      <span class="column-count">{{len .Todo}}</span>
    </div>
    <div class="column-cards" data-status="todo">
      {{range .Todo}}{{template "card" .}}{{end}}
    </div>
  </div>

  <!-- Blocked column placeholder (inserted by Alpine when toggled) -->
  <div class="column column-blocked"
       x-show="blockersOpen"
       x-transition:enter="column-blocked-enter"
       x-transition:enter-start="column-blocked-enter"
       x-transition:enter-end="column-blocked-active"
       x-transition:leave="column-blocked-active"
       x-transition:leave-start="column-blocked-active"
       x-transition:leave-end="column-blocked-enter"
       id="blocked-column">
    <div class="column-header">
      <span>⚠ Blocked</span>
      <span class="column-count" id="blocked-count">{{len $blocked}}</span>
    </div>
    <div class="column-cards" id="blocked-cards"
         hx-get="/ui/blockers"
         hx-trigger="blockers-refresh from:body"
         hx-swap="innerHTML">
      {{range $blocked}}{{template "card" .}}{{end}}
    </div>
  </div>

  <div class="column" data-status="in_flight">
    <div class="column-header">
      <span>In Flight</span>
      <span class="column-count">{{len .InFlight}}</span>
    </div>
    <div class="column-cards" data-status="in_flight">
      {{range .InFlight}}{{template "card" .}}{{end}}
    </div>
  </div>

  <div class="column" data-status="completed">
    <div class="column-header">
      <span>Completed</span>
      <span class="column-count">{{len .Completed}}</span>
    </div>
    <div class="column-cards" data-status="completed">
      {{range .Completed}}{{template "card" .}}{{end}}
    </div>
  </div>

  {{if or (gt (len .Done) 0) (gt (len .Tabled) 0)}}
  <template x-if="showClosed">
    <div style="display:contents">
      <div class="column" data-status="done">
        <div class="column-header">
          <span>Done</span>
          <span class="column-count">{{len .Done}}</span>
        </div>
        <div class="column-cards" data-status="done">
          {{range .Done}}{{template "card" .}}{{end}}
        </div>
      </div>
      <div class="column" data-status="tabled">
        <div class="column-header">
          <span>Tabled</span>
          <span class="column-count">{{len .Tabled}}</span>
        </div>
        <div class="column-cards" data-status="tabled">
          {{range .Tabled}}{{template "card" .}}{{end}}
        </div>
      </div>
    </div>
  </template>
  {{end}}
</div>

{{if or (gt (len .Done) 0) (gt (len .Tabled) 0)}}
<div class="closed-toggle" @click="showClosed = !showClosed"
     x-text="showClosed ? '▼ Hide closed' : '▶ Show closed (done: {{len .Done}}, tabled: {{len .Tabled}})'">
</div>
{{end}}
{{end}}
```

- [x] **Step 5: Add template functions and board handler**

Add to `web/handlers.go`:

```go
// Template function map
var funcMap = template.FuncMap{
	"projectColor": projectColor,
	"tagBg":        tagBg,
	"tagColor":     tagColor,
	"joinStrings":  joinStrings,
	"timeAgo":      timeAgo,
}

// Project color palette — consistent hashing by name
var projectColors = []string{
	"#0969da", "#1a7f37", "#9a6700", "#cf222e", "#8250df",
	"#bf3989", "#0550ae", "#116329", "#7d4e00", "#a40e26",
}

func projectColor(name string) string {
	h := 0
	for _, c := range name {
		h = h*31 + int(c)
	}
	if h < 0 {
		h = -h
	}
	return projectColors[h%len(projectColors)]
}

// Tag color mappings
var tagColorMap = map[string][2]string{
	"bug":         {"#ffebe9", "#cf222e"},
	"feature":     {"#dafbe1", "#1a7f37"},
	"enhancement": {"#ddf4ff", "#0969da"},
	"docs":        {"#dafbe1", "#1a7f37"},
	"rfc":         {"#fff8c5", "#9a6700"},
	"infra":       {"#dafbe1", "#1a7f37"},
	"urgent":      {"#ffebe9", "#cf222e"},
}

var defaultTagColors = [2]string{"#ddf4ff", "#0969da"}

func tagBg(tag string) string {
	if colors, ok := tagColorMap[tag]; ok {
		return colors[0]
	}
	return defaultTagColors[0]
}

func tagColor(tag string) string {
	if colors, ok := tagColorMap[tag]; ok {
		return colors[1]
	}
	return defaultTagColors[1]
}

func joinStrings(strs []string, sep string) string {
	return strings.Join(strs, sep)
}

func timeAgo(t time.Time) string {
	d := time.Since(t)
	switch {
	case d < time.Minute:
		return "just now"
	case d < time.Hour:
		m := int(d.Minutes())
		if m == 1 {
			return "1m ago"
		}
		return fmt.Sprintf("%dm ago", m)
	case d < 24*time.Hour:
		h := int(d.Hours())
		if h == 1 {
			return "1h ago"
		}
		return fmt.Sprintf("%dh ago", h)
	default:
		days := int(d.Hours() / 24)
		if days == 1 {
			return "1d ago"
		}
		return fmt.Sprintf("%dd ago", days)
	}
}
```

Update `initTemplates()` to use funcMap:

```go
func initTemplates() {
	var err error
	tmpl, err = template.New("").Funcs(funcMap).ParseFS(content, "templates/*.html")
	if err != nil {
		log.Fatalf("parse templates: %v", err)
	}
}
```

Add the imports to the top of `handlers.go`:

```go
import (
	"fmt"
	"html/template"
	"log"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/joelhelbling/kkullm/model"
	"github.com/joelhelbling/kkullm/store"
)
```

Add a card view model that wraps model.Card with ShowProject:

```go
type cardView struct {
	model.Card
	ShowProject bool
}

type boardData struct {
	Considering  []cardView
	Todo         []cardView
	InFlight     []cardView
	Completed    []cardView
	Done         []cardView
	Tabled       []cardView
	BlockedCards []cardView
	ShowProject  bool
}

func groupCards(cards []model.Card, showProject bool) boardData {
	bd := boardData{ShowProject: showProject}
	for _, c := range cards {
		cv := cardView{Card: c, ShowProject: showProject}
		switch c.Status {
		case "considering":
			bd.Considering = append(bd.Considering, cv)
		case "todo":
			bd.Todo = append(bd.Todo, cv)
		case "in_flight":
			bd.InFlight = append(bd.InFlight, cv)
		case "completed":
			bd.Completed = append(bd.Completed, cv)
		case "done":
			bd.Done = append(bd.Done, cv)
		case "tabled":
			bd.Tabled = append(bd.Tabled, cv)
		case "blocked":
			bd.BlockedCards = append(bd.BlockedCards, cv)
		}
	}
	return bd
}
```

Add the board handler:

```go
func (ws *WebServer) handleBoard(w http.ResponseWriter, r *http.Request) {
	var cards []model.Card
	var showProject bool
	var err error

	if agentID := r.URL.Query().Get("agent"); agentID != "" {
		id, parseErr := strconv.Atoi(agentID)
		if parseErr != nil {
			http.Error(w, "invalid agent id", 400)
			return
		}
		agent, agentErr := ws.store.GetAgent(id)
		if agentErr != nil {
			http.Error(w, agentErr.Error(), 404)
			return
		}
		cards, err = ws.store.ListCards(store.CardListParams{Assignee: agent.Name})
		showProject = true
	} else {
		projectID := r.URL.Query().Get("project")
		if projectID == "" {
			projectID = "1"
		}
		id, parseErr := strconv.Atoi(projectID)
		if parseErr != nil {
			http.Error(w, "invalid project id", 400)
			return
		}
		project, projErr := ws.store.GetProject(id)
		if projErr != nil {
			http.Error(w, projErr.Error(), 404)
			return
		}
		cards, err = ws.store.ListCards(store.CardListParams{Project: project.Name})
		showProject = false
	}

	if err != nil {
		http.Error(w, err.Error(), 500)
		return
	}

	bd := groupCards(cards, showProject)

	// For blockers: always fetch all blocked cards assigned to user
	blockedCards, _ := ws.store.ListCards(store.CardListParams{
		Status:   "blocked",
		Assignee: "user",
	})
	for _, c := range blockedCards {
		bd.BlockedCards = append(bd.BlockedCards, cardView{Card: c, ShowProject: true})
	}

	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	if err := tmpl.ExecuteTemplate(w, "board", bd); err != nil {
		log.Printf("render board: %v", err)
	}
}
```

- [x] **Step 6: Register the board route in web.go**

Add to the `RegisterRoutes` function, after the root handler:

```go
	// Board view
	mux.HandleFunc("GET /ui/board", ws.handleBoard)
```

- [x] **Step 7: Run tests to verify they pass**

Run: `cd /Users/joelhelbling/code/ai/kkullm && go test ./web/ -run TestBoard -v`
Expected: PASS

- [x] **Step 8: Run all tests**

Run: `cd /Users/joelhelbling/code/ai/kkullm && go test ./...`
Expected: All pass

- [x] **Step 9: Commit**

```bash
git add web/templates/board.html web/templates/card.html web/handlers.go web/handlers_test.go web/web.go
git commit -m "feat: add board view with card tiles for project and agent scopes"
```

---

## Task 6: Card Detail Drawer

Implement the drawer template and handler for card detail view.

**Files:**
- Create: `web/templates/drawer.html`
- Modify: `web/handlers.go`
- Modify: `web/web.go`
- Test: `web/handlers_test.go`

- [x] **Step 1: Write the test**

Add to `web/handlers_test.go`:

```go
func TestDrawerHandler(t *testing.T) {
	mux, st := setupTestMuxWithStore(t)

	card, err := st.CreateCard(store.CardCreateParams{
		Title:     "Drawer test card",
		Body:      "This is the card body",
		Status:    "todo",
		ProjectID: 1,
		Assignees: []string{"user"},
		Tags:      []string{"bug"},
	})
	if err != nil {
		t.Fatalf("create card: %v", err)
	}

	// Add a comment
	_, err = st.CreateComment(card.ID, 1, "Test comment")
	if err != nil {
		t.Fatalf("create comment: %v", err)
	}

	ts := httptest.NewServer(mux)
	defer ts.Close()

	resp, err := http.Get(ts.URL + fmt.Sprintf("/ui/cards/%d/drawer", card.ID))
	if err != nil {
		t.Fatalf("GET drawer: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != 200 {
		t.Fatalf("expected 200, got %d", resp.StatusCode)
	}

	buf := new(strings.Builder)
	buf.ReadFrom(resp.Body)
	body := buf.String()

	checks := []string{
		"Drawer test card",
		"This is the card body",
		"Test comment",
		"bug",
		"user",
	}
	for _, check := range checks {
		if !strings.Contains(body, check) {
			t.Errorf("expected drawer to contain %q", check)
		}
	}

	// Check valid transitions are shown (from todo: in_flight, blocked, tabled)
	if !strings.Contains(body, "in_flight") {
		t.Error("expected drawer to show in_flight as valid transition")
	}
}
```

Add `"fmt"` to the test file imports if not present.

- [x] **Step 2: Run test to verify it fails**

Run: `cd /Users/joelhelbling/code/ai/kkullm && go test ./web/ -run TestDrawerHandler -v`
Expected: FAIL (404)

- [x] **Step 3: Create the drawer template**

Create `web/templates/drawer.html`:

```html
{{define "drawer"}}
<div class="drawer-header">
  <div>
    <div class="drawer-card-id">#{{.Card.ID}} · {{.Card.Project}}</div>
    <div class="drawer-title">{{.Card.Title}}</div>
  </div>
  <button class="drawer-close" @click="closeDrawer()">✕</button>
</div>

<div class="drawer-section">
  <div class="drawer-section-label">Status</div>
  <div class="drawer-status-pills">
    <span class="status-pill active active-{{.Card.Status}}">{{.Card.Status}} ✓</span>
    {{range .Transitions}}
    <span class="status-pill"
          hx-patch="/ui/cards/{{$.Card.ID}}/status"
          hx-vals='{"status":"{{.}}"}'
          hx-target="#drawer-container"
          hx-swap="innerHTML">{{.}}</span>
    {{end}}
  </div>
</div>

<div class="drawer-section">
  <div class="drawer-meta">
    <div>
      <div class="drawer-section-label">Assignees</div>
      <div class="drawer-meta-item">{{if .Card.Assignees}}{{joinStrings .Card.Assignees ", "}}{{else}}unassigned{{end}}</div>
    </div>
    <div>
      <div class="drawer-section-label">Tags</div>
      <div class="card-tile-tags">
        {{range .Card.Tags}}
        <span class="tag" style="background:{{tagBg .}};color:{{tagColor .}};">{{.}}</span>
        {{end}}
        {{if not .Card.Tags}}<span class="drawer-meta-item">none</span>{{end}}
      </div>
    </div>
    <div>
      <div class="drawer-section-label">Created</div>
      <div class="drawer-meta-item">{{timeAgo .Card.CreatedAt}}</div>
    </div>
  </div>
</div>

{{if .Card.Relations}}
<div class="drawer-section">
  <div class="drawer-section-label">Relations</div>
  {{range .Card.Relations}}
  <div class="drawer-relation">
    <span class="drawer-relation-type">{{.RelationType}}</span>
    <a class="drawer-relation-link"
       hx-get="/ui/cards/{{.RelatedCardID}}/drawer"
       hx-target="#drawer-container"
       hx-swap="innerHTML">#{{.RelatedCardID}}</a>
  </div>
  {{end}}
</div>
{{end}}

{{if .Card.Body}}
<div class="drawer-section">
  <div class="drawer-section-label">Description</div>
  <div class="drawer-body">{{.Card.Body}}</div>
</div>
{{end}}

<div class="drawer-section" style="border-bottom:none;">
  <div class="drawer-section-label">Comments ({{len .Comments}})</div>
  <div id="comments-list">
    {{range .Comments}}
    <div class="comment">
      <div class="comment-header">
        <span class="comment-agent{{if eq .Agent "user"}} user-agent{{end}}">{{.Agent}}</span>
        <span class="comment-time">{{timeAgo .CreatedAt}}</span>
      </div>
      <div class="comment-body">{{.Body}}</div>
    </div>
    {{end}}
    {{if not .Comments}}
    <div style="font-size:13px;color:var(--text-secondary);">No comments yet.</div>
    {{end}}
  </div>
</div>
{{end}}
```

- [x] **Step 4: Add the drawer handler**

Add to `web/handlers.go`:

```go
type drawerData struct {
	Card        *model.Card
	Comments    []model.Comment
	Transitions []string
}

func (ws *WebServer) handleDrawer(w http.ResponseWriter, r *http.Request) {
	id, err := strconv.Atoi(r.PathValue("id"))
	if err != nil {
		http.Error(w, "invalid id", 400)
		return
	}

	card, err := ws.store.GetCard(id)
	if err != nil {
		http.Error(w, err.Error(), 404)
		return
	}

	comments, err := ws.store.ListComments(id)
	if err != nil {
		http.Error(w, err.Error(), 500)
		return
	}
	if comments == nil {
		comments = []model.Comment{}
	}

	transitions := model.AllowedTransitions(card.Status)

	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	if err := tmpl.ExecuteTemplate(w, "drawer", drawerData{
		Card:        card,
		Comments:    comments,
		Transitions: transitions,
	}); err != nil {
		log.Printf("render drawer: %v", err)
	}
}
```

- [x] **Step 5: Register the route in web.go**

Add to `RegisterRoutes`:

```go
	// Card drawer
	mux.HandleFunc("GET /ui/cards/{id}/drawer", ws.handleDrawer)
```

- [x] **Step 6: Run test to verify it passes**

Run: `cd /Users/joelhelbling/code/ai/kkullm && go test ./web/ -run TestDrawerHandler -v`
Expected: PASS

- [x] **Step 7: Run all tests**

Run: `cd /Users/joelhelbling/code/ai/kkullm && go test ./...`
Expected: All pass

- [x] **Step 8: Commit**

```bash
git add web/templates/drawer.html web/handlers.go web/web.go web/handlers_test.go
git commit -m "feat: add card detail drawer with status selector and comments"
```

---

## Task 7: Status Change Handler (PATCH)

Implement the `/ui/cards/{id}/status` PATCH handler for both drag-and-drop and drawer status changes.

**Files:**
- Modify: `web/handlers.go`
- Modify: `web/web.go`
- Test: `web/handlers_test.go`

- [x] **Step 1: Write the tests**

Add to `web/handlers_test.go`:

```go
func TestStatusChange(t *testing.T) {
	mux, st := setupTestMuxWithStore(t)

	card, err := st.CreateCard(store.CardCreateParams{
		Title:     "Status test",
		Status:    "considering",
		ProjectID: 1,
	})
	if err != nil {
		t.Fatalf("create card: %v", err)
	}

	ts := httptest.NewServer(mux)
	defer ts.Close()

	// Valid transition: considering -> todo
	req, _ := http.NewRequest("PATCH", ts.URL+fmt.Sprintf("/ui/cards/%d/status", card.ID),
		strings.NewReader("status=todo"))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("PATCH status: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != 200 {
		t.Fatalf("expected 200, got %d", resp.StatusCode)
	}

	// Verify card was updated
	updated, _ := st.GetCard(card.ID)
	if updated.Status != "todo" {
		t.Errorf("expected status 'todo', got %q", updated.Status)
	}
}

func TestStatusChangeInvalid(t *testing.T) {
	mux, st := setupTestMuxWithStore(t)

	card, err := st.CreateCard(store.CardCreateParams{
		Title:     "Invalid transition test",
		Status:    "considering",
		ProjectID: 1,
	})
	if err != nil {
		t.Fatalf("create card: %v", err)
	}

	ts := httptest.NewServer(mux)
	defer ts.Close()

	// Invalid transition: considering -> in_flight (must go through todo first)
	req, _ := http.NewRequest("PATCH", ts.URL+fmt.Sprintf("/ui/cards/%d/status", card.ID),
		strings.NewReader("status=in_flight"))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("PATCH status: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != 422 {
		t.Fatalf("expected 422, got %d", resp.StatusCode)
	}
}
```

- [x] **Step 2: Run tests to verify they fail**

Run: `cd /Users/joelhelbling/code/ai/kkullm && go test ./web/ -run TestStatusChange -v`
Expected: FAIL (405 or 404)

- [x] **Step 3: Add the status change handler**

Add to `web/handlers.go`:

```go
func (ws *WebServer) handleStatusChange(w http.ResponseWriter, r *http.Request) {
	id, err := strconv.Atoi(r.PathValue("id"))
	if err != nil {
		http.Error(w, "invalid id", 400)
		return
	}

	if err := r.ParseForm(); err != nil {
		http.Error(w, "invalid form data", 400)
		return
	}

	newStatus := r.FormValue("status")
	if newStatus == "" {
		http.Error(w, "status is required", 400)
		return
	}

	card, err := ws.store.UpdateCard(id, store.CardUpdateParams{
		Status: &newStatus,
	})
	if err != nil {
		http.Error(w, err.Error(), 422)
		return
	}

	// Publish SSE event
	ws.events.Publish(api.Event{Type: "card_updated", Data: card})

	// Check HX-Trigger header to determine response type
	// If request came from the drawer, return drawer content
	// If from drag-and-drop (board), return the card tile
	hxTrigger := r.Header.Get("HX-Trigger")

	if strings.HasPrefix(hxTrigger, "drawer") || r.Header.Get("HX-Target") == "#drawer-container" {
		// Return drawer content
		comments, _ := ws.store.ListComments(id)
		if comments == nil {
			comments = []model.Comment{}
		}
		transitions := model.AllowedTransitions(card.Status)
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		tmpl.ExecuteTemplate(w, "drawer", drawerData{
			Card:        card,
			Comments:    comments,
			Transitions: transitions,
		})
	} else {
		// Return updated card tile for the board
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		tmpl.ExecuteTemplate(w, "card", cardView{Card: *card, ShowProject: false})
	}
}
```

- [x] **Step 4: Register the route in web.go**

Add to `RegisterRoutes`:

```go
	// Status change
	mux.HandleFunc("PATCH /ui/cards/{id}/status", ws.handleStatusChange)
```

- [x] **Step 5: Run tests to verify they pass**

Run: `cd /Users/joelhelbling/code/ai/kkullm && go test ./web/ -run TestStatusChange -v`
Expected: PASS

- [x] **Step 6: Run all tests**

Run: `cd /Users/joelhelbling/code/ai/kkullm && go test ./...`
Expected: All pass

- [x] **Step 7: Commit**

```bash
git add web/handlers.go web/web.go web/handlers_test.go
git commit -m "feat: add status change handler with SSE event publishing"
```

---

## Task 8: Blockers Handler

Implement the `/ui/blockers` endpoint that returns blocked column content.

**Files:**
- Create: `web/templates/blockers.html`
- Modify: `web/handlers.go`
- Modify: `web/web.go`
- Test: `web/handlers_test.go`

- [x] **Step 1: Write the test**

Add to `web/handlers_test.go`:

```go
func TestBlockersHandler(t *testing.T) {
	mux, st := setupTestMuxWithStore(t)

	// Create a card, move it to todo, then to blocked
	card, _ := st.CreateCard(store.CardCreateParams{
		Title:     "Blocked card",
		Status:    "todo",
		ProjectID: 1,
		Assignees: []string{"user"},
	})

	blockedStatus := "blocked"
	st.UpdateCard(card.ID, store.CardUpdateParams{Status: &blockedStatus})

	ts := httptest.NewServer(mux)
	defer ts.Close()

	resp, err := http.Get(ts.URL + "/ui/blockers")
	if err != nil {
		t.Fatalf("GET /ui/blockers: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != 200 {
		t.Fatalf("expected 200, got %d", resp.StatusCode)
	}

	buf := new(strings.Builder)
	buf.ReadFrom(resp.Body)
	body := buf.String()

	if !strings.Contains(body, "Blocked card") {
		t.Error("expected blockers to contain blocked card")
	}
}
```

- [x] **Step 2: Run test to verify it fails**

Run: `cd /Users/joelhelbling/code/ai/kkullm && go test ./web/ -run TestBlockersHandler -v`
Expected: FAIL (404)

- [x] **Step 3: Create the blockers template**

Create `web/templates/blockers.html`:

```html
{{define "blockers"}}
{{range .}}
<div class="card-tile"
     data-card-id="{{.ID}}"
     hx-get="/ui/cards/{{.ID}}/drawer"
     hx-target="#drawer-container"
     hx-swap="innerHTML">
  <div class="card-tile-project">
    <span class="project-badge" style="background:{{projectColor .Project}};color:white;">{{.Project}}</span>
  </div>
  <div class="card-tile-title">{{.Title}}</div>
  {{if .Relations}}
  <div style="font-size:10px;color:var(--text-secondary);margin-top:4px;">
    {{range .Relations}}
    {{if eq .RelationType "blocked_by"}}blocked_by #{{.RelatedCardID}}{{end}}
    {{end}}
  </div>
  {{end}}
</div>
{{end}}
{{if not .}}
<div style="font-size:12px;color:var(--text-secondary);padding:8px;">No blocked cards.</div>
{{end}}
{{end}}
```

- [x] **Step 4: Add the blockers handler**

Add to `web/handlers.go`:

```go
func (ws *WebServer) handleBlockers(w http.ResponseWriter, r *http.Request) {
	cards, err := ws.store.ListCards(store.CardListParams{
		Status: "blocked",
	})
	if err != nil {
		http.Error(w, err.Error(), 500)
		return
	}

	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	if err := tmpl.ExecuteTemplate(w, "blockers", cards); err != nil {
		log.Printf("render blockers: %v", err)
	}
}
```

- [x] **Step 5: Register the route in web.go**

Add to `RegisterRoutes`:

```go
	// Blockers
	mux.HandleFunc("GET /ui/blockers", ws.handleBlockers)
```

- [x] **Step 6: Run test to verify it passes**

Run: `cd /Users/joelhelbling/code/ai/kkullm && go test ./web/ -run TestBlockersHandler -v`
Expected: PASS

- [x] **Step 7: Run all tests**

Run: `cd /Users/joelhelbling/code/ai/kkullm && go test ./...`
Expected: All pass

- [x] **Step 8: Commit**

```bash
git add web/templates/blockers.html web/handlers.go web/web.go web/handlers_test.go
git commit -m "feat: add blockers column handler"
```

---

## Task 9: Client-Side JavaScript (Alpine, SortableJS, SSE, Dark Mode)

Write `app.js` with all client-side behavior: Alpine.js component, SortableJS initialization, SSE event handling with FLIP animations, dark mode toggle, and blocker column management.

**Files:**
- Modify: `web/static/js/app.js`

- [x] **Step 1: Write the complete app.js**

Replace `web/static/js/app.js`:

```js
// Kkullm Web UI — Alpine.js + SortableJS + SSE

function kkullm() {
  return {
    // State
    viewMode: 'project',
    currentProject: null,
    currentAgent: null,
    drawerOpen: false,
    drawerCardId: null,
    blockersOpen: false,
    blockerCount: 0,
    showClosed: false,
    theme: 'light',

    init() {
      this.initTheme();
      this.connectSSE();

      // After htmx swaps in the board, initialize SortableJS
      document.body.addEventListener('htmx:afterSwap', (e) => {
        if (e.detail.target.id === 'board-container') {
          this.$nextTick(() => this.initSortable());
          this.updateBlockerCount();
        }
        if (e.detail.target.id === 'drawer-container') {
          this.drawerOpen = true;
          // Extract card ID from drawer content
          const idEl = e.detail.target.querySelector('[data-card-id]');
          if (idEl) {
            this.drawerCardId = parseInt(idEl.dataset.cardId);
          }
        }
      });

      // Read initial project from the board container's hx-get
      const boardContainer = document.getElementById('board-container');
      if (boardContainer) {
        const hxGet = boardContainer.getAttribute('hx-get');
        if (hxGet) {
          const match = hxGet.match(/project=(\d+)/);
          if (match) this.currentProject = match[1];
        }
      }
    },

    // === Theme ===

    initTheme() {
      const saved = localStorage.getItem('kkullm-theme');
      if (saved) {
        this.theme = saved;
      } else if (window.matchMedia('(prefers-color-scheme: dark)').matches) {
        this.theme = 'dark';
      }
      document.documentElement.setAttribute('data-theme', this.theme);

      // Listen for OS changes
      window.matchMedia('(prefers-color-scheme: dark)').addEventListener('change', (e) => {
        if (!localStorage.getItem('kkullm-theme')) {
          this.theme = e.matches ? 'dark' : 'light';
          document.documentElement.setAttribute('data-theme', this.theme);
        }
      });
    },

    toggleTheme() {
      this.theme = this.theme === 'dark' ? 'light' : 'dark';
      document.documentElement.setAttribute('data-theme', this.theme);
      localStorage.setItem('kkullm-theme', this.theme);
    },

    // === Navigation ===

    loadBoard() {
      const container = document.getElementById('board-container');
      if (!container) return;

      let url;
      if (this.viewMode === 'agent' && this.currentAgent) {
        url = '/ui/board?agent=' + this.currentAgent;
      } else if (this.currentProject) {
        url = '/ui/board?project=' + this.currentProject;
      } else {
        return;
      }

      htmx.ajax('GET', url, { target: '#board-container', swap: 'innerHTML' });
      this.blockersOpen = false;
    },

    // === Drawer ===

    closeDrawer() {
      this.drawerOpen = false;
      this.drawerCardId = null;
    },

    // === Blockers ===

    toggleBlockers() {
      this.blockersOpen = !this.blockersOpen;
      if (this.blockersOpen) {
        htmx.trigger(document.body, 'blockers-refresh');
      }
    },

    updateBlockerCount() {
      const blockedCards = document.querySelectorAll('#blocked-cards .card-tile');
      const countEl = document.getElementById('blocked-count');
      if (countEl) {
        this.blockerCount = blockedCards.length;
        countEl.textContent = this.blockerCount;
      }
      // Also check for blocked column in board data attribute
      const board = document.getElementById('board');
      if (board && board.dataset.blockerCount !== undefined) {
        this.blockerCount = parseInt(board.dataset.blockerCount) || 0;
      }
    },

    // === SortableJS ===

    initSortable() {
      const columns = document.querySelectorAll('.column-cards[data-status]');
      columns.forEach((column) => {
        if (column._sortable) column._sortable.destroy();
        column._sortable = new Sortable(column, {
          group: 'cards',
          animation: 200,
          ghostClass: 'sortable-ghost',
          chosenClass: 'sortable-chosen',
          onEnd: (evt) => this.onCardDrop(evt),
        });
      });
    },

    onCardDrop(evt) {
      const cardEl = evt.item;
      const cardId = cardEl.dataset.cardId;
      const newStatus = evt.to.dataset.status;
      const oldStatus = evt.from.dataset.status;

      if (newStatus === oldStatus) return;

      // Attempt the status change
      fetch('/ui/cards/' + cardId + '/status', {
        method: 'PATCH',
        headers: { 'Content-Type': 'application/x-www-form-urlencoded' },
        body: 'status=' + encodeURIComponent(newStatus),
      }).then((resp) => {
        if (!resp.ok) {
          // Snap back to original column
          evt.from.appendChild(cardEl);
          resp.text().then((msg) => this.showToast(msg));
        } else {
          // Update card tile with response HTML
          resp.text().then((html) => {
            cardEl.outerHTML = html;
            this.updateColumnCounts();
          });
        }
      });
    },

    updateColumnCounts() {
      document.querySelectorAll('.column').forEach((col) => {
        const cards = col.querySelectorAll('.card-tile');
        const countEl = col.querySelector('.column-count');
        if (countEl) countEl.textContent = cards.length;
      });
    },

    // === SSE ===

    connectSSE() {
      const source = new EventSource('/api/events');

      source.addEventListener('card_created', (e) => {
        const event = JSON.parse(e.data);
        const card = event.data;
        this.handleCardCreated(card);
      });

      source.addEventListener('card_updated', (e) => {
        const event = JSON.parse(e.data);
        const card = event.data;
        this.handleCardUpdated(card);
      });

      source.addEventListener('card_deleted', (e) => {
        const event = JSON.parse(e.data);
        this.handleCardDeleted(event.data.id);
      });

      source.addEventListener('comment_created', (e) => {
        const event = JSON.parse(e.data);
        const comment = event.data;
        this.handleCommentCreated(comment);
      });

      source.onerror = () => {
        // EventSource auto-reconnects; no action needed
      };
    },

    handleCardCreated(card) {
      // Refresh the board to pick up the new card
      this.loadBoard();
    },

    handleCardUpdated(card) {
      const cardEl = document.querySelector('[data-card-id="' + card.id + '"]');
      if (!cardEl) {
        // Card not visible, refresh board
        this.loadBoard();
        return;
      }

      const oldColumn = cardEl.closest('.column-cards');
      const oldStatus = oldColumn ? oldColumn.dataset.status : null;

      if (oldStatus && oldStatus !== card.status) {
        // Status changed — FLIP animation
        this.flipCard(cardEl, card);
      } else {
        // Other update — highlight glow
        cardEl.classList.add('highlight');
        setTimeout(() => cardEl.classList.remove('highlight'), 1500);
      }

      // Handle blocked status changes
      if (card.status === 'blocked') {
        this.blockerCount++;
        if (!this.blockersOpen) {
          this.blockersOpen = true;
        }
        htmx.trigger(document.body, 'blockers-refresh');
      } else if (oldStatus === 'blocked') {
        this.blockerCount = Math.max(0, this.blockerCount - 1);
        if (this.blockerCount === 0) {
          this.blockersOpen = false;
        }
        htmx.trigger(document.body, 'blockers-refresh');
      }

      // Refresh drawer if it's showing this card
      if (this.drawerOpen && this.drawerCardId === card.id) {
        htmx.ajax('GET', '/ui/cards/' + card.id + '/drawer', {
          target: '#drawer-container',
          swap: 'innerHTML',
        });
      }
    },

    flipCard(cardEl, card) {
      // FIRST: record current position
      const first = cardEl.getBoundingClientRect();

      // Move to new column
      const newColumn = document.querySelector('.column-cards[data-status="' + card.status + '"]');
      if (!newColumn) {
        this.loadBoard();
        return;
      }

      // LAST: move element and get new position
      newColumn.prepend(cardEl);
      const last = cardEl.getBoundingClientRect();

      // INVERT: apply transform to make it appear at old position
      const dx = first.left - last.left;
      const dy = first.top - last.top;
      cardEl.style.transform = 'translate(' + dx + 'px, ' + dy + 'px)';
      cardEl.style.transition = 'none';

      // PLAY: animate to new position
      requestAnimationFrame(() => {
        cardEl.style.transition = 'transform 0.4s ease';
        cardEl.style.transform = '';
        cardEl.addEventListener('transitionend', () => {
          cardEl.style.transition = '';
          cardEl.classList.add('highlight');
          setTimeout(() => cardEl.classList.remove('highlight'), 1500);
          this.updateColumnCounts();
        }, { once: true });
      });
    },

    handleCardDeleted(cardId) {
      const cardEl = document.querySelector('[data-card-id="' + cardId + '"]');
      if (cardEl) {
        cardEl.classList.add('fade-out');
        setTimeout(() => {
          cardEl.remove();
          this.updateColumnCounts();
        }, 300);
      }

      if (this.drawerOpen && this.drawerCardId === cardId) {
        this.closeDrawer();
      }
    },

    handleCommentCreated(comment) {
      if (this.drawerOpen && this.drawerCardId === comment.card_id) {
        htmx.ajax('GET', '/ui/cards/' + comment.card_id + '/drawer', {
          target: '#drawer-container',
          swap: 'innerHTML',
        });
      }

      // Update comment count on card tile
      const cardEl = document.querySelector('[data-card-id="' + comment.card_id + '"]');
      if (cardEl) {
        cardEl.classList.add('highlight');
        setTimeout(() => cardEl.classList.remove('highlight'), 1500);
      }
    },

    // === Toast ===

    showToast(message) {
      const container = document.getElementById('toast-container');
      const toast = document.createElement('div');
      toast.className = 'toast';
      toast.textContent = message;
      container.appendChild(toast);
      setTimeout(() => toast.remove(), 4000);
    },
  };
}
```

- [x] **Step 2: Verify JS loads correctly**

Run: `cd /Users/joelhelbling/code/ai/kkullm && go test ./web/ -run TestStaticVendorJS -v`
Expected: PASS (confirms static serving still works)

- [x] **Step 3: Commit**

```bash
git add web/static/js/app.js
git commit -m "feat: add client-side JS with Alpine, SortableJS, SSE, and dark mode"
```

---

## Task 10: Integration Smoke Test

Add an integration test that exercises the full flow: load page, fetch board, open drawer, change status.

**Files:**
- Modify: `web/handlers_test.go`

- [x] **Step 1: Write the integration test**

Add to `web/handlers_test.go`:

```go
func TestFullFlow(t *testing.T) {
	mux, st := setupTestMuxWithStore(t)

	// Create test data
	card, err := st.CreateCard(store.CardCreateParams{
		Title:     "Flow test card",
		Body:      "Testing the full flow",
		Status:    "considering",
		ProjectID: 1,
		Assignees: []string{"user"},
		Tags:      []string{"test"},
	})
	if err != nil {
		t.Fatalf("create card: %v", err)
	}

	ts := httptest.NewServer(mux)
	defer ts.Close()

	// 1. Load root page
	resp, err := http.Get(ts.URL + "/")
	if err != nil {
		t.Fatalf("GET /: %v", err)
	}
	resp.Body.Close()
	if resp.StatusCode != 200 {
		t.Fatalf("root: expected 200, got %d", resp.StatusCode)
	}

	// 2. Fetch board
	resp, err = http.Get(ts.URL + "/ui/board?project=1")
	if err != nil {
		t.Fatalf("GET /ui/board: %v", err)
	}
	buf := new(strings.Builder)
	buf.ReadFrom(resp.Body)
	resp.Body.Close()
	if !strings.Contains(buf.String(), "Flow test card") {
		t.Error("board should contain the test card")
	}

	// 3. Open drawer
	resp, err = http.Get(ts.URL + fmt.Sprintf("/ui/cards/%d/drawer", card.ID))
	if err != nil {
		t.Fatalf("GET drawer: %v", err)
	}
	buf.Reset()
	buf.ReadFrom(resp.Body)
	resp.Body.Close()
	if !strings.Contains(buf.String(), "Testing the full flow") {
		t.Error("drawer should contain card body")
	}

	// 4. Change status: considering -> todo
	req, _ := http.NewRequest("PATCH",
		ts.URL+fmt.Sprintf("/ui/cards/%d/status", card.ID),
		strings.NewReader("status=todo"))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	resp, err = http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("PATCH status: %v", err)
	}
	resp.Body.Close()
	if resp.StatusCode != 200 {
		t.Fatalf("status change: expected 200, got %d", resp.StatusCode)
	}

	// 5. Verify card is now in todo
	updated, _ := st.GetCard(card.ID)
	if updated.Status != "todo" {
		t.Errorf("expected status 'todo', got %q", updated.Status)
	}

	// 6. Fetch blockers (should be empty)
	resp, err = http.Get(ts.URL + "/ui/blockers")
	if err != nil {
		t.Fatalf("GET blockers: %v", err)
	}
	buf.Reset()
	buf.ReadFrom(resp.Body)
	resp.Body.Close()
	if strings.Contains(buf.String(), "Flow test card") {
		t.Error("blockers should not contain the test card (it's in todo, not blocked)")
	}

	// 7. Move to blocked
	req, _ = http.NewRequest("PATCH",
		ts.URL+fmt.Sprintf("/ui/cards/%d/status", card.ID),
		strings.NewReader("status=blocked"))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	resp, _ = http.DefaultClient.Do(req)
	resp.Body.Close()

	// 8. Fetch blockers (should contain the card now)
	resp, err = http.Get(ts.URL + "/ui/blockers")
	if err != nil {
		t.Fatalf("GET blockers: %v", err)
	}
	buf.Reset()
	buf.ReadFrom(resp.Body)
	resp.Body.Close()
	if !strings.Contains(buf.String(), "Flow test card") {
		t.Error("blockers should contain the blocked card")
	}
}
```

- [x] **Step 2: Run the integration test**

Run: `cd /Users/joelhelbling/code/ai/kkullm && go test ./web/ -run TestFullFlow -v`
Expected: PASS

- [x] **Step 3: Run all tests**

Run: `cd /Users/joelhelbling/code/ai/kkullm && go test ./...`
Expected: All pass

- [x] **Step 4: Commit**

```bash
git add web/handlers_test.go
git commit -m "test: add integration smoke test for full web UI flow"
```

---

## Task 11: Manual Verification

Start the server and verify the web UI works in a browser.

- [x] **Step 1: Build and start the server**

```bash
cd /Users/joelhelbling/code/ai/kkullm
go build -o kkullm . && ./kkullm serve
```

- [x] **Step 2: Seed test data via CLI**

In a separate terminal:

```bash
cd /Users/joelhelbling/code/ai/kkullm
./kkullm project create --name "api-backend" --description "Backend API"
./kkullm agent create --name "worker-1" --project "api-backend"
./kkullm card create --title "Fix CORS headers" --project "api-backend" --tag "bug" --assignee "worker-1" --as "user"
./kkullm card create --title "Add pagination" --project "api-backend" --tag "enhancement" --as "user"
./kkullm card create --title "Write API docs" --project "api-backend" --tag "docs" --as "user"
```

- [x] **Step 3: Open browser and verify**

Open `http://localhost:8080` and check:

1. Page loads with nav bar, project dropdown, view switcher
2. Board shows cards in the correct columns
3. Clicking a card opens the drawer from the right
4. Drawer shows status selector with valid transitions
5. Dark mode toggle works (moon/sun icon)
6. Switching projects updates the board
7. Switching to agent view shows project-of-origin badges

- [x] **Step 4: Test drag-and-drop**

1. Drag a card from "Considering" to "Todo"
2. Verify it stays in the new column
3. Try an invalid drag (e.g., Considering → In Flight)
4. Verify error toast appears and card snaps back

- [x] **Step 5: Test blockers**

```bash
./kkullm card update 1 --status todo --as "user"
./kkullm card update 1 --status blocked --as "user"
```

Verify the "Blocked!" badge appears and clicking it slides the blocked column in.

- [x] **Step 6: Fix any issues found during manual testing**

Address any layout, styling, or behavior issues discovered. Commit fixes.
