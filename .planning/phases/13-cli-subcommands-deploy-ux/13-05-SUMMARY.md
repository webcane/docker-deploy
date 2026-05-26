---
phase: 13-cli-subcommands-deploy-ux
plan: 05
subsystem: filetransfer
tags: [verbose, confirm-prompt, sftp-readdir, force-flag, tdd]
dependency_graph:
  requires: [13-04]
  provides: [verbose-pre-confirm-diff, force-upload-param]
  affects: [cmd/docker-deploy/main.go, internal/filetransfer/upload.go]
tech_stack:
  added: []
  patterns: [sftp.ReadDir for remote file listing, bufio.NewReader confirm prompt inside Upload()]
key_files:
  created: []
  modified:
    - internal/filetransfer/upload.go
    - internal/filetransfer/upload_test.go
    - cmd/docker-deploy/main.go
decisions:
  - Upload() now owns both the confirm prompt and verbose diff — main.go just passes force bool
  - Verbose diff uses sftpClient (already open at step 3) for ReadDir; no extra SSH connection
  - First deploy (existsBefore=false) shows verbose diff but no prompt
  - Existing repeat-deploy tests updated to force=true (they test mechanics, not the prompt)
metrics:
  duration: 12min
  completed: "2026-05-26"
  tasks_completed: 1
  files_modified: 3
---

# Phase 13 Plan 05: Verbose Pre-Confirm Diff in Upload() Summary

**One-liner:** Moved confirm prompt into Upload() and added verbose pre-confirm diff showing local+remote file lists (truncated at 20) before the Replace-all-contents? question.

## Tasks Completed

| Task | Name | Commit | Files |
|------|------|--------|-------|
| 1 (RED) | Add failing tests for verbose pre-confirm diff | e32e43b | upload_test.go |
| 1 (GREEN) | Implement force param + verbose diff + move prompt | 5f568d7 | upload.go, upload_test.go, main.go |

## What Was Built

### Upload() signature change

Before (Plan 13-04):
```go
func Upload(ctx, client, localDir, remoteBase, excludes, creds *SudoCreds, warnedOnce *bool, verbose bool) (int, error)
```

After (Plan 13-05):
```go
func Upload(ctx, client, localDir, remoteBase, excludes, creds *SudoCreds, force bool, warnedOnce *bool, verbose bool) (int, error)
```

### Verbose diff block (inside Upload(), before confirm prompt)

When `verbose=true` and `!force` and `existsBefore`:
- Prints `Local files (N):` + filenames from WalkFiles result, truncated at 20
- Calls `sftpClient.ReadDir(remoteBase)` for remote listing; prints `Remote files (M):` + entries, or `Remote files: (unable to list: err)` on failure
- Then prints the `Replace all contents? [y/N]` prompt

When `verbose=true` and `!existsBefore` (first deploy):
- Prints `Local files (N):` + truncated list
- Prints `Remote files: (none)`
- No prompt (first deploy never prompts)

### Confirm prompt moved from main.go to Upload()

main.go lines 391–425 (the `if !resolved.Force` block with SSH session + bufio prompt) removed.
Upload() now contains identical logic tied directly to `force bool` and `existsBefore`.
`bufio` import removed from main.go.

## Tests Added

- `TestUploadVerbose_PreConfirmDiff`: verbose+force=false+remote-exists → stderr has Local files + Remote files
- `TestUpload_ForceSkipsPrompt`: force=true → no prompt in stderr
- `TestUploadVerbose_FirstDeploy_NoRemote`: verbose+first deploy → stderr has "Remote files: (none)"
- `TestUploadVerbose_Truncation`: 25 local files → stderr has "... and N more"

## Deviations from Plan

### Auto-fixed Issues

**1. [Rule 1 - Bug] Existing repeat-deploy tests would block on the new prompt**
- **Found during:** GREEN phase — updating existing test call sites to 9-arg signature
- **Issue:** `TestUploadRepeatDeploy_ThreeStepSwapUnchanged` and the 3 `TestUploadSkipEnvPreservesRemoteEnv` sub-tests use `newMockSSHServer([]string{remoteBase})` (repeat deploy). With `force=false` (their prior value), the new confirm prompt would block waiting for stdin.
- **Fix:** Updated those 4 tests to use `force=true`. They test atomic-swap mechanics and .env preservation, not the prompt behavior. New dedicated tests cover the prompt.
- **Files modified:** internal/filetransfer/upload_test.go
- **Commit:** 5f568d7

## Verification Results

- `go build ./...` exits 0
- `go test ./...` exits 0 (all packages, full suite)
- `grep -c "sftpClient\.ReadDir" internal/filetransfer/upload.go` returns 1
- `grep -c "Replace all contents" cmd/docker-deploy/main.go` returns 0
- `grep -c "Local files" internal/filetransfer/upload.go` returns 2

## Known Stubs

None.

## Threat Flags

None — the pre-confirm diff only prints file names (metadata), no file contents or credentials. Truncation at 20 prevents excessive stderr output (T-13-05-03 mitigated).

## Self-Check: PASSED

Files exist:
- internal/filetransfer/upload.go: FOUND
- internal/filetransfer/upload_test.go: FOUND
- cmd/docker-deploy/main.go: FOUND

Commits exist:
- e32e43b (RED test commit): FOUND
- 5f568d7 (GREEN implementation commit): FOUND
