---
phase: 13-cli-subcommands-deploy-ux
fixed_at: 2026-05-26T00:00:00Z
review_path: .planning/phases/13-cli-subcommands-deploy-ux/13-REVIEW.md
iteration: 1
findings_in_scope: 3
fixed: 3
skipped: 0
status: all_fixed
---

# Phase 13: Code Review Fix Report

**Fixed at:** 2026-05-26
**Source review:** .planning/phases/13-cli-subcommands-deploy-ux/13-REVIEW.md
**Iteration:** 1

**Summary:**
- Findings in scope: 3
- Fixed: 3
- Skipped: 0

## Fixed Issues

### CR-01 + CR-02: Deploy-cancelled sentinel error + staging directory cleanup

**Files modified:** `internal/filetransfer/upload.go`, `cmd/docker-deploy/main.go`
**Commit:** 59647b2
**Applied fix:**
- Added `var ErrDeployCancelled = errors.New("deploy cancelled by user")` as a package-level sentinel in `upload.go`.
- Both cancellation return paths in `Upload()` (EOF/no-input branch and explicit "N" branch) now call `sshRun(client, "rm -rf "+ShellQuote(stagingDir), nil)` to clean the staging directory before returning.
- Both paths return `(0, ErrDeployCancelled)` instead of `(0, nil)`.
- `runDeploy` in `main.go` now checks `errors.Is(err, filetransfer.ErrDeployCancelled)` immediately after the `Upload` call, prints "Deploy cancelled." and returns nil — preventing fallthrough into `RunCompose` and `PollHealth`.
- Added `"errors"` to the import block in `main.go`.

Note: CR-01 and CR-02 were committed together because they are tightly coupled — CR-02's cleanup uses the same return path as CR-01's sentinel, and both changes are required for correctness.

### CR-03: Replace os.IsNotExist with errors.Is(err, fs.ErrNotExist)

**Files modified:** `cmd/docker-deploy/main.go`
**Commit:** 15a128a
**Applied fix:**
- Replaced `os.IsNotExist(err)` with `errors.Is(err, fs.ErrNotExist)` at the `os.Stat` call in `runValidate` (previously line 148).
- Added `"io/fs"` to the import block.
- `os.IsNotExist` does not unwrap error chains; `errors.Is` follows the Go 1.13+ idiomatic convention and handles wrapped errors correctly.

---

_Fixed: 2026-05-26_
_Fixer: Claude (gsd-code-fixer)_
_Iteration: 1_
