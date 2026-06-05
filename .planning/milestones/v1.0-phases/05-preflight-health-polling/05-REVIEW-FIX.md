---
phase: 05-preflight-health-polling
fixed_at: 2026-05-17T00:00:00Z
review_path: .planning/phases/05-preflight-health-polling/05-REVIEW.md
iteration: 1
findings_in_scope: 4
fixed: 4
skipped: 0
status: all_fixed
---

# Phase 5: Code Review Fix Report

**Fixed at:** 2026-05-17T00:00:00Z
**Source review:** .planning/phases/05-preflight-health-polling/05-REVIEW.md
**Iteration:** 1

**Summary:**
- Findings in scope: 4 (CR-01, WR-01, WR-02, WR-03)
- Fixed: 4
- Skipped: 0

## Fixed Issues

### CR-01: `--dry-run` fails when no compose file exists locally

**Files modified:** `cmd/docker-deploy/main.go`
**Commit:** dd2b154
**Applied fix:** Changed the `flagComposeFile` argument in the `runDryRun` call to `config.Resolve` from `""` to `"docker-compose.yml"`. This sentinel value hits the `flagComposeFile != ""` branch in Resolve and bypasses the auto-detect path that stat-checks for compose.yaml/docker-compose.yml on disk. The value is discarded immediately after via `_ = composeFile` as the comment documents.

### WR-01: `sessionOutput.Output(cmd)` silently ignores its argument

**Files modified:** `internal/health/poll.go`, `internal/health/poll_test.go`
**Commit:** 1b645fe
**Applied fix:** Removed the `cmd string` parameter from the `sessionOutput` interface and from `sshSessionWrapper.Output`. The method now takes no arguments and always executes `w.cmd` (baked in at `newSession` construction time). Updated both `session.Output(cmd)` call sites in `listContainers` and `inspectHealth` to `session.Output()`. Updated `fakeSession.Output` in the test file to match the new signature. All existing health tests continue to pass.

### WR-02: Negative `health_timeout` / `health_interval` in deploy.yaml accepted silently

**Files modified:** `internal/config/config.go`
**Commit:** bd6effa
**Applied fix:** Added explicit validation checks before the `> 0` switch blocks in `Resolve`. If `file.Target.HealthTimeout < 0` or `file.Target.HealthInterval < 0`, `Resolve` now returns a descriptive `fmt.Errorf` so the user sees a clear error rather than silently getting the 60s/5s defaults.

### WR-03: `checkSudo` error message exposes username without sanitization

**Files modified:** `internal/config/config.go`
**Commit:** 14c5a8d
**Applied fix:** Added `isValidUnixUsername(s string) bool` helper that allows only `[a-zA-Z0-9._-]` characters. Called it in `ParseHost` immediately after extracting the username from the parsed URL — if the username is non-empty and fails the check, `ParseHost` returns an error with the offending username quoted. This blocks malicious usernames from ever reaching `checkSudo`'s operator-facing error message. No changes to `checks.go` were needed since the upstream validation in `ParseHost` is the correct trust boundary.

## Skipped Issues

None — all in-scope findings were fixed.

---

_Fixed: 2026-05-17T00:00:00Z_
_Fixer: Claude (gsd-code-fixer)_
_Iteration: 1_
