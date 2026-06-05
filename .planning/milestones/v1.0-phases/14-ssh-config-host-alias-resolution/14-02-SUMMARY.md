---
phase: 14-ssh-config-host-alias-resolution
plan: "02"
subsystem: config
tags: [error-messages, loadfile, config, ux]
dependency_graph:
  requires: [14-01]
  provides: [context-aware-no-host-errors, loadfile-bool-signature, NoHostError-helper]
  affects: [cmd/docker-deploy/main.go, internal/config/config.go, internal/config/config_test.go]
tech_stack:
  added: []
  patterns: [three-return-value-signature, exported-error-helper, context-aware-errors]
key_files:
  created: []
  modified:
    - internal/config/config.go
    - internal/config/config_test.go
    - cmd/docker-deploy/main.go
decisions:
  - "LoadFile returns (FileConfig, bool, error) where bool=fileExisted — callers branch on this to emit context-specific errors"
  - "Non-NotExist read errors (e.g. permission denied) return fileExisted=false — conservative: emits not-found message variant"
  - "NoHostError is exported from config package so message strings are defined in one place and testable without main package"
  - "runValidate uses blank identifier for fileExisted — it already does its own os.Stat file-present check"
metrics:
  duration: "~8 minutes"
  completed: "2026-05-29T05:49:55Z"
  tasks_completed: 2
  files_modified: 3
---

# Phase 14 Plan 02: Deploy.yaml Error Message Improvements Summary

Context-aware "no host" error messages: LoadFile now returns a fileExisted bool so callers distinguish missing deploy.yaml from present-but-unconfigured deploy.yaml, producing two distinct user-facing errors.

## Tasks Completed

| Task | Name | Commit | Files |
|------|------|--------|-------|
| 1 | Extend LoadFile to return fileExisted bool; add unit tests | f3933e8 | internal/config/config.go, internal/config/config_test.go |
| 2 | Update all LoadFile call sites in main.go | 7e1891c | cmd/docker-deploy/main.go |

## Changes Made

### internal/config/config.go

- Changed `LoadFile(dir string) (FileConfig, error)` to `LoadFile(dir string) (FileConfig, bool, error)` where the bool reports whether deploy.yaml was found on disk.
- Three return paths: file absent → `(FileConfig{}, false, nil)`; valid YAML → `(populated, true, nil)`; malformed YAML → `(FileConfig{}, true, error)`; non-NotExist read error → `(FileConfig{}, false, error)`.
- Added exported `NoHostError(fileExisted bool, dir string) error` helper that returns one of two messages:
  - `fileExisted=false`: `"no deploy.yaml found in <dir> and no --host flag provided"` (D-14a)
  - `fileExisted=true`: `"deploy.yaml: target.host is not set"` (D-14b)

### internal/config/config_test.go

- Updated all existing `LoadFile` call sites to the new three-value signature (blank identifier for bool where not needed).
- Added `TestLoadFile_Absent`: verifies fileExisted=false and no error for empty directory.
- Added `TestLoadFile_Present`: verifies fileExisted=true and populated FileConfig for valid deploy.yaml.
- Added `TestLoadFile_Malformed`: verifies fileExisted=true and error for malformed YAML.
- Added `TestNoHostError_FileAbsent`: asserts exact error string for missing-file path.
- Added `TestNoHostError_FilePresent`: asserts exact error string for present-but-unconfigured path.

### cmd/docker-deploy/main.go

- `runDeploy()`: updated to `fileConfig, fileExisted, err := config.LoadFile(cwd)`; "no host" check now calls `config.NoHostError(fileExisted, cwd)`.
- `runDryRun()`: same pattern as runDeploy.
- `runValidate()`: updated to `fileConfig, _, err := config.LoadFile(cwd)` — blank identifier for fileExisted since runValidate already uses `os.Stat` to detect file presence separately.

## Deviations from Plan

None — plan executed exactly as written.

## Known Stubs

None.

## Threat Flags

None. Both error paths return before any SSH connection attempt (T-14-02-03 mitigation confirmed). Error strings include the local cwd path, which is acceptable per T-14-02-01.

## Self-Check: PASSED

Files exist:
- internal/config/config.go — modified, contains `NoHostError` and updated `LoadFile`
- internal/config/config_test.go — modified, contains 5 new tests
- cmd/docker-deploy/main.go — modified, all 3 LoadFile call sites updated

Commits exist:
- f3933e8 — feat(14-02): extend LoadFile to return fileExisted bool; add NoHostError helper
- 7e1891c — feat(14-02): update LoadFile call sites; replace generic "no host" error with context-aware messages

Test results: `go test ./...` exits 0 across all packages.
