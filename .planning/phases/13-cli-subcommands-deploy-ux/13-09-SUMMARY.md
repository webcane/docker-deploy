---
plan: 13-09
phase: 13-cli-subcommands-deploy-ux
status: complete
completed: 2026-05-26
commits:
  - 03c38b5
  - ff05aab
key-files:
  created: []
  modified:
    - internal/filetransfer/upload.go
    - internal/filetransfer/upload_test.go
---

# Plan 13-09: Fix Verbose Double File Listing — Summary

## What Was Built

Replaced per-file `  -> filename` arrow lines in the SFTP upload loop with a single `Uploading N files...` summary line printed before the loop when `verbose=true`. This eliminates the duplicate file listing in verbose repeat-deploy mode where both the arrows and the pre-confirm diff block showed the same local files.

## Changes Made

### `internal/filetransfer/upload.go`
- Added `fmt.Fprintf(os.Stderr, "Uploading %d files...\n", len(files))` before the `uploadErr` closure (single pre-loop summary line)
- Removed the per-file `if verbose { fmt.Fprintf(os.Stderr, "  -> %s\n", relPath) }` block from inside the upload loop
- Pre-confirm diff block (`Local files (N):` section) left untouched — still shows individual filenames before the confirm prompt

### `internal/filetransfer/upload_test.go`
- Renamed `TestUploadVerbose_PerFileStderr` → `TestUploadVerbose_SummaryLine`
- `verbose_true` subtest: now asserts `"Uploading 1 files..."` present and `"  -> "` absent
- `verbose_false` subtest: unchanged (still asserts no `"  -> "` lines)

## Verification

All `TestUploadVerbose_*` tests pass. Binary compiles.

## Deviations

None.

## Self-Check: PASSED
