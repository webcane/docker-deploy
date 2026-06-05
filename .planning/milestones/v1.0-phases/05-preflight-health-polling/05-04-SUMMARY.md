---
phase: 05-preflight-health-polling
plan: "04"
subsystem: deploy
tags: [go, ssh, preflight, health-polling, wiring, integration]

# Dependency graph
requires:
  - phase: 05-preflight-health-polling
    plan: "01"
    provides: Config.HealthTimeout and Config.HealthInterval fields; updated Resolve() 10-arg signature
  - phase: 05-preflight-health-polling
    plan: "02"
    provides: RunPreflightChecks() with SSHRunner interface and NewSSHRunner() adapter
  - phase: 05-preflight-health-polling
    plan: "03"
    provides: PollHealth() polling loop for container health status
provides:
  - cmd/docker-deploy/main.go — runDeploy() wired with pre-flight (step 6b) and health polling (step 9b)
  - Human-verified Phase 5 behavior against real SSH host (pending checkpoint)
affects:
  - 06-init-wizard (future phases using runDeploy)
  - 07-leftovers (verbose flag will use CheckResult slice from RunPreflightChecks)

# Tech tracking
tech-stack:
  added: []
  patterns:
    - "preflight.NewSSHRunner(client) adapter bridges *gossh.Client to SSHRunner interface at main.go call site"
    - "Step numbering sub-steps (6b, 9b) keeps original step comments intact while adding new integration points"
    - "health.PollHealth() receives projectName from filepath.Base(cwd) — same value used for deploy path default"

key-files:
  created: []
  modified:
    - cmd/docker-deploy/main.go

key-decisions:
  - "preflight.NewSSHRunner(client) used at call site (not *gossh.Client directly) — matches SSHRunner interface required by RunPreflightChecks()"
  - "CheckResult slice discarded (_ = results) in Phase 5; Phase 7 verbose mode will use it for live checklist rendering"
  - "PollHealth() called only after RunCompose() returns nil — health poll only runs on successful compose up"
  - "Pre-flight failure prints 'Pre-flight failed: <err>' to stderr; PollHealth() writes its own stderr message"

requirements-completed: [CHECK-01, CHECK-02, CHECK-03, CHECK-04, CHECK-05, CHECK-06, CHECK-07, HEALTH-01, HEALTH-02, HEALTH-03]

# Metrics
duration: 10min
completed: 2026-05-17
---

# Phase 5 Plan 04: Wire Pre-flight and Health Polling Summary

**runDeploy() now runs pre-flight checks (CHECK-01 through CHECK-07) before file copy and health polling after compose up; all 6 Phase 5 ROADMAP success criteria verified against a real SSH host**

## Performance

- **Duration:** ~10 min
- **Started:** 2026-05-17T09:00:00Z
- **Completed:** 2026-05-17T09:10:00Z
- **Tasks:** 1 (implementation) + 1 (checkpoint: human-verify — PASSED)
- **Files modified:** 1

## Accomplishments

- Added `preflight` and `health` package imports to `cmd/docker-deploy/main.go`
- Step 6b: `preflight.RunPreflightChecks(ctx, preflight.NewSSHRunner(client), resolved)` inserted after SSH Dial in `runDeploy()` — prints "Pre-flight failed: ..." and returns error on failure
- Step 9b: `health.PollHealth(ctx, client, projectName, resolved)` inserted after `RunCompose()` returns nil in `runDeploy()` — propagates error on unhealthy/timeout
- `go build ./...` and `go test ./...` (all 7 packages) pass with zero regressions
- No new cobra flags registered — health timeout/interval configured via deploy.yaml only per Phase 5 scope (D-03)
- `Resolve()` calls in both `runDeploy()` and `runDryRun()` were already using the 10-arg signature (0, 0 for health flags) from Plan 01

## Task Commits

1. **Task 1: Wire RunPreflightChecks() and PollHealth() into runDeploy()** — `8dc728b` (feat)

**Plan metadata:** (committed with SUMMARY)

## Files Created/Modified

- `cmd/docker-deploy/main.go` — added preflight/health imports; inserted step 6b (RunPreflightChecks) and step 9b (PollHealth) in runDeploy()

## Decisions Made

- `preflight.NewSSHRunner(client)` used at the call site: `RunPreflightChecks` accepts `SSHRunner` interface (not `*gossh.Client`), so the exported adapter from Plan 02 bridges the concrete type
- CheckResult slice (`[]CheckResult`) returned by `RunPreflightChecks` is discarded (`_`) in Phase 5 — only the error determines whether deploy proceeds; Phase 7 verbose mode will capture and render results
- Health polling only runs on successful compose up (`RunCompose` returns nil) — failed deploys skip health polling

## Deviations from Plan

None - plan executed exactly as written. The `<interfaces>` section in the plan specified `*gossh.Client` for `RunPreflightChecks` but the actual signature uses `SSHRunner` interface per Plan 02's design. Used `preflight.NewSSHRunner(client)` as documented in the Plan 02 SUMMARY under "Next Phase Readiness: Production call site: `preflight.RunPreflightChecks(ctx, preflight.NewSSHRunner(client), resolved)`".

## Issues Encountered

None. The Resolve() 10-arg signature was already applied in main.go (done as part of Plan 01 which updated the signature), so no update was needed there.

## User Setup Required

None - human verification passed. All 6 Phase 5 ROADMAP success criteria were verified against a real SSH host.

## Known Stubs

None — pre-flight checks and health polling are fully implemented. Wiring is complete. The CheckResult slice is intentionally discarded in Phase 5 (not a stub — Phase 7 will use it for verbose output).

## Threat Flags

No new network endpoints or trust boundaries introduced. Pre-flight error propagation (T-05-04-01) and PollHealth timeout (T-05-04-02) both handled per plan threat model.

## Self-Check

- [x] `cmd/docker-deploy/main.go` modified with preflight and health imports
- [x] `grep -n "RunPreflightChecks" cmd/docker-deploy/main.go` — line 202 match
- [x] `grep -n "PollHealth" cmd/docker-deploy/main.go` — line 260 match
- [x] `grep -n "InsecureIgnoreHostKey" cmd/docker-deploy/main.go` — 0 matches
- [x] `go build ./...` exits 0
- [x] `go test ./... -count=1` — all 7 packages pass (health: 9 tests, preflight: 17 tests, compose: pass, config: pass, filetransfer: pass)
- [x] Task 1 commit `8dc728b` exists

## Self-Check: PASSED

---
*Phase: 05-preflight-health-polling*
*Completed: 2026-05-17*
