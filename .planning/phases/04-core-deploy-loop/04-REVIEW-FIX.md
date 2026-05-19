---
phase: 04-core-deploy-loop
fixed_at: 2026-05-19T17:00:00Z
review_path: .planning/phases/04-core-deploy-loop/04-REVIEW.md
iteration: 1
findings_in_scope: 8
fixed: 8
skipped: 0
status: all_fixed
---

# Phase 4: Code Review Fix Report

**Fixed at:** 2026-05-19T17:00:00Z
**Source review:** .planning/phases/04-core-deploy-loop/04-REVIEW.md
**Iteration:** 1

**Summary:**
- Findings in scope: 8
- Fixed: 8
- Skipped: 0

## Fixed Issues

### CR-01: Shell Injection via Incorrect Quoting Order in Compose Command

**Files modified:** `internal/compose/run.go`
**Commit:** 45c2fad
**Applied fix:** Quote remotePath and composeFile components separately instead of concatenating them before quoting. This makes the security boundary explicit to future maintainers and aligns with documented pattern T-04-02-01. While the previous approach was technically safe due to ShellQuote's proper escaping, best practice requires quoting each component independently.

### WR-01: SFTP Resource Leak on MkdirAll Failure

**Files modified:** `internal/filetransfer/upload.go`
**Commit:** 7448fd7
**Applied fix:** Protected SFTP client with `defer sftpClient.Close()` immediately after NewClient succeeds, ensuring cleanup in all error paths. Removed manual Close() calls that are now redundant and would have been unsafe if a panic occurred between NewClient and the defer.

### WR-02: Context Cancellation Not Fully Honored in RunCompose

**Files modified:** `internal/compose/run.go`
**Commit:** ac4e280
**Applied fix:** Added `sync.WaitGroup` to ensure the context cancellation watcher goroutine completes before RunCompose returns. This prevents goroutine accumulation in high-concurrency scenarios (rapid successive deployments).

### WR-03: Relative Path Validation Missing in Config Resolution

**Files modified:** `internal/config/config.go`, `cmd/docker-deploy/main.go`
**Commit:** 26a69f2
**Applied fix:** Moved absolute path validation from runDeploy() into config.Resolve() so it applies to both dry-run and deploy flows. Used filepath.IsAbs() for POSIX/Windows compatibility instead of strings.HasPrefix(). This prevents dry-run from succeeding with invalid relative paths.

### WR-04: stderr Warnings on Session Close Race with Concurrent Output

**Files modified:** `internal/compose/run.go`
**Commit:** 084f018
**Applied fix:** Suppressed expected errors (io.EOF and syscall.EPIPE) when closing the session, as these occur naturally when the remote process exits or the connection breaks. Unexpected errors are still logged for diagnosis. This reduces noisy output in concurrent deployments.

### IN-01: Hardcoded SSH Dial Timeout Not Configurable

**Files modified:** `cmd/docker-deploy/main.go`
**Commit:** b2115f7
**Applied fix:** Extracted the hardcoded `10 * time.Second` timeout into a named package-level constant `sshDialTimeout`. Updated both runDryRun() and runDeploy() to use this constant, eliminating duplication and making it easier to maintain or configure in future phases.

### IN-02: Unused composeFile Parameter in runDryRun

**Files modified:** `cmd/docker-deploy/main.go`
**Commit:** 2fc6172
**Applied fix:** Added comprehensive documentation to the runDryRun() function comment explaining why the composeFile parameter is accepted but unused. Clarified that the parameter exists for API symmetry with runDeploy(), and that dry-run only verifies SSH connectivity and config resolution (not compose execution).

### IN-03: Magic Number for Staging Directory Timestamp Precision

**Files modified:** `internal/filetransfer/upload.go`
**Commit:** b6b7afc
**Applied fix:** Changed from `time.Now().Unix()` (second precision) to `time.Now().UnixNano()` (nanosecond precision) for the staging directory timestamp. This eliminates potential collisions when multiple concurrent deployments target the same remote in the same second.

---

_Fixed: 2026-05-19T17:00:00Z_
_Fixer: Claude (gsd-code-fixer)_
_Iteration: 1_
