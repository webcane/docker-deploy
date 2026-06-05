---
phase: 02-ssh-transport-config
plan: "01"
subsystem: config
tags: [config, ssh, go-modules, cobra]
dependency_graph:
  requires: []
  provides:
    - internal/config (Config, Host, FileConfig, Resolve, ParseHost, LoadFile)
    - go.mod entries for golang.org/x/crypto, github.com/pkg/sftp, gopkg.in/yaml.v3
    - cobra flags --host, --path, --dry-run on deploy command
  affects:
    - cmd/docker-deploy/main.go (flags added)
    - go.mod / go.sum (deps added)
tech_stack:
  added:
    - golang.org/x/crypto v0.51.0 (SSH transport — used in plan 02-02)
    - github.com/pkg/sftp v1.13.10 (SFTP file transfer — used in plan 03)
    - gopkg.in/yaml.v3 v3.0.1 (deploy.yaml parsing)
  patterns:
    - Three-tier config precedence: flag > deploy.yaml > built-in default
    - net/url-based SSH URI parsing with scheme validation
    - YAML unmarshal into versioned struct with target subsection
key_files:
  created:
    - internal/config/config.go
    - internal/config/config_test.go
  modified:
    - go.mod
    - go.sum
    - cmd/docker-deploy/main.go
decisions:
  - "deploy.yaml uses version + target subsection (not flat keys) to leave room for future multi-target extension (D-10, CFG-05)"
  - "golang.org/x/crypto and github.com/pkg/sftp pinned via go mod edit to preserve them as direct deps before source imports exist; go mod tidy promotes them once 02-02 source uses them"
  - "Config.DryRun field present in struct for 02-03 wiring; not yet consumed in RunE"
metrics:
  duration_minutes: 5
  completed_date: "2026-05-14T09:18:40Z"
  tasks_completed: 3
  tasks_total: 3
---

# Phase 2 Plan 01: Config Resolution & Dependency Setup Summary

Config resolution package with three-tier precedence (flag > deploy.yaml > default), ssh:// URI parsing with scheme validation, versioned deploy.yaml schema, and --host/--path/--dry-run flags registered on the cobra deploy command.

## Tasks Completed

| # | Task | Commit | Files |
|---|------|--------|-------|
| 1 | Add SSH/SFTP dependencies to go.mod | cba4679 | go.mod, go.sum |
| 2 | Implement internal/config package | e3aa127 | internal/config/config.go, internal/config/config_test.go, go.mod, go.sum |
| 3 | Register --host, --path, --dry-run flags | e2d8843 | cmd/docker-deploy/main.go |

## Verification Results

- `go test ./internal/config/... -v -count=1`: 16 tests PASS (all table-driven cases)
- `go build ./...`: exits 0
- `go vet ./...`: exits 0
- `docker deploy --help`: shows --host, --path, --dry-run flags
- config.Resolve() precedence: flag overrides file; file overrides default
- deploy.yaml schema uses `version` + `target.host`/`target.path` (not flat keys)

## Deviations from Plan

### Auto-fixed Issues

None.

### Noted Behaviors

**1. [Expected - Go modules] golang.org/x/crypto listed as indirect until 02-02 imports it**

- **Found during:** Task 1 / Task 2 verification
- **Issue:** `go mod tidy` removes direct dep markers for packages not imported by any source file. This is correct Go behavior — not a bug.
- **Resolution:** Used `go mod edit -require` to pin golang.org/x/crypto, github.com/pkg/sftp, and github.com/kr/fs as direct deps without an `// indirect` comment. They will remain direct once plan 02-02 writes SSH source files that import `golang.org/x/crypto/ssh` and `github.com/pkg/sftp`.
- **Impact:** None — build passes, deps are present in go.mod and go.sum.

## Known Stubs

- `cmd/docker-deploy/main.go` RunE: non-dry-run path returns `fmt.Errorf("deploy not implemented yet")` — intentional; full deploy wired in later phases.
- `cmd/docker-deploy/main.go` RunE: dry-run path returns `fmt.Errorf("--dry-run: not wired yet")` — intentional; connectivity verification wired in plan 02-03.

## Threat Surface Scan

All mitigations from the plan's threat model are implemented:

| Threat | Component | Implementation |
|--------|-----------|----------------|
| T-02-01 Tampering | deploy.yaml | `yaml.Unmarshal` returns error on malformed YAML; no panic path |
| T-02-02 Spoofing | ParseHost | Rejects non-ssh:// schemes; requires non-empty Hostname; defaults port 22 |
| T-02-03 Info Disclosure | Config.Path | Accept — path is user-visible by design |

## Self-Check: PASSED

- internal/config/config.go: EXISTS
- internal/config/config_test.go: EXISTS
- cmd/docker-deploy/main.go: MODIFIED
- Commits cba4679, e3aa127, e2d8843: EXIST in git log
