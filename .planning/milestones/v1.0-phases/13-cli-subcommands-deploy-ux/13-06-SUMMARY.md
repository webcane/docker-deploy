---
phase: 13-cli-subcommands-deploy-ux
plan: "06"
subsystem: filetransfer
tags: [sudo, path-detection, upload, tdd]
dependency_graph:
  requires: [13-04]
  provides: [needsSudo probe in Upload()]
  affects: [internal/filetransfer/upload.go, internal/filetransfer/upload_test.go]
tech_stack:
  added: []
  patterns: [needsSudo probe, execCmd closure dispatch, path.Dir for Linux remotes]
key_files:
  created: []
  modified:
    - internal/filetransfer/upload.go
    - internal/filetransfer/upload_test.go
decisions:
  - "needsSudo probe uses 'test -w remoteBase || test -w path.Dir(remoteBase)' — OR handles first-deploy case where path doesn't exist yet"
  - "execCmd closure dispatches to SudoExec or sshRun(nil) for all 8 remoteBase operations; /tmp staging cleanup sshRun is unchanged"
  - "path.Dir used (not filepath.Dir) — remote is always Linux"
  - "ShellQuote applied to both paths in OR probe command (T-13-06-01)"
metrics:
  duration: 8 min
  completed: "2026-05-26T12:57:00Z"
---

# Phase 13 Plan 06: Path-Aware Sudo Detection Summary

Path-aware sudo probe in Upload() using `test -w` OR expression; writable paths bypass all SudoExec and use direct sshRun(nil) for all remoteBase operations.

## What Was Built

Added a `needsSudo` probe at the start of `Upload()` (after SFTP opens, before staging dir creation). The probe runs `test -w remoteBase || test -w path.Dir(remoteBase)` via `sshRun(client, probeCmd, nil)`. If the shell exits 0 (path or parent is writable), `needsSudo=false`; if exit 1, `needsSudo=true`.

An `execCmd` closure dispatches all 8 remoteBase operations:
- `needsSudo=false`: direct `sshRun(client, cmd, nil)` — no password, no sudo scaffolding
- `needsSudo=true`: full `SudoExec(client, cmd, creds, warnedOnce, verbose)` fallback chain

The `/tmp` staging dir cleanup always uses direct `sshRun` (unchanged) — `/tmp` is world-writable.

## Tasks

| Task | Name | Commit | Files |
|------|------|--------|-------|
| 1 (RED) | Add failing tests for path-aware sudo detection | b9da652 | upload_test.go |
| 1 (GREEN) | Add needsSudo probe; dispatch SudoExec vs sshRun | b8f619c | upload.go, upload_test.go |

## Verification

- `go build ./...` exits 0
- `go test ./...` exits 0 (all packages pass)
- `grep -c "needsSudo" upload.go` returns 6
- `grep -c "test -w" upload.go` returns 2
- `grep -c "path\.Dir" upload.go` returns 3 (probe + existing remote path ops)
- `grep -c "filepath\.Dir" upload.go` returns 1 (comment only — no functional use)

## Deviations from Plan

### Auto-fixed Issues

**1. [Rule 1 - Bug] Test condition order in ElevatedPath mock**
- **Found during:** GREEN phase testing
- **Issue:** `cmdExitCode` in `TestUpload_PathAwareSudo_ElevatedPath` checked `mkdir/mv/rm -rf` before `sudo -n`. Since `sudo -n sh -c 'mkdir -p ...'` contains both "sudo -n" and "mkdir", the mkdir condition matched first and returned exit 1 — causing Upload() to fail with "could not create target directory"
- **Fix:** Reordered conditions: `sudo -n` checked before bare `mkdir/mv/rm` so NOPASSWD path succeeds
- **Files modified:** upload_test.go
- **Commit:** b8f619c (included in GREEN commit)

## Known Stubs

None.

## Threat Flags

No new threat surface beyond the T-13-06-01/02/03 entries in the plan's threat model. ShellQuote is applied to both probe paths; needsSudo=false correctly skips sudo for user-writable paths (accepted per T-13-06-02).

## Self-Check: PASSED

- `internal/filetransfer/upload.go` — exists with needsSudo probe
- `internal/filetransfer/upload_test.go` — exists with 3 new PathAwareSudo tests
- Commit b9da652 — RED phase test commit
- Commit b8f619c — GREEN phase implementation commit
- `go test ./internal/filetransfer/... -run TestUpload_PathAwareSudo` — all 3 PASS
