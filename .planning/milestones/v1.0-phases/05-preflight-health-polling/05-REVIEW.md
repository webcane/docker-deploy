---
phase: 05-preflight-health-polling
reviewed: 2026-05-17T00:00:00Z
depth: standard
files_reviewed: 7
files_reviewed_list:
  - internal/config/config.go
  - internal/config/config_test.go
  - cmd/docker-deploy/main.go
  - internal/preflight/checks.go
  - internal/preflight/checks_test.go
  - internal/health/poll.go
  - internal/health/poll_test.go
findings:
  critical: 1
  warning: 3
  info: 2
  total: 6
status: issues_found
---

# Phase 5: Code Review Report

**Reviewed:** 2026-05-17T00:00:00Z
**Depth:** standard
**Files Reviewed:** 7
**Status:** issues_found

## Summary

Phase 5 adds pre-flight checks (docker present, compose v2, daemon running, target dir writable, docker group membership, root-user warning) and health polling after `docker compose up`. The core logic is sound and the test coverage is thorough. However, one BLOCKER was found: `--dry-run` fails with "no compose file found" in any working directory that lacks a compose file, directly contradicting the code comment that says compose is not resolved for dry-run. Three warnings cover a silent cmd-argument bypass in the health poller session abstraction, a missing test for negative health timeout values in deploy.yaml, and a force-confirmation bypass when `resolved.Force` is true that skips the existence check entirely. Two info items cover dead variable usage and a code duplication.

---

## Critical Issues

### CR-01: `--dry-run` fails when no compose file exists locally

**File:** `cmd/docker-deploy/main.go:82`

**Issue:** `runDryRun` calls `config.Resolve` with an empty compose-file flag (`""`) and passes `cwd` as `localDir`. Inside `Resolve`, the compose-file resolution path falls through to auto-detect (lines 231-239 of `config.go`), which tries `os.Stat(filepath.Join(cwd, "compose.yaml"))` and `os.Stat(filepath.Join(cwd, "docker-compose.yml"))`. If neither file exists, `Resolve` returns `"no compose file found; use --compose-file to specify one"` and `runDryRun` aborts.

This contradicts the explicit comment on line 80: `// composeFile is not resolved for dry-run; validation happens in runDeploy` and the `_ = composeFile` discard on line 86. Dry-run is specifically intended to test SSH connectivity without requiring a full project layout. A user running `docker deploy --dry-run --host ssh://user@host` from an empty directory gets a misleading error about compose, not about SSH.

**Fix:** Pass a non-empty sentinel for `flagComposeFile` in the `runDryRun` call so that `Resolve` skips auto-detect, or restructure so `runDryRun` does not call `Resolve` for compose resolution at all:

```go
// Option A: pass a sentinel that bypasses auto-detect
resolved, err := config.Resolve(
    host, path, excludes, force,
    "docker-compose.yml", // sentinel: skips auto-detect; value is unused in dry-run
    0, 0, fileConfig, projectName, cwd,
)

// Option B (cleaner): split Resolve into HostResolve + ComposeResolve,
// so dry-run only calls HostResolve. Requires a signature change.
```

Option A is a minimal fix that avoids the error. Option B is cleaner but requires more refactoring.

---

## Warnings

### WR-01: `sessionOutput.Output(cmd)` silently ignores its argument in production

**File:** `internal/health/poll.go:57-59`

**Issue:** `sshSessionWrapper.Output` ignores the `cmd` parameter and always executes `w.cmd`, which was baked in at `newSession(cmd)` construction time:

```go
func (w *sshSessionWrapper) Output(_ string) ([]byte, error) {
    out, err := w.session.Output(w.cmd)  // w.cmd, not the argument
    return out, err
}
```

Both `listContainers` (line 168) and `inspectHealth` (line 246) pass `cmd` to `session.Output(cmd)` — the same value that was passed to `newSession(cmd)` — so in the current call sites this is harmless. But the `sessionOutput` interface declares `Output(cmd string) ([]byte, error)`, implying the argument controls what runs. Any future caller who tries `session.Output("different command")` will silently execute the wrong command with no error.

This is a latent correctness hazard. The `fakeSession` in tests also ignores `cmd`, which means tests cannot catch this class of bug.

**Fix:** Remove `cmd` from the `sessionOutput` interface and from `sshSessionWrapper.Output`. Make the session a one-shot executor:

```go
type sessionOutput interface {
    Output() ([]byte, error)  // no cmd argument — command is baked in at newSession()
    Close() error
}

func (w *sshSessionWrapper) Output() ([]byte, error) {
    return w.session.Output(w.cmd)
}

// Callers:
out, err := session.Output()  // not session.Output(cmd)
```

Update `fakeSession.Output()` in `poll_test.go` to match.

### WR-02: Negative `health_timeout` / `health_interval` in deploy.yaml accepted silently

**File:** `internal/config/config.go:245-261`

**Issue:** Both `HealthTimeout` and `HealthInterval` use `> 0` to gate whether a file value is applied:

```go
case file.Target.HealthTimeout > 0:
    cfg.HealthTimeout = file.Target.HealthTimeout
```

A user who writes `health_timeout: -30` in `deploy.yaml` has the value silently discarded and the default 60s applied, with no warning. The config comment (line 190) documents this as intentional (`T-05-01-01`), but `yaml.Unmarshal` will happily parse negative integers without complaint. The user gets no feedback that their configuration value was rejected.

**Fix:** Validate and return an error (or at minimum warn) when a negative value is present:

```go
if file.Target.HealthTimeout < 0 {
    return Config{}, fmt.Errorf("deploy.yaml: health_timeout must be >= 0, got %d", file.Target.HealthTimeout)
}
```

Alternatively, treat `< 0` the same as `0` but log a warning to `os.Stderr` so the user knows their value was ignored.

### WR-03: `checkSudo` error message exposes username in sudoers suggestion without sanitization

**File:** `internal/preflight/checks.go:207-215`

**Issue:** `checkSudo` formats `cfg.Host.User` directly into the error message:

```go
return fmt.Errorf(
    "preflight: no passwordless sudo available for user %s; "+
        "fix: add '%s ALL=(ALL) NOPASSWD: ALL' to /etc/sudoers.d/%s on the remote host",
    cfg.Host.User, cfg.Host.User, cfg.Host.User,
)
```

The username comes from the SSH URL parsed by `ParseHost`. `ParseHost` does not validate the username for shell-safe characters. A username like `alice; rm -rf /` in a malicious `deploy.yaml` would produce a confusing error message that might mislead the operator. Additionally, `cfg.Host.User` is used unquoted in the suggested fix command shown in the error string — operators who blindly copy-paste the suggestion could execute an injection if the username is malicious.

The risk is limited because this is an error message, not a command executed on the remote, but the trust boundary is `deploy.yaml` which may be written by an adversary in a shared CI context.

**Fix:** Add username validation in `ParseHost` (or `Resolve`) to reject usernames containing characters outside `[a-zA-Z0-9._-]`:

```go
// In ParseHost, after extracting user:
if user != "" {
    if !isValidUnixUsername(user) {
        return Host{}, fmt.Errorf("invalid host URL %q: username %q contains disallowed characters", rawURL, user)
    }
}
```

---

## Info

### IN-01: Dead variable `composeFile` in `runDryRun`

**File:** `cmd/docker-deploy/main.go:86`

**Issue:** The parameter `composeFile string` is accepted by `runDryRun` (line 65) but immediately discarded with `_ = composeFile` (line 86). The function signature accepts it for API symmetry with `runDeploy`, but there is no documentation explaining this, and the `cobra` flag `--compose-file` silently does nothing during `--dry-run`. A user passing `--dry-run --compose-file foo.yaml` would not get an error, but the value is ignored.

**Fix:** Either document clearly in the flag help text that `--compose-file` is ignored during `--dry-run`, or remove the parameter from `runDryRun`'s signature and have the `cobra` `RunE` handler not pass it:

```go
// In the cobra handler:
if dryRun {
    return runDryRun(host, path, excludes, force)
    // composeFile intentionally not passed — dry-run does not deploy
}
```

### IN-02: `fakeSSHClient` in `checks_test.go` mutates shared slice during iteration

**File:** `internal/preflight/checks_test.go:43-56`

**Issue:** `fakeSession.Output` modifies `s.parent.cmds` while the `for i, fc := range` loop is running:

```go
for i, fc := range s.parent.cmds {
    if strings.Contains(cmd, fc.match) {
        s.parent.cmds = append(s.parent.cmds[:i], s.parent.cmds[i+1:]...)  // mutation mid-loop
        ...
    }
}
```

Go's `range` over a slice copies the slice header at loop entry, but the underlying array is shared. The `append` reslice shortens `s.parent.cmds` in place. In the current tests this works because `Output` returns immediately after the first match (the mutation happens only once per call and the loop exits). However if a future test or code path calls `Output` reentrantly (e.g., from a goroutine) this is a data race. It is also fragile: if the match is at index 0, the `append(cmds[:0], cmds[1:]...)` is safe, but if the loop were to continue after the match (which it currently does not due to `return`), elements would be skipped.

**Fix:** Break out of the loop explicitly after the match, making the intent clear:

```go
for i, fc := range s.parent.cmds {
    if strings.Contains(cmd, fc.match) {
        s.parent.matched = append(s.parent.matched, cmd)
        s.parent.cmds = append(s.parent.cmds[:i], s.parent.cmds[i+1:]...)
        if fc.exitCode != 0 {
            return nil, &gossh.ExitError{Waitmsg: gossh.Waitmsg{}}
        }
        return fc.output, nil  // already returns — but make the break explicit if refactoring
    }
}
```

The current code already returns inside the `if` block so there is no actual loop-continuation bug today. The risk is if the `if` body is ever restructured. A `break` after the mutation would make the safety explicit.

---

_Reviewed: 2026-05-17T00:00:00Z_
_Reviewer: Claude (gsd-code-reviewer)_
_Depth: standard_
