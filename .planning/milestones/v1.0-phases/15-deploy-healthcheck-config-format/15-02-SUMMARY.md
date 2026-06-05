---
phase: 15-deploy-healthcheck-config-format
plan: "02"
subsystem: health
tags: [healthcheck, retries, per-container, poll, duration]
dependency_graph:
  requires: [15-01]
  provides: [pollContainers retries semantics, per-container failCount, integration/compose_test.go migration]
  affects: [internal/health/poll.go, internal/health/poll_test.go, integration/compose_test.go]
tech_stack:
  added: []
  patterns: [per-container failCount map, reset-on-healthy counter, retries==0 backward compat, TDD RED/GREEN]
key_files:
  created: []
  modified:
    - internal/health/poll.go
    - internal/health/poll_test.go
    - integration/compose_test.go
decisions:
  - "retries==0 preserves existing immediate-fail behaviour (backward compat); retries>0 activates deferred-fail gate"
  - "failCount is per-container (D-10); one container hitting threshold does not reset others"
  - "healthy/no-healthcheck resets failCount[container]=0 (D-09: single healthy resets consecutive counter)"
  - "Timeout error message uses cfg.Healthcheck.Timeout.String() via %s format (Duration.String() e.g. '30s')"
  - "No direct pollContainers call sites existed in poll_test.go — all tests use pollHealthWithRunner"
metrics:
  duration: "~10 minutes"
  completed: "2026-05-30"
  tasks: 2
  files_modified: 3
---

# Phase 15 Plan 02: Healthcheck Retries Semantics Summary

Per-container consecutive-unhealthy retries semantics added to `pollContainers` via `failCount map[string]int`; `retries==0` preserves existing immediate-fail behaviour; `integration/compose_test.go` migrated from removed flat fields to new `config.HealthcheckConfig` struct.

## Tasks Completed

| # | Task | Commit | Files |
|---|------|--------|-------|
| RED | Add failing retries-semantics tests | 4ae6640 | internal/health/poll_test.go |
| 1 | Update pollHealthWithRunner / pollContainers | 8bc994b | internal/health/poll.go |
| 2 | Migrate tests; add retries tests; integrate compose_test | e3ba150 | internal/health/poll_test.go, integration/compose_test.go |

## What Was Built

- `pollContainers` signature extended: `func pollContainers(runner sessionOpener, containers []string, done map[string]bool, failCount map[string]int, retries int) (bool, error)`
- `failCount := make(map[string]int, len(containers))` declared alongside `done` map in `pollHealthWithRunner`
- `failCount` passed into every `pollContainers` call site along with `cfg.Healthcheck.Retries`
- Retries gate: when `retries > 0`, increment `failCount[container]` on unhealthy; fail when `failCount >= retries`; print `"(%d consecutive unhealthy results)"` in error message
- Reset: on `"healthy"` or `"no-healthcheck"`, `failCount[container] = 0` (D-09)
- Timeout message: `"Health check timed out after %s: container %s is not yet running\n"` using `cfg.Healthcheck.Timeout` (Duration.String() via `%s`)
- Three new tests: `TestPollHealth_RetriesThresholdReached`, `TestPollHealth_RetriesResetOnHealthy`, `TestPollHealth_NoRetries_ImmediateFail`
- `integration/compose_test.go` migrated: 3 call sites from `{HealthTimeout: N, HealthInterval: M}` to `{Healthcheck: config.HealthcheckConfig{Timeout: N*time.Second, Interval: M*time.Second}}`; `"time"` import added

## Deviations from Plan

None — plan executed exactly as written. The existing `poll_test.go` already used `Healthcheck` config struct (fixed as a blocking deviation in 15-01), so Sweep 1 only required updating the package doc comment.

## Verification

- `go build ./...` exits 0
- `go test ./internal/health/...` exits 0
- `go test ./...` exits 0 (full suite)
- `grep -rn "HealthTimeout\|HealthInterval" internal/ cmd/` returns no matches

## Known Stubs

None.

## Threat Flags

No new network endpoints, auth paths, or trust boundaries introduced. All four STRIDE threats from the plan's threat model are mitigated:
- T-15-02-01: Zero-value guards preserved (Interval<=0 → 1ms, Timeout<=0 → 1s)
- T-15-02-02: retries==0 retains immediate-fail semantics
- T-15-02-03: accepted (container names from trusted docker ps on remote)
- T-15-02-04: timeout timer always fires regardless of retries setting

## Self-Check: PASSED

- internal/health/poll.go exists and contains `failCount` ✓
- integration/compose_test.go exists and contains `config.HealthcheckConfig{` ✓
- Commits 4ae6640, 8bc994b, e3ba150 exist ✓
- `go test ./...` all pass ✓
