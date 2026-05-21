---
phase: quick-260521-afl
plan: 01
subsystem: cmd/docker-deploy
tags: [bug-fix, output-format, tdd]
dependency_graph:
  requires: []
  provides: [formatHostTarget helper, corrected deploy complete message]
  affects: [cmd/docker-deploy/main.go, cmd/docker-deploy/main_test.go]
tech_stack:
  added: []
  patterns: [table-driven tests, package-level helper function]
key_files:
  created: []
  modified:
    - cmd/docker-deploy/main.go
    - cmd/docker-deploy/main_test.go
decisions:
  - formatHostTarget accepts port as int and checks for 0 or 22 to suppress colon — both values mean "default SSH port"
metrics:
  duration: ~4 min
  completed: 2026-05-21
---

# Quick Task 260521-afl: Fix Deploy Complete Status Message Omitting Colon for Default Port

**One-liner:** Added `formatHostTarget()` helper that omits the colon separator for default SSH port 22, fixing the confusing `host:/path` format to `host/path`.

## What Was Done

The deploy complete message in `runDeploy()` used `%s:%s` formatting, producing output like:

```
Deploy complete: 5 files copied to 192.168.1.99:/opt/myapp
```

This looks like `host:port/path` with an empty port, which is confusing. The fix:

- Added `formatHostTarget(hostname string, port int, path string) string` helper at package level
- Port 0 or 22 → `hostname + path` (no colon)
- Any other port → `hostname:PORT + path`
- Updated line 346 in `runDeploy()` to use the helper with the already-resolved `port` variable

Correct output examples:
- Default port 22: `Deploy complete: 5 files copied to 192.168.1.99/opt/myapp`
- Custom port 2222: `Deploy complete: 5 files copied to 192.168.1.99:2222/opt/myapp`

## Task Commits

| Task | Name | Commit | Files |
|------|------|--------|-------|
| 1 (RED) | TestFormatHostTarget failing tests | 809373d | cmd/docker-deploy/main_test.go |
| 1 (GREEN) | formatHostTarget implementation + message fix | 1f6103a | cmd/docker-deploy/main.go |

## TDD Gate Compliance

- RED gate: `test(quick-260521-afl-01)` commit `809373d` — 3 table-driven sub-tests, all failing (undefined: formatHostTarget)
- GREEN gate: `feat(quick-260521-afl-01)` commit `1f6103a` — all 3 sub-tests pass, full test suite clean

## Verification

```
go test ./cmd/docker-deploy/... -run TestFormatHostTarget -v
# === RUN   TestFormatHostTarget/default_port_22_omits_colon   --- PASS
# === RUN   TestFormatHostTarget/custom_port_includes_colon_and_port   --- PASS
# === RUN   TestFormatHostTarget/zero_port_treated_as_default   --- PASS

go test ./...
# All packages pass, no regressions
```

## Deviations from Plan

None — plan executed exactly as written.

## Known Stubs

None.

## Threat Flags

None — output format change only; no new network endpoints, auth paths, or trust boundary changes.

## Self-Check: PASSED

- [x] `cmd/docker-deploy/main.go` contains `formatHostTarget`
- [x] `cmd/docker-deploy/main_test.go` contains `TestFormatHostTarget`
- [x] RED commit `809373d` exists
- [x] GREEN commit `1f6103a` exists
- [x] `go test ./...` passes with no regressions
