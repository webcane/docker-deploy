---
phase: 05-preflight-health-polling
plan: "02"
subsystem: preflight
tags: [go, ssh, preflight, tdd, security]

# Dependency graph
requires:
  - phase: 05-preflight-health-polling
    plan: "01"
    provides: Config.HealthTimeout and Config.HealthInterval fields consumed by health polling
  - phase: 04-core-deploy-loop
    provides: filetransfer.ShellQuote() reused for remote shell argument quoting
affects:
  - 05-04-wire-main (RunPreflightChecks wired into runDeploy())

# Tech tracking
tech-stack:
  added: []
  patterns:
    - "SSHRunner/Session interfaces narrow the gossh.Client surface for testable injection"
    - "NewSSHRunner(*gossh.Client) adapter bridges concrete type to interface at production call sites"
    - "Separate NewSession() per SSH command — 8 session opens across 7 checks"
    - "Fail-fast ordering: CHECK-01 → CHECK-02 → CHECK-03 → CHECK-07 → CHECK-06 → CHECK-04"
    - "CHECK-05 (sudo check) is conditional — invoked only when CHECK-04 or CHECK-06 needs sudo"

key-files:
  created:
    - internal/preflight/checks.go
    - internal/preflight/checks_test.go
  modified: []

key-decisions:
  - "SSHRunner interface accepted by RunPreflightChecks (not *gossh.Client directly) to keep tests free of real SSH"
  - "NewSSHRunner(*gossh.Client) exported adapter for production wiring in Plan 04"
  - "CHECK-03 (daemon) and CHECK-07 (root) are warn-only per D-05 — never return error"
  - "CHECK-05 (sudo) is conditional: only called when CHECK-04 or CHECK-06 need to escalate"
  - "Error message for CHECK-04 failure includes exact fix command (sudo usermod -aG docker <user>)"
  - "Error message for CHECK-05 failure includes exact /etc/sudoers.d fix"

# Metrics
duration: 20min
completed: 2026-05-17
---

# Phase 5 Plan 02: Preflight Checks Summary

**SSHRunner interface + RunPreflightChecks() implementing CHECK-01 through CHECK-07 with fail-fast ordering, warn-only checks for daemon/root, and sudo auto-fix for docker group and target dir**

## Performance

- **Duration:** ~20 min
- **Started:** 2026-05-17T08:37:00Z
- **Completed:** 2026-05-17T08:57:00Z
- **Tasks:** 1 (TDD: RED + GREEN)
- **Files created:** 2

## Accomplishments

- Created `internal/preflight/checks.go` with:
  - `Session` interface (Output, Run, Close) matching `*gossh.Session` method signatures
  - `SSHRunner` interface (NewSession) with `sshClientRunner` adapter for `*gossh.Client`
  - `NewSSHRunner(*gossh.Client) SSHRunner` exported constructor for production use
  - `CheckResult` struct (Name, Status, Message) for Phase 7 verbose extension
  - `RunPreflightChecks(ctx, SSHRunner, Config) ([]CheckResult, error)` with all 7 checks
  - `checkSudo` (CHECK-05) called conditionally only when needed
  - All remote commands use separate `NewSession()` per call (8 total across 7 checks)
  - `filetransfer.ShellQuote()` applied to all path and username args (T-05-02-01)
- Created `internal/preflight/checks_test.go` with 17 tests covering:
  - All 7 check behaviors per the plan's `<behavior>` spec
  - Compile-time SSHRunner interface guard via `NewSSHRunner`
  - Fake SSH client injecting configurable exit codes and stdout per command substring

## Task Commits

TDD task with RED and GREEN phases committed separately:

1. **RED: Failing tests for CHECK-01 through CHECK-07** — `897b7de` (test)
2. **GREEN: RunPreflightChecks + all 7 checks implemented** — `56d215a` (feat)

## Files Created/Modified

- `internal/preflight/checks.go` — new: full preflight package with RunPreflightChecks, 7 check functions, SSHRunner interface, NewSSHRunner adapter
- `internal/preflight/checks_test.go` — new: 17 TDD tests; fakeSSHClient/fakeSession inject per-command responses

## Decisions Made

- `SSHRunner` interface accepted by `RunPreflightChecks` instead of `*gossh.Client` directly — enables test injection without real SSH connections; production callers use `NewSSHRunner(client)`
- `CHECK-05` (sudo check) is conditional: only invoked when `checkDockerGroup` or `checkTargetDir` needs to escalate to sudo — not run unconditionally (per must_haves spec)
- Error messages for `CHECK-04` and `CHECK-05` failures include exact operator fix commands for actionable remediation
- Execution order places `CHECK-03` and `CHECK-07` (warning-only) after the two hard-block checks, before the auto-fix checks

## Deviations from Plan

### Auto-fixed Issues

**1. [Rule 1 - Bug] Error message substring mismatch**
- **Found during:** GREEN phase test run
- **Issue:** Test `TestCheck04_UserNotInDockerGroup_SudoUsermodFails` checked for `"user not in docker group"` but initial implementation used `"user deploy is not in the docker group"` — different word order caused substring mismatch
- **Fix:** Changed error string to `"preflight: user not in docker group (%s); fix: sudo usermod -aG docker %s ..."` matching the test expectation from the plan spec
- **Files modified:** `internal/preflight/checks.go`

**2. [Rule 2 - Missing] SSHRunner compile-time guard**
- **Found during:** GREEN phase — `*gossh.Client` cannot satisfy `SSHRunner` directly because Go interfaces don't support covariant return types (`NewSession()` returns `(*gossh.Session, error)` not `(Session, error)`)
- **Fix:** Added `NewSSHRunner(*gossh.Client) SSHRunner` exported adapter; updated the compile-time test to use `preflight.NewSSHRunner(client)` instead of `(*gossh.Client)(nil)`
- **Files modified:** `internal/preflight/checks.go`, `internal/preflight/checks_test.go`

## Issues Encountered

None beyond the two auto-fixed deviations above.

## User Setup Required

None.

## Next Phase Readiness

- `RunPreflightChecks(ctx, SSHRunner, Config) ([]CheckResult, error)` is ready for Plan 04 wiring
- Production call site: `preflight.RunPreflightChecks(ctx, preflight.NewSSHRunner(client), resolved)`
- `CheckResult` struct ready for Phase 7 verbose checklist rendering
- All 17 tests pass; `go build ./...` clean

## Known Stubs

None — all 7 checks are fully implemented against real SSH session semantics.

## Threat Flags

None — threat model T-05-02-01 (shell injection via cfg.Host.User and cfg.Path) is mitigated by `filetransfer.ShellQuote()` applied to all user-supplied values in remote commands.

## Self-Check

- [x] `internal/preflight/checks.go` exists and compiles: `go build ./internal/preflight/...` exits 0
- [x] `RunPreflightChecks` exported with correct signature: `func RunPreflightChecks(ctx context.Context, client SSHRunner, cfg config.Config) ([]CheckResult, error)`
- [x] `CheckResult` struct exported with `Name`, `Status`, `Message` string fields
- [x] `go test ./internal/preflight/... -count=1` — 17 tests pass
- [x] No `InsecureIgnoreHostKey` in `internal/preflight/checks.go`
- [x] `NewSession()` appears 8 times in `checks.go` (>= 6 required; one per check that runs a command)
- [x] CHECK-03 test: `RunPreflightChecks` returns nil when only docker info fails
- [x] CHECK-07 test: `RunPreflightChecks` returns nil when `cfg.Host.User == "root"`
- [x] CHECK-02 test: error returned when docker compose version fails and docker-compose v1 detected
- [x] RED commit `897b7de` — exists
- [x] GREEN commit `56d215a` — exists

## Self-Check: PASSED

---
*Phase: 05-preflight-health-polling*
*Completed: 2026-05-17*
