---
phase: 03-file-copy
reviewed: 2026-05-15T00:00:00Z
depth: quick
files_reviewed: 3
files_reviewed_list:
  - internal/filetransfer/upload.go
  - internal/filetransfer/shellquote_test.go
  - cmd/docker-deploy/main.go
findings:
  critical: 2
  warning: 2
  info: 1
  total: 5
status: issues_found
---

# Phase 03: Gap-Closure Code Review Report (Plan 03-04 Re-Review)

**Reviewed:** 2026-05-15
**Depth:** quick
**Files Reviewed:** 3 (targeted re-review of gap-closure changes)
**Status:** issues_found

## Summary

This review validates the gap-closure fixes for CR-01 (shellquote injection) and CR-02 (atomic swap rollback) from plan 03-04. Findings:

1. **CR-01 RESOLVED** — ShellQuote correctly escapes embedded single quotes using the `'\''` technique. All call sites in upload.go and main.go:179 use the escaped version.
2. **CR-02 PARTIALLY RESOLVED BUT NEW BLOCKER** — The atomic swap now includes rollback logic on step-2 failure (lines 191-194), but the rollback itself is fatally flawed: it calls `sudoRun(fmt.Sprintf("mv %s %s", ...))`, which will fail silently if the password is empty (the backup `oldDir` is not restored; remoteBase remains absent). This is a regression relative to the original code, which at least had deterministic behavior.
3. **NEW BLOCKER** — The `sudoRun` closure's lazy-sudo logic has a critical flow: if step 2 (placement of staging dir) is reached but fails **before** collecting the password (because the first unsudo'd attempt itself fails), and if the rollback in step 2 error path is reached, the rollback will SILENTLY FAIL because `sudoPw` is empty and the rollback uses `sudoRun` which tries without-sudo-first. The old backup dir is left in place and remoteBase is absent.
4. **CR-02 REGRESSION** — The backup cleanup (line 196: `rm -rf`) is now non-fatal (warning only, per the summary), but the error message does not include the backup directory path, leaving the operator without actionable recovery instructions.

---

## Critical Issues

### CR-01: ShellQuote injection — RESOLVED

**File:** `internal/filetransfer/upload.go:259-261` (lines verified)

**Status:** FIXED ✓

The implementation correctly uses `strings.ReplaceAll(s, "'", "'\\''")` to escape embedded single quotes. Test cases in `shellquote_test.go` verify the fix (all 5 cases including embedded quote pass). Call sites in `upload.go` (lines 119, 148, 159, 163, 188, 191, 196, 201, 212) all use `ShellQuote()`. The main.go line 179 also uses `filetransfer.ShellQuote(resolved.Path)`.

**No further action required for CR-01.**

---

### CR-02: Atomic swap rollback — BLOCKER (Regression)

**File:** `internal/filetransfer/upload.go:181-204`

**Issue:** The three-step atomic swap includes rollback on step 2 failure:

```go
if err := sudoRun(fmt.Sprintf("mv %s %s", ShellQuote(stagingDir), ShellQuote(remoteBase))); err != nil {
    // Rollback: restore remoteBase from backup.
    _ = sudoRun(fmt.Sprintf("mv %s %s", ShellQuote(oldDir), ShellQuote(remoteBase)))
    return fmt.Errorf("placing new version at target (backup is at %s): %w", oldDir, err)
}
```

The **critical flaw:** the rollback invokes `sudoRun()`. If the step-2 `mv` fails at a point where the sudo password was never collected (e.g., the first unsudo'd `mv stagingDir remoteBase` fails due to permission denied — `stagingDir` is in /tmp, owned by the deploy user, but remoteBase is owned by root), then `sudoPw` is empty. The rollback `sudoRun(fmt.Sprintf("mv %s %s", ...))` will then:

1. Try without sudo first: `mv /opt/app-old-<ts> /opt/app` — fails because remoteBase parent dir is owned by root
2. Check if `sudoPw == ""` — YES, it is
3. Interactively prompt for sudo password (up to 3 attempts)

This means the operator sees an interactive sudo prompt **during the error recovery path**, which violates the documented behavior in lines 130-133 ("at most one interactive prompt fires per Upload invocation"). If the operator is running this non-interactively (e.g., in CI/CD or as part of a larger deployment script), the rollback will hang or timeout waiting for input that never comes.

Even worse: if the password prompt times out or the operator does not enter it (because they are not at the terminal), the rollback is silently skipped (the `_` blank assignment ignores the error). The old backup dir remains at `oldDir`, remoteBase is absent, and the error message does not indicate recovery path.

**Second issue:** The error message says "backup is at %s" but does not provide the `sudo mv` command needed to recover manually. Contrast with the earlier warning (lines 165-176), which does provide actionable instructions.

**Fix:**

Option A (Strict): Collect the sudo password **before** any mv operation, not lazily. Move the initial `mkdir -p` step and password collection outside of the `existsBefore` conditional:

```go
// Before the 3-step swap, ensure we have sudo auth if mkdir needs it:
if err := sudoRun(fmt.Sprintf("mkdir -p %s", ShellQuote(remoteBase))); err != nil {
    // password collection happens here, then all subsequent ops use sudoPw
}
// NOW both existsBefore paths can safely use sudoRun in rollback
```

Option B (Pragmatic): Use the unsudo'd mv for rollback (do NOT call `sudoRun`), and document that rollback is best-effort:

```go
if err := sudoRun(fmt.Sprintf("mv %s %s", ShellQuote(stagingDir), ShellQuote(remoteBase))); err != nil {
    // Attempt rollback with unsudo'd mv (may fail silently if root-owned, but no interactive prompt).
    // Real recovery: operator can manually `sudo mv <oldDir> <remoteBase>` from the error message.
    _ = sshExec(client, fmt.Sprintf("mv %s %s", ShellQuote(oldDir), ShellQuote(remoteBase)))
    return fmt.Errorf(
        "placing new version at target (old files are at %s on %s). Recover with:\n"+
        "  ssh %s 'sudo mv %s %s'\n",
        oldDir, client.RemoteAddr().String(),
        client.RemoteAddr().String(), ShellQuote(oldDir), ShellQuote(remoteBase),
    )
}
```

**Current fix is BLOCKER: do not ship.**

---

### WR-01: Backup cleanup failure now non-fatal but lacks recovery instructions

**File:** `internal/filetransfer/upload.go:196-198`

**Issue:** The cleanup of the old backup dir (step 3 of the swap) now prints a warning instead of returning an error (per the summary, "non-fatal"). However, the warning at line 197:

```go
fmt.Fprintf(os.Stderr, "Warning: could not remove backup directory %s: %v\n", oldDir, err)
```

Does not provide the path or command needed for the operator to clean up manually. The earlier warning (lines 165-176) is much more actionable.

**Fix:** Include a cleanup command in the warning:

```go
fmt.Fprintf(os.Stderr,
    "Warning: could not remove backup directory %s: %v\n"+
    "To clean up manually:\n"+
    "  ssh %s 'sudo rm -rf %s'\n",
    oldDir, err,
    client.RemoteAddr().String(), ShellQuote(oldDir),
)
```

This is a WARNING (not critical), but the operator feedback is now complete.

---

## Warnings

### WR-02: `main.go` line 179 existence check does NOT use ShellQuote

**File:** `cmd/docker-deploy/main.go:179`

**Issue:** The file reads:

```go
out, err := session.Output(fmt.Sprintf("test -d %s && echo exists || echo absent", filetransfer.ShellQuote(resolved.Path)))
```

This DOES use `ShellQuote`. ✓ No issue here; the fix was applied correctly.

**(Retracted — code is correct)**

---

### WR-03: sudoRun closure error handling does not document the laziness constraint

**File:** `internal/filetransfer/upload.go:130-160`

**Issue:** The `sudoRun` closure (lines 135-160) implements "lazy sudo" — it tries each command without elevation first, and only collects the password on first failure. The docstring at lines 130-133 claims "at most one interactive prompt fires per Upload invocation", which is true IF all operations are attempted. However, if an operation fails **before the password is ever collected**, subsequent rollback operations that also need sudo will either:

1. Hang waiting for a prompt (if running interactively)
2. Fail silently (if running non-interactively)

The docstring does not document this edge case or warn the caller. If the caller (runDeploy) is called from a script or CI/CD pipeline with stdin redirected, the behavior is undefined.

**Fix:** Document the constraint in the docstring:

```go
// sudoRun tries cmd without sudo first; on failure it collects the
// password interactively (once, with up to 3 attempts) and retries.
// If called with stdin not connected to a TTY, the interactive prompt
// will fail, and rollback operations requiring sudo will fail silently.
// Callers should ensure stdin is available when sudo elevation may be needed.
sudoRun := func(cmd string) error {
    ...
}
```

This is a WARNING due to the unclear behavior in non-interactive contexts.

---

## Info

### IN-01: ShellQuote test coverage could include shell metacharacters beyond single quotes

**File:** `internal/filetransfer/shellquote_test.go:5-46`

**Issue:** The test cases cover empty string, normal path, embedded single quotes, and multiple quotes. However, they do not test other shell metacharacters that are NOT escaped by wrapping in single quotes:

- Newlines: `/opt/foo\nbar` — the newline inside single quotes is literal and will confuse commands like `mkdir -p '/opt/foo\nbar'`
- Null bytes: not possible in Go strings, but worth documenting
- Unicode: `/opt/föö` — should work but not explicitly tested

However, given that `remoteBase` is derived from `Resolve()` which validates it via `ParseHost` (SSH URL parsing), and the staging dir uses only alphanumerics + timestamp, the current test coverage is **adequate for the actual call sites**. This is an info item, not a blocker.

**Suggestion:** Add a comment to `ShellQuote` documenting the assumption that inputs are already validated:

```go
// ShellQuote wraps s in single quotes for safe use in shell commands,
// escaping any embedded single quotes using the '\'' technique.
// Assumes s contains no newlines or other control characters.
// All call sites pass values from Resolve() (which validates SSH URLs)
// or synthetic staging dir names (alphanumerics + timestamp) — T-03-05.
```

---

_Reviewed: 2026-05-15_
_Reviewer: Claude (gsd-code-reviewer)_
_Depth: quick_
