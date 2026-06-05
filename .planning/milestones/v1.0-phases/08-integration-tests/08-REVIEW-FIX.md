---
phase: 08-integration-tests
fixed_at: 2026-05-22T06:28:00Z
review_path: .planning/phases/08-integration-tests/08-REVIEW.md
iteration: 1
findings_in_scope: 8
fixed: 4
skipped: 4
status: partial
---

# Phase 08: Code Review Fix Report

**Fixed at:** 2026-05-22T06:28:00Z
**Source review:** .planning/phases/08-integration-tests/08-REVIEW.md
**Iteration:** 1

**Summary:**
- Findings in scope: 8 (CR-01, CR-02, CR-03, WR-01, WR-02, WR-03, WR-04, WR-05)
- Fixed: 4 (WR-01, WR-02, WR-04, WR-05)
- Skipped: 4 (CR-01, CR-02, CR-03, WR-03 — all already resolved in prior commits)

## Fixed Issues

### WR-01: `captureStderr` ignores `os.Pipe()` error — nil pipe causes panic

**Files modified:** `integration/helpers_test.go`, `integration/preflight_test.go`
**Commit:** 7c658d5
**Applied fix:** Changed `captureStderr` signature to accept `*testing.T`, added `t.Helper()`, and replaced the discarded `os.Pipe()` error with `t.Fatalf`. Updated the single call site in `preflight_test.go` to pass `t` as the first argument.

### WR-02: False-pass condition when neither check result exists in nosudouser test

**Files modified:** `integration/preflight_test.go`
**Commit:** f5f703a
**Applied fix:** Added an explicit nil guard (`if targetDir == nil && dockerGroup == nil { t.Fatal(...) }`) before the `allPass` logic. This ensures the test fails loudly when neither expected check result is returned, rather than silently passing because the nil-check branches were never entered.

### WR-04: `entrypoint.sh` starts sshd even when `dockerd` fails

**Files modified:** `integration/testdata/entrypoint.sh`
**Commit:** 7498270
**Applied fix:** After the 30-second readiness loop, added a `docker info` assertion that exits with code 1 and an error message if the daemon is still not available. This prevents the container from appearing healthy (port 22 open) while Docker is broken, which would cause all Docker-dependent tests to fail with opaque SSH errors.

### WR-05: Staging path in `/tmp` causes cross-filesystem rename (non-atomic)

**Files modified:** `internal/filetransfer/upload.go`, `integration/filetransfer_test.go`
**Commit:** a54b5db
**Applied fix:** Changed the staging directory in `upload.go` from `/tmp/docker-deploy-<ts>` to `path.Dir(remoteBase) + "/.deploy-tmp-" + timestamp`, co-locating it with the target so the final `mv` is an atomic same-filesystem `rename(2)` per CLAUDE.md Rule 3. Updated the comment block in the doc comment and the test assertion in `filetransfer_test.go` to match the new path (`/opt/.deploy-tmp-*`).

## Skipped Issues

### CR-01: `container.Exec` reads raw multiplexed Docker stream

**File:** `integration/helpers_test.go:179`
**Reason:** Already resolved in prior commit `40a1cf1` (`fix(08): use CopyFileFromContainer instead of container.Exec for key extraction`). The current code at the cited location uses `container.CopyFileFromContainer` (the Docker API TAR-based copy) rather than `container.Exec`, which entirely avoids the multiplexed stream issue.
**Original issue:** Private key bytes corrupted by 8-byte Docker stream multiplexing headers; `TestMain` always failed.

### CR-02: `TestDial_Timeout` fails when no auth methods available before TCP connect

**File:** `integration/dial_test.go:17-33`
**Reason:** Already resolved in a prior commit. The current code at lines 21-28 includes `KnownHostsPath: emptyKnownHosts(t)` which ensures `buildAuthMethods` proceeds past the "no methods" early-exit so the TCP connection attempt reaches 192.0.2.1 and the 500 ms timeout fires as expected.
**Original issue:** `buildAuthMethods` returned empty on CI (no SSH agent, no `~/.ssh/config`), causing auth error before any TCP connect.

### CR-03: Integration test timeout too short for cold CI runners

**File:** `Makefile:15`, `.github/workflows/ci.yml`
**Reason:** Already resolved in prior commits. `Makefile` currently uses `-timeout 15m` and `.github/workflows/ci.yml` already has `timeout-minutes: 20` on the `integration` job.
**Original issue:** `-timeout 5m` was shorter than combined container startup cost on cold runners.

### WR-03: `TestUpload_AtomicCancel` — `cancel()` not deferred; context leak on skip

**File:** `integration/filetransfer_test.go:81-86`
**Reason:** Already resolved in a prior commit. The current code has `defer cancel()` at line 90 immediately after context creation, and `t.Cleanup` for remote directory removal is registered at lines 79-81, before any `t.Skip` call site.
**Original issue:** Context leaked if `t.Skip` fired before goroutine completed; cleanup registered after skip point.

---

_Fixed: 2026-05-22T06:28:00Z_
_Fixer: Claude (gsd-code-fixer)_
_Iteration: 1_
