# Installer Deployment Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Add three installation channels for kkullm — a shared Homebrew tap, GitHub-release binary archives, and a `curl | sh` script — driven by goreleaser and a tag-triggered GitHub Actions release workflow.

**Architecture:** kkullm is a single self-contained Go binary (embedded web assets + migrations, CGO-free SQLite), so it cross-compiles cleanly with `CGO_ENABLED=0`. Two small in-repo prerequisites (version ldflags wiring + a canonical XDG data directory) precede the packaging work. goreleaser builds the matrix, attaches archives + checksums to a GitHub release, and writes `Casks/kkullm.rb` into the shared `joelhelbling/homebrew-tap` repo.

**Tech Stack:** Go 1.25.5, Cobra, modernc.org/sqlite, goreleaser v2, GitHub Actions, Task (Taskfile), POSIX sh.

**Spec:** `docs/superpowers/specs/2026-06-13-installer-deployment-design.md`

---

## File Structure

| File | Responsibility |
|------|----------------|
| `cmd/version.go` | Holds `Version` var; wires Cobra's `--version` flag + a `version` subcommand |
| `cmd/version_test.go` | Tests version output |
| `cmd/datadir.go` | Pure resolver for the default XDG DB path |
| `cmd/datadir_test.go` | Tests path resolution under XDG/HOME |
| `cmd/serve.go` | `--db` default uses resolver + `KKULLM_DB`; creates data dir before opening |
| `db/db.go` | Adds `busy_timeout` PRAGMA in `Open` |
| `.goreleaser.yaml` | Build matrix, archives, checksums, homebrew_casks → shared tap, release |
| `.github/workflows/release.yaml` | Tag-triggered goreleaser run |
| `install.sh` | curl-pipe installer (detect OS/arch, verify checksum, install) |
| `Taskfile.yaml` | Version ldflags in `build`; `release-snapshot` task |
| `README.md` | Installation section + data-dir notes |
| `plugins/kkullm/skills/cli/SKILL.md` | Review/update install & version references |

---

## Task 1: Version wiring

Cobra gives `--version` for free when `rootCmd.Version` is set, and we add an explicit `version` subcommand for discoverability. goreleaser overrides `Version` via ldflags at build time; local builds stay `"dev"`.

**Files:**
- Create: `cmd/version.go`
- Create: `cmd/version_test.go`
- Modify: `cmd/root.go` (wire `rootCmd.Version` in `init`)

- [ ] **Step 1: Write the failing test**

Create `cmd/version_test.go`:

```go
package cmd

import (
	"bytes"
	"testing"
)

func TestVersionCommandPrintsVersion(t *testing.T) {
	Version = "1.2.3"
	defer func() { Version = "dev" }()

	var out bytes.Buffer
	rootCmd.SetOut(&out)
	rootCmd.SetErr(&out)
	rootCmd.SetArgs([]string{"version"})
	if err := rootCmd.Execute(); err != nil {
		t.Fatalf("execute version: %v", err)
	}

	if got := out.String(); got == "" || !bytes.Contains(out.Bytes(), []byte("1.2.3")) {
		t.Errorf("version output = %q; want it to contain %q", got, "1.2.3")
	}
}
```

- [ ] **Step 2: Run test to verify it fails**

Run: `go test ./cmd/ -run TestVersionCommandPrintsVersion -v`
Expected: FAIL — `version` is an unknown command (or `Version` undefined).

- [ ] **Step 3: Write minimal implementation**

Create `cmd/version.go`:

```go
package cmd

import (
	"fmt"

	"github.com/spf13/cobra"
)

// Version is the build version. Overridden at release time via ldflags:
//   -X github.com/joelhelbling/kkullm/cmd.Version={{.Version}}
var Version = "dev"

var versionCmd = &cobra.Command{
	Use:   "version",
	Short: "Print the kkullm version",
	RunE: func(cmd *cobra.Command, args []string) error {
		fmt.Fprintln(cmd.OutOrStdout(), Version)
		return nil
	},
}

func init() {
	rootCmd.AddCommand(versionCmd)
}
```

In `cmd/root.go`, inside the existing `init()` (after the flag registrations), add the line that enables the built-in `--version` flag:

```go
	rootCmd.Version = Version
```

- [ ] **Step 4: Run test to verify it passes**

Run: `go test ./cmd/ -run TestVersionCommandPrintsVersion -v`
Expected: PASS

- [ ] **Step 5: Manual smoke check**

Run: `go run . version` (expect `dev`) and `go run . --version` (expect `kkullm version dev`).

- [ ] **Step 6: Commit**

```bash
git add cmd/version.go cmd/version_test.go cmd/root.go
git commit -m "feat: add version command and --version flag"
```

---

## Task 2: Canonical XDG data-dir resolver

A pure function (no side effects) so it's easily testable; directory creation happens later in `serve` only.

**Files:**
- Create: `cmd/datadir.go`
- Create: `cmd/datadir_test.go`

- [ ] **Step 1: Write the failing test**

Create `cmd/datadir_test.go`:

```go
package cmd

import (
	"path/filepath"
	"testing"
)

func TestDefaultDBPathUsesXDG(t *testing.T) {
	t.Setenv("XDG_DATA_HOME", "/tmp/xdghome")
	got := defaultDBPath()
	want := filepath.Join("/tmp/xdghome", "kkullm", "kkullm.db")
	if got != want {
		t.Errorf("defaultDBPath() = %q; want %q", got, want)
	}
}

func TestDefaultDBPathFallsBackToHome(t *testing.T) {
	t.Setenv("XDG_DATA_HOME", "")
	t.Setenv("HOME", "/tmp/fakehome")
	got := defaultDBPath()
	want := filepath.Join("/tmp/fakehome", ".local", "share", "kkullm", "kkullm.db")
	if got != want {
		t.Errorf("defaultDBPath() = %q; want %q", got, want)
	}
}
```

- [ ] **Step 2: Run test to verify it fails**

Run: `go test ./cmd/ -run TestDefaultDBPath -v`
Expected: FAIL — `defaultDBPath` undefined.

- [ ] **Step 3: Write minimal implementation**

Create `cmd/datadir.go`:

```go
package cmd

import (
	"os"
	"path/filepath"
)

// defaultDBPath returns the canonical location for the server's SQLite database:
// $XDG_DATA_HOME/kkullm/kkullm.db, falling back to ~/.local/share/kkullm/kkullm.db.
// It is pure — directory creation happens in the serve command, not here.
func defaultDBPath() string {
	base := os.Getenv("XDG_DATA_HOME")
	if base == "" {
		home, err := os.UserHomeDir()
		if err != nil {
			// Last-resort fallback: relative path, preserving old behavior.
			return "kkullm.db"
		}
		base = filepath.Join(home, ".local", "share")
	}
	return filepath.Join(base, "kkullm", "kkullm.db")
}
```

- [ ] **Step 4: Run test to verify it passes**

Run: `go test ./cmd/ -run TestDefaultDBPath -v`
Expected: PASS

- [ ] **Step 5: Commit**

```bash
git add cmd/datadir.go cmd/datadir_test.go
git commit -m "feat: add XDG default database path resolver"
```

---

## Task 3: Wire serve to the resolver + KKULLM_DB + dir creation

Precedence: explicit `--db` flag > `KKULLM_DB` env > XDG default. The flag default is computed from env-or-default at init; the data directory is created in `RunE` before opening (so non-serve commands never create it).

**Files:**
- Modify: `cmd/serve.go`

- [ ] **Step 1: Update the flag default**

In `cmd/serve.go`, change the `init()` `--db` flag registration from:

```go
	serveCmd.Flags().StringVar(&dbPath, "db", "kkullm.db", "Database file path")
```

to:

```go
	serveCmd.Flags().StringVar(&dbPath, "db", envOrDefault("KKULLM_DB", defaultDBPath()),
		"Database file path (defaults to $XDG_DATA_HOME/kkullm/kkullm.db; override with KKULLM_DB)")
```

(`envOrDefault` already exists in `cmd/root.go`.)

- [ ] **Step 2: Create the data directory before opening**

In `cmd/serve.go`, add `"os"` and `"path/filepath"` to the imports, then at the top of the `RunE` func (before `db.Open`) insert:

```go
		if dir := filepath.Dir(dbPath); dir != "" && dir != "." {
			if err := os.MkdirAll(dir, 0o700); err != nil {
				return fmt.Errorf("create data dir %s: %w", dir, err)
			}
		}
```

- [ ] **Step 3: Build to verify it compiles**

Run: `go build ./...`
Expected: no errors.

- [ ] **Step 4: Manual smoke check**

Run: `XDG_DATA_HOME=/tmp/kkullm-test go run . serve --addr :0` briefly (Ctrl-C after the "listening" log), then confirm `/tmp/kkullm-test/kkullm/kkullm.db` was created. Clean up `/tmp/kkullm-test`.

- [ ] **Step 5: Commit**

```bash
git add cmd/serve.go
git commit -m "feat: serve defaults DB to XDG data dir, honors KKULLM_DB"
```

---

## Task 4: busy_timeout PRAGMA

So concurrent access waits briefly instead of erroring with `SQLITE_BUSY`.

**Files:**
- Modify: `db/db.go`

- [ ] **Step 1: Add the PRAGMA**

In `db/db.go`, add `"PRAGMA busy_timeout=5000"` to the pragma slice in `Open`:

```go
	for _, pragma := range []string{
		"PRAGMA journal_mode=WAL",
		"PRAGMA foreign_keys=ON",
		"PRAGMA busy_timeout=5000",
	} {
```

- [ ] **Step 2: Run the db tests**

Run: `go test ./db/...`
Expected: PASS.

- [ ] **Step 3: Commit**

```bash
git add db/db.go
git commit -m "feat: set sqlite busy_timeout to tolerate concurrent access"
```

---

## Task 5: goreleaser config + snapshot validation

**Files:**
- Create: `.goreleaser.yaml`
- Modify: `Taskfile.yaml`

- [ ] **Step 1: Create `.goreleaser.yaml`**

```yaml
# yaml-language-server: $schema=https://goreleaser.com/static/schema.json
version: 2

before:
  hooks:
    - go mod tidy

builds:
  - id: kkullm
    main: .
    binary: kkullm
    env:
      - CGO_ENABLED=0
    goos:
      - linux
      - darwin
    goarch:
      - amd64
      - arm64
    ldflags:
      - -s -w
      - -X github.com/joelhelbling/kkullm/cmd.Version={{.Version}}

archives:
  - id: kkullm
    formats:
      - tar.gz
    name_template: >-
      {{ .ProjectName }}_
      {{- .Version }}_
      {{- .Os }}_
      {{- .Arch }}
    files:
      - README.md
      - LICENSE*

checksum:
  name_template: 'checksums.txt'

snapshot:
  version_template: "{{ incpatch .Version }}-next"

changelog:
  sort: asc
  filters:
    exclude:
      - '^docs:'
      - '^test:'
      - '^ci:'
      - Merge pull request
      - Merge branch

homebrew_casks:
  - name: kkullm
    ids:
      - kkullm
    binaries:
      - kkullm
    repository:
      owner: joelhelbling
      name: homebrew-tap
      branch: main
      token: "{{ .Env.HOMEBREW_TAP_GITHUB_TOKEN }}"
    homepage: "https://github.com/joelhelbling/kkullm"
    description: "Agent orchestration system based on the blackboard pattern"
    commit_author:
      name: goreleaserbot
      email: bot@goreleaser.com
    commit_msg_template: "Brew cask update for {{ .ProjectName }} version {{ .Tag }}"
    hooks:
      post:
        install: |
          if OS.mac?
            system_command "/usr/bin/xattr", args: ["-dr", "com.apple.quarantine", "#{staged_path}/kkullm"]
          end

release:
  github:
    owner: joelhelbling
    name: kkullm
  draft: false
  prerelease: auto
  name_template: "{{.Tag}}"
```

- [ ] **Step 2: Add a release-snapshot task to `Taskfile.yaml`**

Add to the `tasks:` map:

```yaml
  release-snapshot:
    desc: Build all release artifacts locally without publishing (validates goreleaser)
    cmds:
      - goreleaser release --snapshot --clean
```

And add version ldflags to the existing `build` task `cmds` (replace the single `go build` line):

```yaml
      - go build -ldflags "-X github.com/joelhelbling/kkullm/cmd.Version=dev" -o {{.BINARY}} .
```

- [ ] **Step 3: Validate with a snapshot build**

Run: `goreleaser release --snapshot --clean`
(If goreleaser is not installed: `brew install goreleaser` first.)
Expected: completes successfully; `dist/` contains four `kkullm_*_*.tar.gz` archives (linux/darwin × amd64/arm64), `checksums.txt`, and a generated cask (`dist/homebrew/Casks/kkullm.rb`). Confirm `goreleaser check` reports no deprecation warnings and no errors about the homebrew_casks block (token is only needed for actual publish, not snapshot).

- [ ] **Step 4: Verify the embedded version in a snapshot binary**

Run: `./dist/kkullm_*_darwin_arm64/kkullm version` (adjust path to your platform).
Expected: prints a snapshot version like `0.0.1-next` (not `dev`), proving ldflags injection works.

- [ ] **Step 5: Ensure dist/ is git-ignored**

Confirm `.gitignore` contains `dist/`; if not, add it. Run: `grep -q '^dist/' .gitignore || echo 'dist/' >> .gitignore`

- [ ] **Step 6: Commit**

```bash
git add .goreleaser.yaml Taskfile.yaml .gitignore
git commit -m "build: add goreleaser config and release-snapshot task"
```

---

## Task 6: Release workflow

**Files:**
- Create: `.github/workflows/release.yaml`

- [ ] **Step 1: Create the workflow**

```yaml
name: Release

on:
  push:
    tags:
      - 'v*'

permissions:
  contents: write

jobs:
  goreleaser:
    runs-on: ubuntu-latest
    steps:
      - name: Checkout
        uses: actions/checkout@v4
        with:
          fetch-depth: 0

      - name: Set up Go
        uses: actions/setup-go@v5
        with:
          go-version: '1.25.5'

      - name: Run GoReleaser
        uses: goreleaser/goreleaser-action@v6
        with:
          distribution: goreleaser
          version: '~> v2'
          args: release --clean
        env:
          GITHUB_TOKEN: ${{ secrets.GITHUB_TOKEN }}
          HOMEBREW_TAP_GITHUB_TOKEN: ${{ secrets.HOMEBREW_TAP_GITHUB_TOKEN }}
```

- [ ] **Step 2: Validate YAML**

Run: `goreleaser check` (validates `.goreleaser.yaml`) and visually confirm the workflow indentation.
Expected: `goreleaser check` reports the config is valid.

- [ ] **Step 3: Commit**

```bash
git add .github/workflows/release.yaml
git commit -m "ci: add tag-triggered goreleaser release workflow"
```

---

## Task 7: Install script

A POSIX `sh` script for `curl … | sh`. Detects OS/arch, resolves the release (latest or pinned via `KKULLM_VERSION`), downloads + checksum-verifies the archive, and installs to `~/.local/bin` (override via `INSTALL_DIR`).

**Files:**
- Create: `install.sh`

- [ ] **Step 1: Create `install.sh`**

```sh
#!/bin/sh
# Install kkullm from GitHub releases.
# Usage: curl -fsSL https://raw.githubusercontent.com/joelhelbling/kkullm/main/install.sh | sh
# Env:
#   KKULLM_VERSION  pin a version (e.g. v0.1.0); default: latest
#   INSTALL_DIR     install location; default: ~/.local/bin
set -eu

REPO="joelhelbling/kkullm"
INSTALL_DIR="${INSTALL_DIR:-$HOME/.local/bin}"

# --- detect OS ---
os="$(uname -s)"
case "$os" in
  Linux)  os="linux" ;;
  Darwin) os="darwin" ;;
  *) echo "kkullm: unsupported OS: $os" >&2; exit 1 ;;
esac

# --- detect arch ---
arch="$(uname -m)"
case "$arch" in
  x86_64|amd64) arch="amd64" ;;
  arm64|aarch64) arch="arm64" ;;
  *) echo "kkullm: unsupported architecture: $arch" >&2; exit 1 ;;
esac

# --- resolve version ---
version="${KKULLM_VERSION:-}"
if [ -z "$version" ]; then
  version="$(curl -fsSL "https://api.github.com/repos/$REPO/releases/latest" \
    | grep '"tag_name":' | head -n1 | sed -E 's/.*"tag_name": *"([^"]+)".*/\1/')"
fi
if [ -z "$version" ]; then
  echo "kkullm: could not resolve release version" >&2
  exit 1
fi

# goreleaser strips the leading 'v' from the version in archive names.
ver_no_v="${version#v}"
archive="kkullm_${ver_no_v}_${os}_${arch}.tar.gz"
base="https://github.com/$REPO/releases/download/$version"

tmp="$(mktemp -d)"
trap 'rm -rf "$tmp"' EXIT

echo "kkullm: downloading $archive ($version)"
curl -fsSL "$base/$archive" -o "$tmp/$archive"
curl -fsSL "$base/checksums.txt" -o "$tmp/checksums.txt"

# --- verify checksum ---
echo "kkullm: verifying checksum"
expected="$(grep " $archive\$" "$tmp/checksums.txt" | awk '{print $1}')"
if [ -z "$expected" ]; then
  echo "kkullm: no checksum found for $archive" >&2
  exit 1
fi
if command -v sha256sum >/dev/null 2>&1; then
  actual="$(sha256sum "$tmp/$archive" | awk '{print $1}')"
else
  actual="$(shasum -a 256 "$tmp/$archive" | awk '{print $1}')"
fi
if [ "$expected" != "$actual" ]; then
  echo "kkullm: checksum mismatch (expected $expected, got $actual)" >&2
  exit 1
fi

# --- extract and install ---
tar -xzf "$tmp/$archive" -C "$tmp"
mkdir -p "$INSTALL_DIR"
install -m 0755 "$tmp/kkullm" "$INSTALL_DIR/kkullm"

echo "kkullm: installed $version to $INSTALL_DIR/kkullm"
case ":$PATH:" in
  *":$INSTALL_DIR:"*) ;;
  *) echo "kkullm: note — $INSTALL_DIR is not on your PATH" >&2 ;;
esac
```

- [ ] **Step 2: Make it executable and lint syntax**

```bash
chmod +x install.sh
sh -n install.sh
```
Expected: `sh -n` produces no output (syntax OK).

- [ ] **Step 3: Commit**

```bash
git add install.sh
git commit -m "feat: add curl-pipe install script"
```

*(End-to-end execution of `install.sh` is only possible after the first real release exists; see Task 9.)*

---

## Task 8: Documentation

**Files:**
- Modify: `README.md`
- Modify: `plugins/kkullm/skills/cli/SKILL.md` (only if it references install/version)

- [ ] **Step 1: Add an Installation section to README.md**

Insert near the top of `README.md` (after the intro, before usage). Place it where it reads naturally relative to the existing structure:

```markdown
## Installation

### Homebrew (macOS / Linux)

```sh
brew install joelhelbling/tap/kkullm
```

### Install script

```sh
curl -fsSL https://raw.githubusercontent.com/joelhelbling/kkullm/main/install.sh | sh
```

Pin a version with `KKULLM_VERSION=v0.1.0` or change the location with
`INSTALL_DIR=/usr/local/bin`.

### Manual

Download an archive for your platform from the
[releases page](https://github.com/joelhelbling/kkullm/releases) and place the
`kkullm` binary on your `PATH`.

### From source

```sh
go install github.com/joelhelbling/kkullm@latest
```

### Data directory

`kkullm serve` stores its SQLite database at
`$XDG_DATA_HOME/kkullm/kkullm.db` (default `~/.local/share/kkullm/kkullm.db`).
Override with the `KKULLM_DB` environment variable or the `--db` flag.

Run **one server per database**: a second `kkullm serve` on the same machine
will fail to bind the port, and pointing two servers at the same database file
means their live (SSE) updates won't reach each other's browsers even though the
data stays consistent. Do not place the database on a network filesystem (NFS),
where SQLite locking is unreliable.
```

- [ ] **Step 2: Review the CLI skill for stale references**

Run: `grep -n -i "install\|version\|go build\|kkullm.db" plugins/kkullm/skills/cli/SKILL.md`
If it documents installation or the DB location in a way this change makes stale (e.g. claims the DB lives in the cwd, or that there's no `version` command), update those lines to match the new behavior. If there are no such references, make no change.

- [ ] **Step 3: Commit**

```bash
git add README.md plugins/kkullm/skills/cli/SKILL.md
git commit -m "docs: add installation section and data-dir guidance"
```

---

## Task 9: First release (maintainer, manual — one-time prerequisites)

Not a code task; performed once to bring the pipeline online. Listed so it isn't forgotten.

- [ ] **Step 1: Create the shared tap repo**

Create an empty `github.com/joelhelbling/homebrew-tap` repository (public, no README needed).

- [ ] **Step 2: Create and register the tap token**

Create a GitHub Personal Access Token with write (`contents`) access to `homebrew-tap`. In the kkullm repo: Settings → Secrets and variables → Actions → add `HOMEBREW_TAP_GITHUB_TOKEN`.

- [ ] **Step 3: Tag and push a release**

```bash
git tag v0.1.0
git push origin v0.1.0
```

- [ ] **Step 4: Verify the release**

Confirm the GitHub release for `v0.1.0` has four `tar.gz` archives + `checksums.txt`, and that `Casks/kkullm.rb` was committed to `homebrew-tap`.

- [ ] **Step 5: Verify each channel on a clean machine**

```sh
brew install joelhelbling/tap/kkullm && kkullm version
# and, separately:
curl -fsSL https://raw.githubusercontent.com/joelhelbling/kkullm/main/install.sh | sh && kkullm version
```
Expected: both print `0.1.0`.

---

## Self-Review Notes

- **Spec coverage:** version wiring (T1), XDG data dir + KKULLM_DB + dir creation (T2, T3), busy_timeout (T4), goreleaser/archives/checksums/homebrew_casks-to-shared-tap/snapshot task (T5), release workflow w/ both tokens + Go 1.25.5 (T6), install.sh with checksum verify + ~/.local/bin + env overrides (T7), README install + data-dir caveat + SKILL.md review (T8), one-time tap/token setup + first release verification (T9). All spec sections mapped.
- **Type consistency:** `Version` (cmd) and `defaultDBPath()` (cmd) are referenced consistently across tasks and the ldflags path `github.com/joelhelbling/kkullm/cmd.Version` matches in T1, T5, T6.
- **Non-goals respected:** no Docker, Windows, brew service, or single-instance guard.
