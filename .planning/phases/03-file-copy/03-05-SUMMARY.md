---
phase: 03-file-copy
plan: "05"
subsystem: filetransfer
tags: [bug-fix, upload, atomic-swap, tdd]
dependency_graph:
  requires: [03-04]
  provides: [first-deploy-correct-file-placement]
  affects: [internal/filetransfer/upload.go]
tech_stack:
  added: []
  patterns: [in-process-ssh-mock, sftp-InMemHandler, tdd-red-green]
key_files:
  created:
    - internal/filetransfer/upload_test.go
  modified:
    - internal/filetransfer/upload.go
decisions:
  - "Use in-process SSH server (golang.org/x/crypto/ssh server API) + pkg/sftp InMemHandler for Upload unit tests — avoids testcontainers dependency for this logic-level test"
  - "Insert rm -rf only in the else branch (first-deploy) — repeat-deploy (existsBefore=true) three-step swap is unchanged"
  - "rm failure returns a distinct error 'removing target placeholder before first deploy' to aid diagnosis"
metrics:
  duration: "~8 min"
  completed: "2026-05-15"
  tasks_completed: 1
  files_changed: 2
---

# Phase 03 Plan 05: First-Deploy MV Nesting Bug Fix Summary

Fixed the first-deploy directory nesting bug in Upload(): `rm -rf remoteBase` inserted before `mv stagingDir remoteBase` in the else branch so mv performs a clean rename rather than nesting inside the placeholder directory.

## What Was Built

Single targeted fix in `internal/filetransfer/upload.go` — the `else` branch of the `existsBefore` check now runs two sequential `sudoRun` calls:

1. `rm -rf remoteBase` — removes the empty placeholder directory that `mkdir -p` (step 8) created
2. `mv stagingDir remoteBase` — clean rename succeeds because destination no longer exists

The `existsBefore = true` (repeat-deploy) path and its three-step atomic swap are unchanged.

## Root Cause

`mkdir -p remoteBase` in step 8 succeeds unconditionally — it creates `remoteBase` as an empty directory even on first deploy. When step 9 then runs `mv stagingDir remoteBase`, Unix `mv` detects that the destination exists and moves `stagingDir` *inside* it rather than renaming it. Files landed at `remoteBase/<staging-name>/` instead of `remoteBase/`.

## TDD Gate Compliance

- **RED commit:** `e34465b` — `test(03-05): add failing regression test for first-deploy mv nesting bug`
  - `TestUploadFirstDeploy_RmBeforeMv` failed (expected — rm -rf not yet present)
  - `TestUploadRepeatDeploy_ThreeStepSwapUnchanged` passed (existing three-step swap verified)
- **GREEN commit:** `d1a3407` — `fix(03): first-deploy mv — rm target placeholder before rename`
  - Both tests pass
  - All pre-existing filetransfer tests pass

## Test Infrastructure

New `upload_test.go` introduces an in-process SSH server using `golang.org/x/crypto/ssh` server APIs. Each test session type is handled:
- `exec` sessions: records commands, returns "exists"/"absent" for `test -d` queries, exits 0
- `sftp` subsystem: served by `pkg/sftp`'s `NewRequestServer` + `InMemHandler()` for file uploads

This allows `Upload()` to be tested end-to-end without Docker or network access.

## Deviations from Plan

None — plan executed exactly as written. The test approach (in-process SSH mock vs. testcontainers) was left to the executor's judgement per the plan's guidance; in-process was chosen as it requires no external infrastructure.

## Threat Model

T-03-05-01 (Tampering — sudoRun rm -rf remoteBase): accepted. `remoteBase` is validated via `Resolve()`/`ParseHost` and wrapped in `ShellQuote()` — same trust level as the existing `mkdir -p` and `mv` calls in the same function.

## Self-Check: PASSED

- internal/filetransfer/upload.go: FOUND
- internal/filetransfer/upload_test.go: FOUND
- .planning/phases/03-file-copy/03-05-SUMMARY.md: FOUND
- Commit e34465b (RED test): FOUND
- Commit d1a3407 (GREEN fix): FOUND
- upload.go line 207 contains rm -rf ShellQuote(remoteBase): VERIFIED
