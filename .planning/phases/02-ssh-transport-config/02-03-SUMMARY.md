---
phase: 02-ssh-transport-config
plan: "03"
subsystem: cli
tags: [dry-run, config, ssh, wiring]
dependency_graph:
  requires: [02-01, 02-02]
  provides: [dry-run-e2e-flow]
  affects: [cmd/docker-deploy/main.go]
tech_stack:
  added: []
  patterns: [flag-override, goroutine-timeout-dial]
key_files:
  created: []
  modified:
    - cmd/docker-deploy/main.go
decisions:
  - "Auth method indicator is best-effort: SSH_AUTH_SOCK presence determines ssh-agent vs key file label"
  - "runDryRun extracted as a named helper function to keep RunE readable"
  - "client.Conn.ServerVersion() cast to string for human-readable output"
metrics:
  duration: "10 minutes"
  completed: "2026-05-14T09:30:34Z"
  tasks_completed: 1
  tasks_total: 2
  files_modified: 1
---

# Phase 2 Plan 03: Dry-Run Wiring Summary

Wire config resolution and SSH dial together in `cmd/docker-deploy/main.go` so that `docker deploy --dry-run` performs a real connectivity check and prints a human-readable summary.

## What Was Built

`--dry-run` wires the config and SSH packages end-to-end:

1. `os.Getwd()` → `filepath.Base()` → `projectName`
2. `config.LoadFile(cwd)` → loads `deploy.yaml` (zero value if absent)
3. `config.Resolve(flagHost, flagPath, fileConfig, projectName)` → applies flag > file > default precedence
4. Validates `resolved.Host.Hostname != ""` — returns actionable error if no host set
5. Builds `sshpkg.DialConfig` with 10s timeout, `os.Stdin`, `os.Stderr`
6. `sshpkg.Dial(context.Background(), dialCfg)` — real SSH handshake with knownhosts verification
7. Prints resolved config + `client.Conn.ServerVersion()` on success; error + non-zero exit on failure

Non-dry-run path unchanged: returns `"deploy not implemented yet"`.

## Tasks

| # | Name | Commit | Status |
|---|------|--------|--------|
| 1 | Wire --dry-run flow in deploy RunE | 4bd777d | complete |
| 2 | Human verify checkpoint | — | awaiting |

## Deviations from Plan

None — plan executed exactly as written.

## Known Stubs

None — dry-run produces real SSH connection output.

## Threat Flags

None — implementation matches the plan threat model. No secrets or key material printed; `client.Conn.ServerVersion()` reveals only the SSH server software version (by design, D-12).

## Self-Check

- [x] `cmd/docker-deploy/main.go` exists and contains `config.Resolve(` and `sshpkg.Dial(`
- [x] Commit `4bd777d` exists
- [x] `go build ./cmd/docker-deploy/` exits 0
- [x] `go vet ./...` exits 0
- [x] No `InsecureIgnoreHostKey` anywhere in codebase
- [x] `fmt.Errorf("deploy not implemented yet")` still present for non-dry-run path

## Self-Check: PASSED
