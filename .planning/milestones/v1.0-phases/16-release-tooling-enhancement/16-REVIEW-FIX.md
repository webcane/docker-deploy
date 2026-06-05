---
phase: 16-release-tooling-enhancement
fixed_at: 2026-05-29T00:00:00Z
review_path: .planning/phases/16-release-tooling-enhancement/16-REVIEW.md
iteration: 1
findings_in_scope: 13
fixed: 13
skipped: 0
status: all_fixed
---

# Phase 16: Code Review Fix Report

**Fixed at:** 2026-05-29
**Source review:** `.planning/phases/16-release-tooling-enhancement/16-REVIEW.md`
**Iteration:** 1

**Summary:**
- Findings in scope: 13
- Fixed: 13
- Skipped: 0

## Fixed Issues

### CR-01: Goroutine leak — SSH dial goroutine is never joined after timeout or context cancel

**Files modified:** `internal/ssh/client.go`
**Commit:** d50860f
**Applied fix:** Added drain goroutine in both the `ctx.Done()` and `time.After(timeout)` select branches. Each branch spawns a background goroutine that reads from `ch` and calls `r.client.Close()` if a client arrives after the caller has already returned.

---

### CR-02: Shell injection in health docker ps filter — ShellQuote applied outside the filter value

**Files modified:** `internal/health/poll.go`
**Commit:** d50860f
**Applied fix:** Changed `listContainers` to build the full filter token (`label=com.docker.compose.project=<projectName>`) and then apply `ShellQuote` to the entire token. Previously `ShellQuote` was applied only to `projectName`, causing literal single-quote characters to be passed as part of the Docker filter value.

---

### CR-03: TOFU known-hosts race — concurrent appendKnownHost calls can corrupt file

**Files modified:** `internal/ssh/knownhosts.go`
**Commit:** d50860f
**Applied fix:** Added a package-level `knownHostsMu sync.Mutex` variable and added `Lock()`/`Unlock()` at the start of `appendKnownHost`. Added `"sync"` to imports.

---

### WR-01: Double error message printed in runValidate

**Files modified:** `cmd/docker-deploy/main.go`
**Commit:** d50860f
**Applied fix:** Removed the three `fmt.Fprintln(os.Stderr, ...)` calls in `runValidate` (lines 153, 163, 170). The returned errors propagate to cobra's `RunE` handler which prints them. Added a comment explaining the intent.

---

### WR-02: DialConfig.Stdout field misleadingly named

**Files modified:** `internal/ssh/client.go`, `cmd/docker-deploy/main.go`, `internal/ssh/client_test.go`, `integration/dial_test.go`
**Commit:** d50860f
**Applied fix:** Renamed `DialConfig.Stdout io.Writer` to `DialConfig.UserOutput io.Writer` everywhere: struct definition, nil-check in `Dial()`, `handleTOFU`/`handleKeyMismatch` call sites, and all struct literals in `main.go`, `client_test.go`, and `integration/dial_test.go`.

---

### WR-03: .env backup skipped when .env is excluded via glob pattern

**Files modified:** `internal/filetransfer/upload.go`
**Commit:** d50860f
**Applied fix:** Replaced the `for _, exc := range excludes { if exc == ".env" { ... break } }` loop with a single `if existsBefore && ShouldExclude(".env", excludes)` condition. `ShouldExclude` handles both exact string matches and glob patterns consistently with `WalkFiles`.

---

### WR-04: release-tag.md Step 6 may stage pre-existing uncommitted changes

**Files modified:** `.claude/commands/gsd/release-tag.md`
**Commit:** d50860f
**Applied fix:** Added Wave 0 Step 0 — a `git diff --name-only HEAD -- README.md INSTALL.md .planning/STATE.md` check that aborts if any of the three release files have uncommitted changes before any edits begin.

---

### WR-05: checkTargetDir checks target writability but atomic swap needs parent writable

**Files modified:** `internal/preflight/checks.go`
**Commit:** d50860f
**Applied fix:** Updated `checkTargetDir` to compute `parentPath := path.Dir(cfg.Path)` and include `test -w <parentPath>` in the writability probes (steps 1 and 2). Added `"path"` to imports. The sudo fallback (step 3) is unchanged — if the user has passwordless sudo, the mv will succeed regardless of parent writability.

---

### WR-06: SudoExec verbose log hardcodes "exit 1" on direct-failure path

**Files modified:** `internal/filetransfer/upload.go`
**Commit:** d50860f
**Applied fix:** Changed the verbose message from `"  → exit 1 (direct failed, trying sudo)\n"` to `"  → direct failed, trying sudo\n"` to avoid implying the failure was specifically exit code 1.

---

### IN-01: sshDialTimeout comment says "covers TCP phase only"

**Files modified:** `cmd/docker-deploy/main.go`
**Commit:** d50860f
**Applied fix:** Updated the comment to correctly state the timeout covers the full SSH handshake (TCP dial + protocol negotiation + authentication) and references the goroutine + select enforcement per CLAUDE.md Rule 2.

---

### IN-02: Permanently-skipped upload test stubs

**Files modified:** `internal/filetransfer/upload_test.go`
**Commit:** d50860f
**Applied fix:** Replaced the five permanently-skipped `t.Skip` stubs with one real test: `TestSudoExec_WrongPasswordRetry`. This test uses `promptSudoPasswordFunc` injection to supply a wrong password on all 3 attempts and verifies the function returns an error containing "no valid auth path available" and calls the prompt exactly 3 times. The `TestUploadAuthFallback_InteractivePassword`, `_Timeout`, `_RootUser`, and `_AllPathsExhausted` stubs were removed — they describe infrastructure-level behaviors (terminal detection, root-user warnings) that are not unit-testable without significant additional mocking; the wrong-password retry loop is the highest-value gap and is now covered.

---

### IN-03: Confirmation plan understates scope of version replacements

**Files modified:** `.claude/commands/gsd/release-tag.md`
**Commit:** d50860f
**Applied fix:** Updated Step 4 confirmation message to show explicit literal strings (`"$CURRENT_TAG" → "$NEXT_TAG"`) rather than `s/$CURRENT_TAG/$NEXT_TAG/g` sed notation. Also added the STATE.md update to the plan so the user sees the full scope.

---

### IN-04: loadSSHConfigKeys ignores user parameter without documentation

**Files modified:** `internal/ssh/client.go`
**Commit:** d50860f
**Applied fix:** Added a doc comment above `loadSSHConfigKeys` documenting that the `user` parameter is intentionally ignored, that OpenSSH config `User` directives within `Host` blocks are not yet parsed, and that User-aware parsing is a future enhancement. The `_` parameter name is kept as-is to signal the intentional deferral.

---

_Fixed: 2026-05-29_
_Fixer: Claude (gsd-code-fixer)_
_Iteration: 1_
