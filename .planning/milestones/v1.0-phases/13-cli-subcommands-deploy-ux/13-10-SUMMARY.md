---
phase: 13-cli-subcommands-deploy-ux
plan: 10
subsystem: filetransfer
tags: [bug-fix, sudo-detection, probe-logic, regression-test]
dependency_graph:
  requires: [13-09]
  provides: [correct-needsSudo-probe]
  affects: [internal/filetransfer/upload.go, internal/filetransfer/upload_test.go]
tech_stack:
  added: []
  patterns: [parent-only writability probe, test -w parent]
key_files:
  modified:
    - internal/filetransfer/upload.go
    - internal/filetransfer/upload_test.go
decisions:
  - "Probe only the parent directory for writability — mkdir/mv/rm require parent write perms, not target dir perms"
  - "Single test -w parent probe replaces OR probe to prevent false-negative on user-owned /opt/myapp"
metrics:
  duration: "~10 minutes"
  completed: "2026-05-26"
  tasks_completed: 2
  files_modified: 2
---

# Phase 13 Plan 10: Fix needsSudo Parent-Only Probe Summary

**One-liner:** Fix false-negative sudo detection by probing only the parent directory — `test -w parent` instead of `test -w target || test -w parent`.

## What Was Built

Fixed the `needsSudo` probe in `Upload()` that incorrectly returned `false` for paths like `/opt/test-deploy` where the user owns the target directory but cannot write to the parent (`/opt`).

The root cause: `test -w /opt/test-deploy || test -w /opt` — when the user owns `/opt/test-deploy`, the first operand exits 0 and the OR short-circuits to `needsSudo=false`. But `mv /opt/test-deploy /opt/test-deploy-old-XXXXX` renames an entry WITHIN `/opt`, requiring write permission on `/opt` (the parent), not on `/opt/test-deploy` itself.

**Fix:** Probe ONLY the parent directory: `test -w path.Dir(remoteBase)`. Both first-deploy (`mkdir` creates an entry in parent) and repeat-deploy (`mv` renames an entry in parent) require the parent to be writable.

## Tasks Completed

| Task | Description | Commit |
|------|-------------|--------|
| 1 | Change probeCmd to parent-only in upload.go | 82b0a10 |
| 2 | Update existing probe tests and add regression test | 1fe9fd4 |

## Changes

### internal/filetransfer/upload.go

- Replaced `fmt.Sprintf("test -w %s || test -w %s", ShellQuote(remoteBase), ShellQuote(path.Dir(remoteBase)))` with `fmt.Sprintf("test -w %s", ShellQuote(path.Dir(remoteBase)))`
- Updated comment block at Step 3b to explain the parent-writability rationale

### internal/filetransfer/upload_test.go

- `TestUpload_PathAwareSudo_WritablePath`: updated comment to reflect parent probe semantics
- `TestUpload_PathAwareSudo_ParentWritable`: replaced OR-clause assertion with single parent-probe assertion (no `||` in probe)
- Added `TestUpload_PathAwareSudo_OwnsTargetButParentElevated`: regression test for UAT failure — user owns `/opt/test-deploy` but `/opt` is root-owned → must use sudo

## Verification

```
go test ./internal/filetransfer/... -run TestUpload_PathAwareSudo -v
```

All four tests pass:
- `TestUpload_PathAwareSudo_WritablePath` — PASS
- `TestUpload_PathAwareSudo_ElevatedPath` — PASS
- `TestUpload_PathAwareSudo_ParentWritable` — PASS (updated)
- `TestUpload_PathAwareSudo_OwnsTargetButParentElevated` — PASS (new)

`go test ./...` exits 0.

## Deviations from Plan

None — plan executed exactly as written.

## Threat Flags

None — no new network endpoints, auth paths, file access patterns, or schema changes introduced. The fix makes the probe MORE conservative (parent-only check is stricter — more likely to trigger sudo when uncertain), which is the safe direction per T-13-10-01.

## Self-Check: PASSED

- [x] internal/filetransfer/upload.go modified — confirmed
- [x] internal/filetransfer/upload_test.go modified — confirmed
- [x] Commit 82b0a10 exists — fix(13-10): change needsSudo probe to parent-only
- [x] Commit 1fe9fd4 exists — test(13-10): update PathAwareSudo tests for parent-only probe
- [x] All four TestUpload_PathAwareSudo_* tests pass
- [x] go test ./... exits 0
