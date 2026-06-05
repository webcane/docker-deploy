---
phase: 05-preflight-health-polling
plan: "03"
subsystem: health
tags: [go, ssh, docker, health-polling, tdd]

# Dependency graph
requires:
  - phase: 05-preflight-health-polling
    plan: "01"
    provides: Config.HealthTimeout and Config.HealthInterval fields consumed by PollHealth()
  - phase: 03-file-copy
    provides: filetransfer.ShellQuote() used for remote command injection protection
provides:
  - internal/health/poll.go — PollHealth() function with full terminal state handling
  - sessionOpener/sessionOutput interfaces for test injection
  - All HEALTH-01, HEALTH-02, HEALTH-03 behaviors implemented and tested
affects:
  - 05-04-wire-main

# Tech tracking
tech-stack:
  added: []
  patterns:
    - "sessionOpener interface injection: testable SSH exec without real server (same pattern as compose/run.go context wiring)"
    - "ticker + timer select loop: standard Go timeout pattern for polling loops"
    - "done map for terminal state tracking: O(n) container completion check per poll tick"
    - "ShellQuote applied at every shell boundary: projectName and container names both quoted (T-05-03-01, T-05-03-02)"

key-files:
  created:
    - internal/health/poll.go
    - internal/health/poll_test.go
  modified: []

key-decisions:
  - "sessionOpener interface defined in poll.go (not test file) so both fakeClient and sshClientRunner satisfy it from same package"
  - "HealthInterval=0 treated as 1ms in pollHealthWithRunner to allow test-fast mode without blocking"
  - "Inspect failures (session error) treated as unknown/warning — container continues polling rather than hard-failing"
  - "unexpected status strings (not healthy/unhealthy/starting/none/'') fall through to continue polling (T-05-03-03 safe default)"

patterns-established:
  - "Test-fast polling: HealthInterval=0 → 1ms effective for unit tests, production Resolve() always sets >= 1"
  - "fakeClient scripted responses: ordered slice of fakeSession, exhaustion returns error to catch infinite loop bugs"

requirements-completed: [HEALTH-01, HEALTH-02, HEALTH-03]

# Metrics
duration: 20min
completed: 2026-05-17
---

# Phase 5 Plan 03: Health Polling Summary

**PollHealth() polling loop with docker ps label enumeration, per-container docker inspect, and all D-13 terminal states (healthy/unhealthy/no-healthcheck/timeout/context-cancel) implemented via sessionOpener interface injection**

## Performance

- **Duration:** ~20 min
- **Started:** 2026-05-17T00:00:00Z
- **Completed:** 2026-05-17T00:20:00Z
- **Tasks:** 1 (TDD: RED + GREEN)
- **Files modified:** 2

## Accomplishments
- Implemented PollHealth() with correct signature: `func PollHealth(ctx context.Context, client *gossh.Client, projectName string, cfg config.Config) error`
- Container enumeration via `docker ps --filter label=com.docker.compose.project=<name> --format '{{.Names}}'`
- Poll loop using `time.NewTicker(healthInterval)` + `time.NewTimer(healthTimeout)` in a select
- All D-13 terminal states handled: healthy (mark done), unhealthy (immediate error), "" or "none" (warning + pass), starting (continue), timeout (error), context cancel (error)
- ShellQuote() applied to both projectName and container names (T-05-03-01, T-05-03-02)
- Separate NewSession() per command per CLAUDE.md Rule 3
- 9 TDD tests covering all specified behaviors — all pass

## Task Commits

TDD task with RED and GREEN phases committed separately:

1. **RED: Failing tests for PollHealth()** - `d9fabe1` (test)
2. **GREEN: Full PollHealth() implementation** - `c4e2942` (feat)

**Plan metadata:** (committed with SUMMARY)

_Note: TDD tasks have separate RED (test) and GREEN (feat) commits_

## Files Created/Modified
- `internal/health/poll.go` - PollHealth(), pollHealthWithRunner(), sessionOpener/sessionOutput interfaces, sshClientRunner adapter, listContainers(), pollContainers(), inspectHealth()
- `internal/health/poll_test.go` - 9 TDD tests: NoContainers, AllHealthy, UnhealthyImmediate, NoHealthcheck_EmptyStatus, NoHealthcheck_NoneStatus, StartingThenHealthy, Timeout, Mixed, ContextCancel

## Decisions Made
- `sessionOpener`/`sessionOutput` interfaces defined in `poll.go` (not test file) since both files are in `package health` — avoids duplicate type declarations in the same package
- `HealthInterval=0` → 1ms effective inside `pollHealthWithRunner` so tests with `defaultCfg(T, 0)` run fast; production `Resolve()` always sets `HealthInterval >= 1`
- Inspect session errors (e.g. container exited mid-poll) treated as unknown with warning, not hard fail — retries are safe
- `fakeClient.newSession()` ignores the cmd argument (passes it along for production use via `sshSessionWrapper.cmd`); fake sessions return pre-scripted output regardless of cmd

## Deviations from Plan

None — plan executed exactly as written. Implementation and test structure match the specified behavior and action sections.

## Issues Encountered

None.

## User Setup Required

None - no external service configuration required.

## Known Stubs

None. PollHealth() is fully implemented with no placeholder behavior.

## Threat Flags

No new network endpoints, auth paths, or trust boundaries introduced beyond those documented in the plan's threat model.

## Next Phase Readiness
- `PollHealth()` is ready to be called from `runDeploy()` in Plan 04 after `RunCompose()` returns nil
- Signature: `PollHealth(ctx, client, projectName, cfg)` where `projectName = filepath.Base(localDir)`
- Returns nil on success (all healthy or no containers), non-nil on unhealthy/timeout/ctx-cancel
- No further changes to `internal/health` package needed for Phase 5

## Self-Check

- [x] `internal/health/poll.go` — exists and compiles: `go build ./internal/health/... exits 0`
- [x] `internal/health/poll_test.go` — exists with 9 test functions
- [x] `PollHealth` exported with correct signature: `func PollHealth(ctx context.Context, client *gossh.Client, projectName string, cfg config.Config) error`
- [x] RED commit `d9fabe1` — exists (test: failing tests for PollHealth())
- [x] GREEN commit `c4e2942` — exists (feat: implement PollHealth())
- [x] `go test ./internal/health/... -count=1` — 9 tests pass
- [x] `go build ./...` — no compilation errors
- [x] `grep -n "InsecureIgnoreHostKey" internal/health/poll.go` — no matches

## Self-Check: PASSED

---
*Phase: 05-preflight-health-polling*
*Completed: 2026-05-17*
