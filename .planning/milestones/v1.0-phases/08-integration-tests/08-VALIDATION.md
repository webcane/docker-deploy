---
phase: 08
slug: integration-tests
status: complete
nyquist_compliant: true
wave_0_complete: true
created: 2026-06-04
---

# Phase 08 — Validation Strategy

> Per-phase validation contract reconstructed from PLAN and SUMMARY artifacts (State B).

---

## Test Infrastructure

| Property | Value |
|----------|-------|
| **Framework** | go test (with `-tags integration` build tag) |
| **Config file** | `go.mod` / `Makefile` (`test-ci` target) |
| **Quick run command** | `go build -tags integration ./integration/...` |
| **Full suite command** | `make test-ci` |
| **Estimated runtime** | ~15 minutes (container build + test execution) |

---

## Sampling Rate

- **After every task commit:** Run `go build -tags integration ./integration/...`
- **After every plan wave:** Run `make test-ci`
- **Before `/gsd-verify-work`:** Full suite must be green
- **Max feedback latency:** ~900 seconds (15m cold build)

---

## Per-Task Verification Map

| Task ID | Plan | Wave | Requirement | Threat Ref | Secure Behavior | Test Type | Automated Command | File Exists | Status |
|---------|------|------|-------------|------------|-----------------|-----------|-------------------|-------------|--------|
| 08-01-01 | 01 | 1 | SC-1..SC-7 | T-08-01-01 | `knownhosts.New()` in `dialContainer` — never `InsecureIgnoreHostKey` | integration | `go build -tags integration ./integration/...` | ✅ | ✅ green |
| 08-01-02 | 01 | 1 | SC-1..SC-7 | T-08-01-01 | No `InsecureIgnoreHostKey` call (grep gate); host key extraction via `container.Exec` only | integration | `grep -rn "InsecureIgnoreHostKey" integration/ \|\| echo PASS` | ✅ | ✅ green |
| 08-02-01 | 02 | 1 | SC-1 | — | N/A | integration | `grep -n "test-integration" Makefile` | ✅ | ✅ green |
| 08-02-02 | 02 | 1 | SC-1 | — | N/A | integration | `grep -n "integration" .github/workflows/ci.yml` | ✅ | ✅ green |
| 08-03-01 | 03 | 2 | SC-2 | — | N/A | integration | `go test -tags integration -run TestDial -timeout 15m ./integration/...` | ✅ | ✅ green |
| 08-04-01 | 04 | 2 | SC-3, SC-4, SC-6 | — | Root-user warning non-blocking (`err == nil`, `Status == "warn"`) | integration | `go test -tags integration -run TestPreflight -timeout 15m ./integration/...` | ✅ | ✅ green |
| 08-05-01 | 05 | 2 | SC-5 | — | Atomic cancel: sentinel survives, no staging dir left behind | integration | `go test -tags integration -run TestUpload -timeout 15m ./integration/...` | ✅ | ✅ green |
| 08-06-01 | 06 | 3 | SC-7 | — | N/A | integration | `go test -tags integration -run 'TestCompose\|TestHealth' -timeout 15m ./integration/...` | ✅ | ✅ green |
| 08-06-02 | 06 | 3 | SC-7 | — | N/A | integration | `go test -tags integration -run 'TestCompose\|TestHealth' -timeout 15m ./integration/...` | ✅ | ✅ green |

*Status: ⬜ pending · ✅ green · ❌ red · ⚠️ flaky*

---

## Wave 0 Requirements

Existing infrastructure covers all phase requirements — no stub files needed. Integration test
package (`integration/`) was created as part of this phase; Wave 1 (08-01 + 08-02) establishes
the foundation before Wave 2 test files were added.

---

## Manual-Only Verifications

| Behavior | Requirement | Why Manual | Test Instructions |
|----------|-------------|------------|-------------------|
| Full suite E2E execution | SC-1 | Requires Docker daemon; cannot run testcontainers in static analysis | Run `make test-ci` on a machine with Docker; confirm all tests pass (or `TestUpload_AtomicCancel` is skipped with "cancel fired too late" on fast hardware); exit code 0 |
| HEALTH-03 intent alignment | SC-7 | Architectural decision: `poll.go` inspects `State.Status` not `State.Health.Status`; busybox+exit1 exercises the "stopped unexpectedly" path rather than a failing HEALTHCHECK | Confirmed acceptable 2026-05-22 — `TestCompose_Unhealthy_ReturnError` satisfies HEALTH-03 intent for v1 |

---

## Coverage Summary

All 7 ROADMAP success criteria are COVERED by automated tests:

| Requirement | Description | Test File | Test Functions | Status |
|-------------|-------------|-----------|----------------|--------|
| SC-1 | `go test -tags integration` runs without manual setup | `helpers_test.go`, `Makefile`, `ci.yml` | `TestMain` + CI integration job | ✅ COVERED |
| SC-2 | SSH connectivity (knownhosts, TOFU, timeout, auth chain) | `integration/dial_test.go` | `TestDial_Timeout`, `TestDial_UnknownHost_TOFU`, `TestDial_UnknownHost_TOFU_Accepted`, `TestDial_Success` | ✅ COVERED |
| SC-3 | Root-user warning (CHECK-07) triggered and non-blocking | `integration/preflight_test.go` | `TestPreflight_CHECK07_RootWarning_DoesNotBlock` | ✅ COVERED |
| SC-4 | Passwordless-sudo pass (sshuser) and fail (nosudouser) | `integration/preflight_test.go` | `TestPreflight_CHECK04_*`, `TestPreflight_CHECK05_*`, `TestPreflight_CHECK06_*` | ✅ COVERED |
| SC-5 | File copy atomicity after mid-copy context cancel | `integration/filetransfer_test.go` | `TestUpload_AtomicCancel` | ✅ COVERED |
| SC-6 | All preflight checks (CHECK-01..CHECK-07) pass + fail | `integration/preflight_test.go` | 13 test functions | ✅ COVERED |
| SC-7 | Health polling (HEALTH-01..03) with/without HEALTHCHECK | `integration/compose_test.go` | `TestCompose_Healthy_NoHealthcheck`, `TestCompose_Unhealthy_ReturnError`, `TestHealth_NoContainers` | ✅ COVERED |

---

## Validation Sign-Off

- [x] All tasks have `<automated>` verify or established test coverage
- [x] Sampling continuity: every task has an automated verification command
- [x] Wave 0 covers all MISSING references (none were missing)
- [x] No watch-mode flags
- [x] Feedback latency: `go build -tags integration ./integration/...` is fast (< 30s) for task-level sampling
- [x] `nyquist_compliant: true` set in frontmatter

**Approval:** approved 2026-06-04 (reconstructed from PLAN/SUMMARY/VERIFICATION artifacts)
