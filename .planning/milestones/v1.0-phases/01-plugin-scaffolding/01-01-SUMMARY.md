---
phase: 01-plugin-scaffolding
plan: 01
subsystem: plugin-scaffolding
tags: [go-module, docker-cli-plugin, cobra, plugin-handshake, makefile]
dependency_graph:
  requires: []
  provides:
    - go.mod with docker/cli v29.4.3 locked as first dependency
    - cmd/docker-deploy/main.go with plugin.Run() wiring and cobra root
    - Makefile with build/install/test targets
    - .gitignore for generated artifacts
  affects: []
tech_stack:
  added:
    - github.com/docker/cli v29.4.3+incompatible
    - github.com/spf13/cobra v1.10.2
    - Go 1.26.3
  patterns:
    - Docker CLI plugin protocol via plugin.Run()
    - cobra.Command root with no user-facing flags (skeleton)
    - ldflags version injection via -X main.version
key_files:
  created:
    - go.mod
    - go.sum
    - cmd/docker-deploy/main.go
    - internal/.gitkeep
    - Makefile
    - .gitignore
  modified: []
decisions:
  - docker/cli v29.4.3+incompatible locked first per CLAUDE.md Rule 4 before any other dependency
  - Metadata type from github.com/docker/cli/cli-plugins/metadata (not manager) in v29.4.3
  - import path github.com/docker/cli/cli/command (double cli/) for command.Cli interface
  - go mod tidy resolved 40+ transitive deps from docker/cli+incompatible automatically
metrics:
  duration: ~15 minutes
  completed_date: "2026-05-13"
  tasks_completed: 3
  tasks_total: 3
  files_created: 6
  files_modified: 0
---

# Phase 1 Plan 1: Plugin Scaffolding Bootstrap Summary

**One-liner:** Docker CLI plugin skeleton using plugin.Run() from docker/cli v29.4.3 with cobra root, Makefile build/install/test targets, and verified `docker deploy --help` output.

## Objective

Bootstrap the Go module, implement the Docker CLI plugin handshake, and wire a minimal cobra root command. Lock the plugin interface contract and Go module path before any business logic is layered on top.

## Tasks Completed

| Task | Name | Commit | Files |
|------|------|--------|-------|
| 1 | Bootstrap Go module and lock docker/cli | 6be78ab | go.mod, go.sum |
| 2 | Implement plugin entry point with plugin.Run() | 1f006da | cmd/docker-deploy/main.go, internal/.gitkeep, go.mod, go.sum |
| 3 | Add Makefile and .gitignore; verify end-to-end | 205042a | Makefile, .gitignore |

## Verification Results

All 7 plan verification criteria passed:

1. `go build ./cmd/docker-deploy/` exits 0
2. `go vet ./...` exits 0
3. `go test ./...` exits 0
4. `bin/docker-deploy docker-cli-plugin-metadata` outputs `{"SchemaVersion":"0.1.0","Vendor":"mniedre","Version":"dev","ShortDescription":"Deploy a docker-compose project to a remote VPS"}`
5. `make build && make install` exits 0, `~/.docker/cli-plugins/docker-deploy` installed
6. `docker deploy --help` shows cobra-generated usage output via Docker CLI
7. `go.mod` starts with `module github.com/mniedre/docker-deploy` and includes `github.com/docker/cli`

## Success Criteria Met

- `docker deploy --help` displays usage via the Docker CLI (PLUG-01, PLUG-02)
- `docker-deploy docker-cli-plugin-metadata` returns JSON with `"SchemaVersion":"0.1.0"` (PLUG-03)
- `go.mod` has `github.com/docker/cli` as first-pinned dependency (CLAUDE.md Rule 4)
- `make build`, `make install`, `make test` all exit 0
- No user-facing flags registered (D-07 — skeleton only)
- `var version = "dev"` is overrideable via `-ldflags "-X main.version=..."` (D-08)

## Deviations from Plan

### Auto-fixed Issues

**1. [Rule 3 - Blocking] Transitive dependencies missing for docker/cli +incompatible**
- **Found during:** Task 2
- **Issue:** `github.com/docker/cli v29.4.3+incompatible` is a pre-modules version that does not ship a `go.mod`. `go build` failed with 30+ missing package errors for transitive deps (moby, opentelemetry, containerd, etc.).
- **Fix:** Ran `go mod tidy` after creating main.go, which resolved and pinned all 40+ transitive deps automatically. No manual intervention needed.
- **Files modified:** go.mod, go.sum
- **Commit:** 1f006da

**2. [Rule 1 - Bug] Incorrect import path for command.Cli interface**
- **Found during:** Task 2
- **Issue:** Plan specified `github.com/docker/cli/command` for the `command.Cli` type but the actual path in docker/cli v29.4.3 is `github.com/docker/cli/cli/command` (double `cli/`).
- **Fix:** Corrected import path to `github.com/docker/cli/cli/command` after inspecting the module cache.
- **Files modified:** cmd/docker-deploy/main.go
- **Commit:** 1f006da (same task commit)

**3. [Rule 3 - Blocking] Go not installed on system**
- **Found during:** Task 1
- **Issue:** Go was not installed on the developer machine.
- **Fix:** Installed Go 1.26.3 via Homebrew (`brew install go`). Go is now available at `/opt/homebrew/bin/go`.
- **Impact:** No code changes; prerequisite installed.

**4. [Rule 2 - Untracked binary] Root-level docker-deploy binary**
- **Found during:** Task 3
- **Issue:** `go build ./cmd/docker-deploy/` in Task 2 produced a binary at repo root that was not in .gitignore.
- **Fix:** Added `docker-deploy` to .gitignore alongside other generated artifacts.
- **Files modified:** .gitignore
- **Commit:** 205042a

## Decisions Made

| Decision | Rationale |
|----------|-----------|
| Use `github.com/docker/cli/cli/command` (double `cli/`) import path | This is the actual package path in docker/cli v29.4.3+incompatible; the `cli-plugins/plugin` package uses it internally |
| Allow `go mod tidy` to resolve transitive deps | docker/cli+incompatible requires many transitive deps; `go mod tidy` is the correct tool to pull them in after first code import |
| Pin `docker-deploy` root binary in .gitignore | Prevents accidental commit of locally-built binary from `go build ./cmd/docker-deploy/` calls |

## Known Stubs

None — this plan implements the minimal plugin skeleton as specified. No data flows, no UI rendering.

## Threat Flags

None — no new security-relevant surface introduced beyond what was in the plan's threat model. The plan's T-01-01 through T-01-04 dispositions remain accurate.

## Self-Check: PASSED

- go.mod exists and contains correct module path and docker/cli dep
- go.sum exists and is populated
- cmd/docker-deploy/main.go exists with plugin.Run() wiring
- internal/.gitkeep exists
- Makefile exists with build/install/test targets
- .gitignore exists with bin/, dist/, *.exe, .DS_Store, *.log, docker-deploy
- Commits 6be78ab, 1f006da, 205042a all exist in git log
