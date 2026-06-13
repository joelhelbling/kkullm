# Installer Deployment Design

**Date:** 2026-06-13
**Status:** Approved (pending implementation plan)

## Goal

Give kkullm a real installation story. Today the only way to get the binary is
to clone the repo and `task build`. This design adds three distribution channels
modeled on the proven setup in the sibling `glovebox` project: a Homebrew tap, GitHub-release
binary archives, and a `curl | sh` install script — all driven by
[goreleaser](https://goreleaser.com) and a tag-triggered GitHub Actions workflow.

kkullm is well-suited to this: it's a single self-contained Go binary (web assets
and DB migrations are embedded via `go:embed`) using the CGO-free
`modernc.org/sqlite` driver, so it cross-compiles cleanly to linux/darwin ×
amd64/arm64 with `CGO_ENABLED=0`.

## Non-Goals

- **Docker image** — deferred to its own fast-follow spec, scoped to the *server*
  role only (`docker run … kkullm serve`). The CLI is an HTTP client that defaults
  to `localhost`; running it in a container would force `-e KKULLM_SERVER=…` on
  every call and defeat its ergonomics. Not worth it for the CLI.
- **Windows builds** — not targeted initially (matches glovebox).
- **Homebrew service** (`brew services start kkullm`) — possible future enhancement
  for the server role; out of scope now.
- **Single-instance server guard** — see the caveat under Data Directory; handled
  by documentation for now.

## Channels

1. **Homebrew tap** — `brew install joelhelbling/tap/kkullm`
2. **Binary archives** — tar.gz for linux/darwin × amd64/arm64, attached to each
   GitHub release, with a `checksums.txt`.
3. **Install script** — `install.sh` served from the repo, run via
   `curl -fsSL https://raw.githubusercontent.com/joelhelbling/kkullm/main/install.sh | sh`.

## Shared Homebrew Tap

Rather than one `homebrew-<project>` repo per tool, kkullm publishes to a single
personal catch-all tap: **`github.com/joelhelbling/homebrew-tap`**, referenced as
`joelhelbling/tap` (Homebrew strips the `homebrew-` prefix). This is the conventional
name for a personal multi-formula tap.

- A tap is just a git repo with a `Formula/` directory holding one `.rb` per tool.
- Each project's goreleaser `brews:` block points at the **same** repository and
  writes its **own** formula file (`Formula/kkullm.rb`), so multiple projects coexist.
- The `HOMEBREW_TAP_GITHUB_TOKEN` secret is shared across projects; it only needs
  write access to this one tap repo.
- **glovebox migration (out of scope, future):** change glovebox's
  `.goreleaser.yaml` brews `repository.name` from `homebrew-glovebox` →
  `homebrew-tap`; its next release writes `Formula/glovebox.rb` into the shared tap.
  The old `homebrew-glovebox` repo can stay in place (existing installs keep working).

## Prerequisite Code Changes

The deployment depends on two small in-repo changes.

### 1. Version wiring

Mirror glovebox's pattern:

- Add `var Version = "dev"` in package `cmd` (e.g. `cmd/version.go`).
- Add a `kkullm version` subcommand and a persistent `--version` flag on the root
  command, both printing `Version`.
- goreleaser injects the real value at build time via ldflags:
  `-X github.com/joelhelbling/kkullm/cmd.Version={{.Version}}`.
- Local `task build` builds keep the `"dev"` default (acceptable; matches glovebox).

### 2. Canonical data directory + busy_timeout

Today `cmd/serve.go` defaults `--db` to `"kkullm.db"` — relative to the current
working directory, so every directory you `cd` into spawns a separate database.
For an installed binary this is surprising. Move to a stable, canonical location:

- New helper (e.g. `cmd/datadir.go`) resolving the default DB path to
  `$XDG_DATA_HOME/kkullm/kkullm.db`, falling back to
  `~/.local/share/kkullm/kkullm.db` when `XDG_DATA_HOME` is unset. The helper
  creates the directory (`0700`) if absent.
- Resolution precedence for the server DB path:
  1. explicit `--db` flag (highest)
  2. `KKULLM_DB` environment variable
  3. resolved XDG default (lowest)
- `cmd/serve.go`'s `--db` flag default becomes the resolved path instead of the
  literal `"kkullm.db"`.
- Add a `busy_timeout` PRAGMA (e.g. 5000 ms) in `db.Open` so that concurrent
  access serializes and waits briefly rather than erroring immediately with
  `SQLITE_BUSY`.

**Single-server caveat (documented, not enforced):** A canonical DB location
reinforces the intended "one server per user/machine" model — the old cwd default
was itself the cause of database fragmentation. If a user nonetheless runs two
`kkullm serve` processes against the same DB file on a *local* filesystem, SQLite's
WAL-mode file locking prevents corruption (reads concurrent, writes serialized).
However, kkullm's SSE EventBus is in-memory per process, so live updates would
**not** propagate between the two servers' connected browsers even though the data
stays consistent. Networked filesystems (NFS) are unsafe for SQLite locking and
should be avoided. The README will state: run one server per dataset, and don't
put the DB on a network share. A future enhancement could add an advisory
single-instance lock.

## Distribution Mechanics

### `.goreleaser.yaml` (new, repo root)

goreleaser v2 config, structured like glovebox's:

- `before.hooks`: `go mod tidy`
- `builds`: one build `id: kkullm`, `main: .`, `binary: kkullm`,
  `env: [CGO_ENABLED=0]`, `goos: [linux, darwin]`, `goarch: [amd64, arm64]`,
  `ldflags: [-s -w, -X github.com/joelhelbling/kkullm/cmd.Version={{.Version}}]`.
- `archives`: tar.gz, name template `kkullm_{Version}_{Os}_{Arch}`, including
  `README.md` and `LICENSE*`.
- `checksum`: `checksums.txt`.
- `snapshot`: `{{ incpatch .Version }}-next`.
- `changelog`: sort asc, exclude `^docs:`, `^test:`, `^ci:`, merge commits.
- `brews`: one entry —
  - `name: kkullm`
  - `repository: { owner: joelhelbling, name: homebrew-tap, branch: main,
    token: "{{ .Env.HOMEBREW_TAP_GITHUB_TOKEN }}" }`
  - `directory: Formula`
  - `homepage`, `description`, `license: MIT`
  - `install: bin.install "kkullm"`
  - `test: system "#{bin}/kkullm", "version"`
- `release`: `github: { owner: joelhelbling, name: kkullm }`, `prerelease: auto`.

### `.github/workflows/release.yaml` (new)

- Trigger: `push` on tags matching `v*`.
- Permissions: `contents: write`.
- Steps: checkout (`fetch-depth: 0`), setup-go, `goreleaser-action@v6` with
  `args: release --clean`.
- Env: `GITHUB_TOKEN: ${{ secrets.GITHUB_TOKEN }}` and
  `HOMEBREW_TAP_GITHUB_TOKEN: ${{ secrets.HOMEBREW_TAP_GITHUB_TOKEN }}`.
- Go version pinned consistent with the repo's `go.mod` toolchain.

### `install.sh` (new, repo root)

Standalone POSIX `sh` script for `curl … | sh`:

- Detect OS (`uname -s` → linux/darwin) and arch (`uname -m` → amd64/arm64).
- Resolve the release to install: latest by default, or a pinned version via a
  `KKULLM_VERSION` env var.
- Download the matching `kkullm_<version>_<os>_<arch>.tar.gz` and `checksums.txt`
  from the GitHub release; verify the archive's checksum before extracting.
- Install the `kkullm` binary into an install dir: prefer `~/.local/bin`, fall
  back to `/usr/local/bin` (with a note if `sudo` is needed); allow override via
  an `INSTALL_DIR` env var.
- Print the resolved version and final binary path; warn if the install dir is
  not on `PATH`.

### Taskfile additions

- Inject version ldflags into the existing `build` task so local builds can carry
  a version when desired (default remains `"dev"`).
- Add a `release-snapshot` task running `goreleaser release --snapshot --clean`
  for local validation of the build matrix and formula generation (no publish).

## Documentation

Update `README.md` with an **Installation** section covering:

- Homebrew: `brew install joelhelbling/tap/kkullm`
- Install script: the `curl … | sh` one-liner (+ `KKULLM_VERSION` / `INSTALL_DIR`)
- Manual: download an archive from the releases page
- A short note on the default data directory (`~/.local/share/kkullm/kkullm.db`),
  the `KKULLM_DB` / `--db` overrides, and the single-server-per-dataset guidance.

Also double-check the `/kkullm:cli` skill (`plugins/kkullm/skills/cli/SKILL.md`)
for any install/version references that should be updated (per CLAUDE.md maintenance note).

## Manual Setup (one-time, performed by maintainer)

Documented as release prerequisites:

1. Create an empty `github.com/joelhelbling/homebrew-tap` repository.
2. Create a GitHub Personal Access Token with write access to that repo; add it
   to the kkullm repository's secrets as `HOMEBREW_TAP_GITHUB_TOKEN`.

## Testing & Verification

- **Data-dir resolver unit test:** `XDG_DATA_HOME` set, unset (HOME fallback),
  and `KKULLM_DB` override; assert directory creation.
- **Version smoke check:** `kkullm version` and `kkullm --version` print the value.
- **goreleaser snapshot:** `task release-snapshot` builds all four platform
  archives and generates `Formula/kkullm.rb` without publishing.
- **Existing suite:** `task test` continues to pass (serve/db changes covered).
- **First real release:** push a `v0.x.0` tag, confirm the GitHub release has all
  archives + checksums and that `Formula/kkullm.rb` lands in `homebrew-tap`; then
  `brew install joelhelbling/tap/kkullm` and `kkullm version` on a clean machine.

## File Summary

| File | Change |
|------|--------|
| `cmd/version.go` | New: `Version` var, `version` subcommand, `--version` flag |
| `cmd/datadir.go` | New: XDG default DB path resolver + dir creation |
| `cmd/serve.go` | `--db` default uses resolver; honor `KKULLM_DB` |
| `db/db.go` | Add `busy_timeout` PRAGMA in `Open` |
| `.goreleaser.yaml` | New: builds, archives, checksum, brews → shared tap, release |
| `.github/workflows/release.yaml` | New: tag-triggered goreleaser run |
| `install.sh` | New: curl-pipe installer |
| `Taskfile.yaml` | Version ldflags in `build`; add `release-snapshot` |
| `README.md` | New Installation section + data-dir notes |
| `plugins/kkullm/skills/cli/SKILL.md` | Review/update install & version references |
