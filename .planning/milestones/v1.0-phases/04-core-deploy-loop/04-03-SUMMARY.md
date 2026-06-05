---
phase: 04-core-deploy-loop
plan: "03"
subsystem: deploy
tags: [cobra, ssh, sftp, docker-compose, compose, exit-codes, streaming]

# Dependency graph
requires:
  - phase: 04-01
    provides: Config.ComposeFile field and updated 8-arg Resolve() signature
  - phase: 04-02
    provides: compose.RunCompose() primitive with PTY/pipe output routing and exit code propagation
  - phase: 03-core-file-copy
    provides: Upload() function and atomic staging pattern wired into runDeploy()
provides:
  - Full deploy loop in main.go: copy files then run compose up, with streamed output and correct exit codes
  - --compose-file flag registered on cobra command
  - Basename validation (T-04-03-01 mitigation) rejecting paths with separators
  - Auto-detection of compose.yaml / docker-compose.yml from project root
  - Human-verified end-to-end behavior across 6 test scenarios
affects: [phase-5-preflight, phase-6-init-wizard]

# Tech tracking
tech-stack:
  added: []
  patterns:
    - "RunCompose() called after Upload() succeeds; error returned directly (RunCompose writes its own failure line)"
    - "filepath.Base(resolved.ComposeFile) == resolved.ComposeFile as injection guard before remote command"
    - "composeFile left as empty string in runDryRun() — compose not needed for dry-run; validation deferred to runDeploy()"

key-files:
  created: []
  modified:
    - cmd/docker-deploy/main.go

key-decisions:
  - "RunCompose() writes its own 'Deploy failed: ...' line to stderr; runDeploy() just returns the error — no double-printing"
  - "Basename validation in runDeploy() (not in RunCompose) aligns injection guard with trust boundary T-04-03-01"
  - "context.Background() passed to RunCompose — no separate timeout needed; compose up -d returns quickly (detached)"

patterns-established:
  - "Injection guard pattern: filepath.Base(value) == value check before using user input in remote path concatenation"
  - "Error propagation: inner functions write human-readable failure lines; outer caller propagates error for exit code"

requirements-completed: [DEPLOY-01, DEPLOY-04, DEPLOY-05, DEPLOY-06]

# Metrics
duration: 45min
completed: 2026-05-15
---

# Phase 4 Plan 03: Core Deploy Loop — Wire & Verify Summary

**Full deploy loop wired into main.go: --compose-file flag, basename injection guard, RunCompose() call after Upload(), and 6 end-to-end scenarios verified against a real SSH host**

## Performance

- **Duration:** ~45 min (including human verification round-trip)
- **Started:** 2026-05-15T19:58:49Z
- **Completed:** 2026-05-15
- **Tasks:** 2 (1 auto + 1 human-verify checkpoint)
- **Files modified:** 1

## Accomplishments

- Wired compose.RunCompose() into runDeploy() after Upload(), delivering the complete copy-then-compose cycle
- Registered --compose-file flag on the cobra command with auto-detection fallback
- Added filepath.Base() basename validation (T-04-03-01 injection guard) before remote execution
- Updated Resolve() call to the 8-arg signature from Plan 01; updated runDryRun() to match
- Human verification confirmed all 6 test scenarios pass against a real SSH host including the user-added compose validation test (empty compose file exits code 1, obsolete `version` attribute surfaced as warning)

## Task Commits

Each task was committed atomically:

1. **Task 1: Wire --compose-file flag and updated Resolve() call in main.go** - `88cca8f` (feat)
2. **Task 2: Human verification checkpoint** - approved (no commit — checkpoint only)

## Files Created/Modified

- `cmd/docker-deploy/main.go` - Added --compose-file flag, updated runDeploy() and runDryRun() signatures, updated Resolve() call to 8-arg form, added basename validation, added RunCompose() call after Upload()

## Decisions Made

- RunCompose() writes its own stderr failure line; runDeploy() returns the error without double-printing. Keeps failure messaging inside the compose package boundary.
- context.Background() passed to RunCompose — compose up -d is detached and returns quickly; no separate deadline needed.
- Basename validation placed in runDeploy() (not inside RunCompose) to match where the trust boundary is crossed (T-04-03-01).

## Deviations from Plan

None — plan executed exactly as written.

## Issues Encountered

None. go build, go vet, and go test all passed on first attempt. Human verification passed all 6 tests (5 planned + 1 user-added).

## Human Verification Results

All 6 tests confirmed against real SSH host (192.168.1.99):

| Test | Description | Result |
|------|-------------|--------|
| 1 | Full deploy with streaming output | OK |
| 2 | Compose file auto-detection (compose.yaml) | OK |
| 3 | --compose-file flag override | OK |
| 4 | No compose file found — error before SSH | OK |
| 5 | Exit code on compose failure | OK |
| 6 (user-added) | Compose validation: obsolete `version` attribute surfaced as warning; empty compose file exits code 1 | OK |

Sample output from Tests 5/6:
```
docker deploy --host ssh://sshuser@192.168.1.99
Uploading 4 files...
  -> .env
  -> compose.yaml
[sudo] password for remote host:
WARN[0000] /opt/test-deploy/compose.yaml: the attribute `version` is obsolete, it will be ignored, please remove it to avoid potential confusion
empty compose file
Deploy failed: docker compose exited with code 1
docker compose exited with code 1
```

## User Setup Required

None — no external service configuration required.

## Next Phase Readiness

Phase 4 complete. The full deploy loop (copy + compose) is working end-to-end.

Phase 5 (Pre-flight & Health Polling) can begin immediately:
- Pre-flight checks (docker installed, compose v2, root warning, target writability) run before Upload()
- Health polling runs after RunCompose() returns nil
- No interface changes to RunCompose() or Upload() required for Phase 5

---
*Phase: 04-core-deploy-loop*
*Completed: 2026-05-15*
