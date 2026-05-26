---
phase: 13-cli-subcommands-deploy-ux
plan: "07"
subsystem: preflight
tags: [verbose, sudo, diagnostics, tdd]
dependency_graph:
  requires: []
  provides: [sudo-l-verbose-output-in-check-04]
  affects: [internal/preflight/checks.go, internal/preflight/checks_test.go]
tech_stack:
  added: []
  patterns: [best-effort-verbose-diagnostic, tdd-red-green]
key_files:
  created: []
  modified:
    - internal/preflight/checks.go
    - internal/preflight/checks_test.go
decisions:
  - Verbose sudo -l block inserted after id -nG succeeds, before the docker group loop — runs regardless of docker group membership, giving full sudo context whenever id -nG succeeds
  - No else branch on sudoLErr — non-zero or error silently swallowed per D-27
  - cfg.Verbose already threaded through checkDockerGroup via config.Config parameter — zero signature change
metrics:
  duration: 8min
  completed: "2026-05-26T12:43:00Z"
  tasks_completed: 1
  files_changed: 2
---

# Phase 13 Plan 07: Verbose sudo -l Output in CHECK-04 Summary

One-liner: Verbose `sudo -l` diagnostic block in `checkDockerGroup()` — best-effort, failure-silent, gated on `cfg.Verbose`.

## What Was Built

Added a single `if cfg.Verbose` block inside `checkDockerGroup()` in `internal/preflight/checks.go`. When `--verbose` is active, it runs `sudo -l` on the remote immediately after the `id -nG` call succeeds, and prints the policy output to stderr prefixed with `[sudo -l]`. If `sudo -l` fails (non-zero exit, error, or timeout), the failure is silently swallowed — the deploy is never blocked.

## Tasks

### Task 1: Add sudo -l verbose block to checkDockerGroup; add tests (TDD)

**RED:** Added 3 failing tests:
- `TestCheckDockerGroup_SudoL_VerboseShown` — `[sudo -l]` in stderr when Verbose=true and sudo -l succeeds
- `TestCheckDockerGroup_SudoL_FailureSilenced` — no `[sudo -l]` when sudo -l fails
- `TestCheckDockerGroup_SudoL_NotVerbose` — sudo -l never invoked when Verbose=false

Commit: `f20d912`

**GREEN:** Added 7-line `if cfg.Verbose` block to `checkDockerGroup()` after `id -nG` succeeds, before the groups loop. All 3 new tests and all 16 existing preflight tests pass.

Commit: `782520a`

## Verification

- `go test ./internal/preflight/... -run TestCheckDockerGroup` — all pass
- `go test ./...` — full suite exits 0
- `grep -c "sudo -l" internal/preflight/checks.go` — 2 (command string + comment)
- `grep -c "\[sudo -l\]" internal/preflight/checks.go` — 1
- `grep -c "cfg\.Verbose" internal/preflight/checks.go` — 1

## Deviations from Plan

None — plan executed exactly as written.

## Known Stubs

None.

## Threat Flags

No new security surface introduced. The `[sudo -l]` output path was already analyzed in the plan's threat model (T-13-07-01 through T-13-07-03) and accepted.

## Self-Check: PASSED

- `internal/preflight/checks.go` — exists and contains `[sudo -l]` block
- `internal/preflight/checks_test.go` — exists and contains 3 new test functions
- Commit `f20d912` — RED phase test commit
- Commit `782520a` — GREEN phase implementation commit
- Full test suite exits 0
