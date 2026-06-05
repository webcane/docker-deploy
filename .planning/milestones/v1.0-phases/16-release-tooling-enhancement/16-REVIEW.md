---
phase: 16-release-tooling-enhancement
reviewed: 2026-05-27T00:00:00Z
depth: standard
files_reviewed: 18
files_reviewed_list:
  - .claude/commands/gsd/release-tag.md
  - .golangci.yml
  - cmd/docker-deploy/main.go
  - cmd/docker-deploy/main_test.go
  - internal/compose/run.go
  - internal/compose/run_test.go
  - internal/config/config.go
  - internal/config/config_test.go
  - internal/filetransfer/filter.go
  - internal/filetransfer/filter_test.go
  - internal/filetransfer/upload.go
  - internal/filetransfer/upload_test.go
  - internal/health/poll.go
  - internal/preflight/checks.go
  - internal/preflight/checks_test.go
  - internal/ssh/client.go
  - internal/ssh/knownhosts.go
  - internal/sshconfig/sshconfig.go
findings:
  critical: 3
  warning: 6
  info: 4
  total: 13
status: issues_found
---

# Phase 16: Code Review Report

**Reviewed:** 2026-05-27T00:00:00Z
**Depth:** standard
**Files Reviewed:** 18
**Status:** issues_found

## Summary

Reviewed the full source tree submitted for phase 16 (release tooling enhancement plus existing codebase). The release-tag command script is the new deliverable; the rest of the codebase was reviewed at standard depth as context.

Three critical issues were found: a goroutine leak in the SSH dial path that leaks the underlying TCP connection on timeout, a shell injection vector in the health-polling `docker ps` command, and a TOFU race condition in known-hosts append. Six warnings cover logic gaps (double error print in `runValidate`, wrong `Stdout` field wired in dry-run, missed `.env` backup when `.env` appears as a non-exact-match exclude, the `release-tag` script committing unintended files, and two minor logic gaps in the preflight check). Four informational items cover dead code, skipped tests, and a magic-string convention.

---

## Critical Issues

### CR-01: Goroutine leak — SSH dial goroutine is never joined after timeout or context cancel

**File:** `internal/ssh/client.go:137-153`
**Issue:** When the `time.After(timeout)` or `ctx.Done()` branch fires first, the goroutine launched on line 139 continues running and will eventually complete its `gossh.Dial` call — returning a live `*gossh.Client` to a channel that no one reads. The client is never closed, leaking the TCP connection and the goroutine. On a host that accepts TCP connections but hangs on SSH negotiation the leak persists indefinitely.

```go
// current — goroutine runs to completion even after timeout fires
go func() {
    c, dialErr := gossh.Dial("tcp", addr, clientCfg)
    ch <- result{c, dialErr}
}()
select {
case <-time.After(timeout):
    return nil, fmt.Errorf("SSH connection timed out after %v", timeout)
// goroutine still running, nobody drains ch
```

**Fix:** Add a background drain goroutine that closes the client if one arrives after the caller has already returned:

```go
go func() {
    c, dialErr := gossh.Dial("tcp", addr, clientCfg)
    ch <- result{c, dialErr}
}()
select {
case <-ctx.Done():
    // drain and close in background
    go func() {
        if r := <-ch; r.client != nil {
            r.client.Close()
        }
    }()
    return nil, fmt.Errorf("SSH connection cancelled: %w", ctx.Err())
case <-time.After(timeout):
    go func() {
        if r := <-ch; r.client != nil {
            r.client.Close()
        }
    }()
    return nil, fmt.Errorf("SSH connection timed out after %v", timeout)
case r := <-ch:
    ...
}
```

---

### CR-02: Shell injection in health `docker ps` filter — `ShellQuote` applied outside the filter value

**File:** `internal/health/poll.go:172`
**Issue:** The `docker ps` command is constructed as:

```go
cmd := "docker ps -a --filter label=com.docker.compose.project=" +
    filetransfer.ShellQuote(projectName) + " --format '{{.Names}}'"
```

`ShellQuote` wraps `projectName` in single quotes, producing:

```
docker ps -a --filter label=com.docker.compose.project='myapp' --format '{{.Names}}'
```

This is correct for simple names. However the Docker filter parser (`label=key=value`) splits on the **first** `=` only, so the `key` here is `com.docker.compose.project` and the `value` is `'myapp'`. The shell single-quotes are passed literally to Docker as part of the value string, meaning Docker looks for a container with the label value `'myapp'` (with surrounding single-quote characters) rather than `myapp`. No containers will match, so `PollHealth` silently returns `nil` (no containers to poll) on every deploy — the health check is effectively dead.

**Fix:** Do not shell-quote the filter argument. The entire `--filter label=...` token should be single-quoted as one unit so the shell sees it as one argument but Docker receives the unquoted value:

```go
filterVal := "label=com.docker.compose.project=" + projectName
cmd := "docker ps -a --filter " + filetransfer.ShellQuote(filterVal) + " --format '{{.Names}}'"
```

Separately, `projectName` (the local directory basename) should be validated earlier to contain only safe characters before being used in a remote command.

---

### CR-03: TOFU known-hosts race — file opened read-only by `knownhosts.New` before `appendKnownHost` writes

**File:** `internal/ssh/knownhosts.go:51-58` and `internal/ssh/client.go:218`
**Issue:** `buildHostKeyCallback` opens the file with `O_CREATE|O_APPEND|O_WRONLY` (line 53) to ensure it exists, then **closes** the file (line 57) before passing the path to `knownhosts.New`. The `knownhosts.New` call reads the file content at that point in time. Later, when a TOFU prompt is accepted, `appendKnownHost` appends a new line. This sequence is correct in isolation, but `buildHostKeyCallback` returns a callback closure that captures `knownHostsPath` — the file is **not re-read** between the initial load and subsequent calls to the callback.

A separate, more acute problem: if two concurrent `Dial` calls race on the same unknown host, both will receive `UnknownHostError`, both will prompt the user (or one will, and the other will silently append a duplicate line), and both will call `appendKnownHost` concurrently. `appendKnownHost` has no locking, so concurrent appends can corrupt the file. This matters in the single-binary context where tests or concurrent operations both call `Dial`.

**Fix:** Protect `appendKnownHost` with a package-level mutex, or ensure `Dial` is documented as not safe for concurrent use on the same `knownHostsPath`.

```go
var knownHostsMu sync.Mutex

func appendKnownHost(knownHostsPath string, ...) error {
    knownHostsMu.Lock()
    defer knownHostsMu.Unlock()
    // ... existing append logic
}
```

---

## Warnings

### WR-01: Double error message printed in `runValidate` — error written to stderr then again via cobra

**File:** `cmd/docker-deploy/main.go:153-155, 163-165, 169-171`
**Issue:** `runValidate` explicitly calls `fmt.Fprintln(os.Stderr, err.Error())` before returning the error. Since `buildValidateCmd` uses `RunE`, cobra will also print the error (unless `SilenceErrors` is set on the parent). The same pattern occurs in two of the three error paths. This results in the error message appearing twice in the terminal.

**Fix:** Remove the manual `fmt.Fprintln(os.Stderr, ...)` calls inside `runValidate`. Let the returned error propagate to cobra's error handler, or set `SilenceErrors: true` on the command and handle printing in a single place.

```go
// Remove these lines — cobra prints the returned error:
// fmt.Fprintln(os.Stderr, "deploy.yaml not found")
// fmt.Fprintln(os.Stderr, err.Error())
```

---

### WR-02: `runDryRun` wires `DialConfig.Stdout` to `os.Stderr` — TOFU prompts go to wrong stream

**File:** `cmd/docker-deploy/main.go:255-258`
**Issue:** The `DialConfig` struct has both `Stdin` and `Stdout` fields. In `runDryRun`, `Stdout` is set to `os.Stderr`. The field is used by `handleTOFU` (client.go:207-209) to print the TOFU fingerprint prompt and the permanent-addition warning. Writing a user-facing interactive prompt to `stderr` is unusual and will cause the prompt to disappear in piped output scenarios. The same issue exists in `runDeploy` (line 341-344). However, since the CLAUDE.md `Stdout` field comment says "Pass `os.Stderr` from callers", this appears to be an intentional design choice. If so, the `Stdout` field on `DialConfig` is misleadingly named — it should be called `PromptWriter` or `UserOutput` to avoid confusion.

**Fix (if intentional — clarify the API):**
```go
// Rename DialConfig.Stdout to DialConfig.UserOutput to signal it is
// not stdout but rather the stream for user-facing interactive messages.
```

**Fix (if unintentional — use os.Stdout for prompts):**
```go
dialCfg := sshpkg.DialConfig{
    ...
    Stdout: os.Stdout, // TOFU prompts are user-facing, not error output
}
```

---

### WR-03: `.env` backup skipped when `.env` is excluded via a glob pattern, not exact string

**File:** `internal/filetransfer/upload.go:453-468`
**Issue:** The `.env` backup logic iterates `excludes` and checks `exc == ".env"` (exact string equality). If the caller excluded `.env` via a glob pattern that happens to match `.env` (e.g. `".env*"` or via `ShouldExclude`), the backup will not run but `.env` will still be absent from the uploaded files, causing the remote `.env` to be destroyed by the atomic swap. In practice, the path is that `config.Resolve` appends the literal string `".env"` when `SkipEnv` is true, so this is currently safe. But the logic is fragile: any future code path that excludes `.env` via a non-exact pattern will silently break `.env` preservation without any error.

**Fix:** Replace the linear scan for `".env"` with a call to `ShouldExclude`:

```go
if existsBefore && ShouldExclude(".env", excludes) {
    envPath := path.Join(remoteBase, ".env")
    // ... backup logic
}
```

---

### WR-04: `release-tag.md` Step 6 uses `git add README.md INSTALL.md .planning/STATE.md` — may accidentally stage pre-existing unstaged changes to those files

**File:** `.claude/commands/gsd/release-tag.md:171-174`
**Issue:** The commit step runs `git add README.md INSTALL.md .planning/STATE.md` without checking whether those files have unexpected pre-existing modifications. If the working tree has unrelated uncommitted changes to any of those files (e.g. a half-edited README), they will be included in the release commit without the user realising it. The `git diff --staged` confirmation in Step 4 shows only what *will* change after the Edit calls, not what was already staged.

**Fix:** Add a `git diff --name-only HEAD -- README.md INSTALL.md .planning/STATE.md` check in Wave 0 (before any edits) and abort if any of those files already have uncommitted changes:

```bash
dirty=$(git diff --name-only HEAD -- README.md INSTALL.md .planning/STATE.md)
if [ -n "$dirty" ]; then
  echo "ABORT: release files have uncommitted changes — commit or stash first"
  exit 1
fi
```

---

### WR-05: `checkTargetDir` step 2 (`mkdir -p && test -w`) — `mkdir -p` on existing dir always exits 0, `test -w` match may be wrong session

**File:** `internal/preflight/checks.go:238-243`
**Issue:** The combined command `mkdir -p <path> && test -w <path>` is sent as a single string to `runCmd`, which opens one SSH session and calls `session.Run`. This is correct. However, the comment in the code says "mkdir -p returns 0 for directories that already exist — even if the caller cannot write to them". If `mkdir -p` returns 0 (because the dir exists) and `test -w` subsequently returns non-zero (dir not writable), the whole `&&` chain exits non-zero — which is correct. But this means the check will correctly fall through to step 3 (sudo). There is no bug in the logic per se.

The actual issue is that the check at step 2 (`mkdir -p <path> && test -w <path>`) can **succeed** (exit 0) for a directory that is writable only because `mkdir` just created it with default permissions. But on many systems, `/opt/myapp` created by the SSH user will be owned by that user and writable, so the check passes — but the parent `/opt` is root-owned, and later `mv /opt/myapp /opt/myapp-old` will fail during the Upload atomic swap. Preflight passes but deploy fails. The preflight check and the Upload probe are testing different conditions: preflight checks if `cfg.Path` is writable, Upload probes if `path.Dir(cfg.Path)` is writable. They are inconsistent.

**Fix:** Align the preflight `checkTargetDir` probe to also check the parent directory:

```go
parentPath := path.Dir(cfg.Path)  // path (not filepath) — remote is Linux
// Check parent writability instead of / in addition to target writability
if err := runCmd(client, "test -w "+filetransfer.ShellQuote(parentPath)); err == nil {
    // parent writable — mv will succeed
}
```

---

### WR-06: `SudoExec` step 1 verbose logging logs the command before it runs but does not log the actual exit code on direct-failure path

**File:** `internal/filetransfer/upload.go:108-116`
**Issue:** When `verbose=true` and step 1 (direct) fails, the code prints `"  → exit 1 (direct failed, trying sudo)"`. However the actual exit code is not checked — it prints a hardcoded `exit 1` even if the command failed due to a different error (connection reset, session limit, etc.). This misleads debugging.

```go
if err := sshRun(client, cmd, nil); err == nil {
    ...
    return nil
}
if verbose {
    fmt.Fprintf(os.Stderr, "  → exit 1 (direct failed, trying sudo)\n") // hardcoded, may be wrong
}
```

**Fix:** Either inspect the error type to get the real exit code, or use a generic message:

```go
if verbose {
    fmt.Fprintf(os.Stderr, "  → failed (direct, trying sudo): reason unknown\n")
}
```

---

## Info

### IN-01: `sshDialTimeout` comment says "covers TCP phase only" but the goroutine+select implementation covers the full handshake

**File:** `cmd/docker-deploy/main.go:34-37`
**Issue:** The comment on `sshDialTimeout` reads: "This timeout covers the TCP dial phase; SSH protocol negotiation and authentication may take additional time". This is misleading — the `Dial` implementation in `internal/ssh/client.go` uses a goroutine + `time.After` to enforce the full SSH handshake timeout (per CLAUDE.md Rule 2). The comment predates the full implementation and is now incorrect.

**Fix:** Update the comment:
```go
// sshDialTimeout is the maximum time to wait for the full SSH handshake
// (TCP dial + protocol negotiation + authentication) to complete.
// Enforced via goroutine + select in internal/ssh.Dial per CLAUDE.md Rule 2.
const sshDialTimeout = 10 * time.Second
```

---

### IN-02: Several upload test cases are permanently skipped with `t.Skip`

**File:** `internal/filetransfer/upload_test.go:325-350`
**Issue:** Four test functions — `TestUploadAuthFallback_InteractivePassword`, `TestUploadAuthFallback_InteractivePassword_WrongPassword`, `TestUploadAuthFallback_InteractivePassword_Timeout`, and `TestUploadAuthFallback_RootUser` — call `t.Skip("... to be implemented in GREEN phase")`. These have never been implemented. The password retry logic (`SudoExec` step 4) is production code with no test coverage for the wrong-password and timeout scenarios. `TestSudoExec_AllStepsExhausted` covers the EOF case but not the 3-attempt retry loop.

**Fix:** Implement or remove these test stubs. At minimum implement the wrong-password retry test using `promptSudoPasswordFunc` injection (the mechanism is already in place, as shown by `TestSudoExec_SinglePromptMultiFile`).

---

### IN-03: `release-tag.md` confirmation plan (Step 4) shows `$CURRENT_TAG` in the replacement preview but Step 5 replaces by literal string — mismatch in user communication

**File:** `.claude/commands/gsd/release-tag.md:117-123`
**Issue:** Step 4 shows the user:
```
• Update README.md: s/$CURRENT_TAG/$NEXT_TAG/g
```
But Step 5 says "Replace **all** occurrences of the old version string with `$NEXT_TAG`". If the README contains the version string in multiple formats (e.g. `v0.9.3` and `0.9.3` without the `v` prefix), the script replaces all of them. The confirmation message underrepresents the scope of changes the user is about to approve.

**Fix:** Make the confirmation plan explicit about which strings will be replaced, e.g.:
```
• Update README.md: all occurrences of "v0.9.3" → "v0.9.4"
```

---

### IN-04: `loadSSHConfigKeys` ignores `user` parameter

**File:** `internal/ssh/client.go:185`
**Issue:** The function signature is `loadSSHConfigKeys(hostname, _ string)` — the `user` parameter is explicitly discarded. OpenSSH config supports `User` directives inside `Host` blocks, meaning different identity files can be configured per user on the same host. Currently the parser in `sshconfig.go` also ignores `User` directives. The underscore makes the intent clear (intentional deferral), but this means deployments using host-specific per-user SSH config will silently fall back to default key locations.

**Fix:** Either document this limitation explicitly (it is already partially noted in the `parseIdentityFiles` function) or implement `User`-aware parsing. No immediate action required if this is a known deferral.

---

_Reviewed: 2026-05-27T00:00:00Z_
_Reviewer: Claude (gsd-code-reviewer)_
_Depth: standard_
