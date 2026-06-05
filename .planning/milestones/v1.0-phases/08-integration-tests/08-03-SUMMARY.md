---
phase: 08-integration-tests
plan: 03
subsystem: testing
tags: [ssh, knownhosts, tofu, testcontainers, integration-tests]

# Dependency graph
requires:
  - phase: 08-01
    provides: "helpers_test.go with TestMain, sshA container (Container A), and helper functions: emptyKnownHosts, seedKnownHosts, generateTestKeyFile"
  - phase: 02-ssh-transport-config
    provides: "internalssh.Dial() and DialConfig with TOFU, timeout, and knownhosts verification"
provides:
  - "integration/dial_test.go with four TestDial_* integration tests covering SC-2 (SSH connectivity)"
affects: [08-04, 08-05, 08-06]

# Tech tracking
tech-stack:
  added: []
  patterns: ["shared container variable pattern (sshA from TestMain instead of per-test container startup)"]

key-files:
  created:
    - integration/dial_test.go
  modified: []

key-decisions:
  - "Port tests use sshA package-level variable from TestMain (not per-test container start) — faster and consistent with integration test suite design"
  - "TestDial_Success accepts auth failure as the expected path — no key injected into container, host-key verification success is what matters"
  - "TestDial_Timeout targets 192.0.2.1 (TEST-NET, RFC 5737) with 500ms timeout — no container needed for this test"

patterns-established:
  - "Integration dial tests: use sshA.host, sshA.port, sshA.hostKey — never startSSHContainer(t) inside individual tests"
  - "No InsecureIgnoreHostKey anywhere — all tests use emptyKnownHosts or seedKnownHosts per CLAUDE.md Rule 1"

requirements-completed: [SC-2]

# Metrics
duration: 5min
completed: 2026-05-21
---

# Phase 08 Plan 03: SSH Connectivity Integration Tests Summary

**Four TestDial_* integration tests using shared Container A (sshA): timeout against RFC 5737 TEST-NET, TOFU rejection, TOFU acceptance writes to known_hosts, and pre-seeded host-key verification**

## Performance

- **Duration:** ~5 min
- **Started:** 2026-05-21T00:00:00Z
- **Completed:** 2026-05-21T00:05:00Z
- **Tasks:** 1
- **Files modified:** 1

## Accomplishments
- Created `integration/dial_test.go` with four TestDial_* tests covering SC-2 (SSH connectivity)
- Ported tests from `internal/ssh/client_test.go` into the integration package, replacing per-test container startup with the shared `sshA` variable from TestMain
- Verified `go build -tags integration ./integration/...` exits 0 with no errors

## Task Commits

1. **Task 1: Create integration/dial_test.go** - `1cc8616` (feat)

**Plan metadata:** (docs commit — see below)

## Files Created/Modified
- `integration/dial_test.go` - Four SSH connectivity integration tests (TestDial_Timeout, TestDial_UnknownHost_TOFU, TestDial_UnknownHost_TOFU_Accepted, TestDial_Success)

## Decisions Made
- Tests use `sshA` package-level variable (started in TestMain) instead of `startSSHContainer(t)` inside each test, avoiding redundant container startup and aligning with the integration suite design
- `TestDial_Success` treats auth failure as acceptable — the container has no authorized key for the generated RSA key, so the test verifies host-key verification succeeds without checking auth

## Deviations from Plan

None - plan executed exactly as written.

## Issues Encountered

None.

## User Setup Required

None - no external service configuration required.

## Next Phase Readiness
- `integration/dial_test.go` is complete and compiles; ready for integration with the full test run (08-05 CI/Makefile plan)
- Plans 08-04 and later can reference dial_test.go as a pattern for container-using integration tests

---
*Phase: 08-integration-tests*
*Completed: 2026-05-21*

## Self-Check: PASSED
- `integration/dial_test.go` exists and confirmed via build
- Commit `1cc8616` present in git log
- Four test functions verified: TestDial_Timeout, TestDial_UnknownHost_TOFU, TestDial_UnknownHost_TOFU_Accepted, TestDial_Success
- No InsecureIgnoreHostKey
- No startSSHContainer calls inside test functions
- sshA.host, sshA.port, sshA.hostKey used in 3 of 4 tests (TestDial_Timeout does not need container)
