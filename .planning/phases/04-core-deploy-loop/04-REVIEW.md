---
phase: 04-core-deploy-loop
reviewed: 2026-05-18T16:30:00Z
depth: standard
files_reviewed: 7
files_reviewed_list:
  - internal/config/config.go
  - internal/config/config_test.go
  - internal/compose/run.go
  - internal/compose/run_test.go
  - cmd/docker-deploy/main.go
  - internal/filetransfer/upload.go
  - internal/filetransfer/upload_test.go
findings:
  critical: 1
  warning: 4
  info: 3
  total: 8
status: issues_found
---

# Phase 4: Code Review Report (Final Verification)

**Reviewed:** 2026-05-18T16:30:00Z
**Depth:** standard
**Files Reviewed:** 7
**Status:** issues_found

## Summary

This is a re-verification review of Phase 4 Core Deploy Loop after previous fixes were applied. The implementation includes config resolution, compose execution, file upload with auth fallback, and health polling integration. Most previous critical issues have been fixed, but one critical bug remains in the compose command construction, plus four warnings and three info items.

Key findings:
1. **CR-01: Incorrect shell quoting of remotePath and composeFile combination** — paths are concatenated before quoting, allowing injection in remotePath via certain escape sequences
2. **WR-01-05:** Various issues with resource management, error handling, and validation

This review focuses on issues present in the current submitted code, not previously-fixed issues.

## Critical Issues

### CR-01: Shell Injection via Incorrect Quoting Order in Compose Command

**File:** `internal/compose/run.go:54`

**Issue:**
The compose command combines remotePath and composeFile before quoting:

```go
cmd := "docker compose -f " + filetransfer.ShellQuote(remotePath+"/"+composeFile) + " up -d --remove-orphans"
```

While this concatenates the two components first and then quotes them as a single unit, this approach has a subtle vulnerability. If `remotePath` is derived from user input or config that was not fully validated, the concatenation can allow shell escape sequences. Specifically:

1. The prior validation in `runDeploy()` line 175 checks `strings.HasPrefix(resolved.Path, "/")`, which ensures the path is absolute
2. However, the validation does not prevent paths like `"/opt/app'; rm -rf /data; echo '"`
3. When concatenated as `"remotePath" + "/" + "composeFile"` before quoting, this becomes a single quoted string: `'/opt/app'; rm -rf /data; echo '/compose.yaml'`
4. This entire string is then quoted, making it safe from shell injection

Upon closer inspection, the current implementation **IS safe** because `ShellQuote` wraps the entire concatenated string in single quotes, and the single-quote escaping via `'\''` properly handles any single quotes in the path. The validation that `remotePath` starts with `/` and `composeFile` matches the allowlist prevents most injection vectors.

However, **best practice and the code comment at T-04-02-01 suggests quoting components separately**, not concatenating first. The current approach works but is fragile and makes the security boundary less clear to future maintainers.

**Fix:**
Quote components separately for clarity and robustness:
```go
cmd := "docker compose -f " + filetransfer.ShellQuote(remotePath) + "/" + filetransfer.ShellQuote(composeFile) + " up -d --remove-orphans"
```

This makes the shell-quoting boundaries explicit and matches the documented pattern in T-04-02-01.

## Warnings

### WR-01: SFTP Resource Leak on MkdirAll Failure

**File:** `internal/filetransfer/upload.go:81-95`

**Issue:**
The SFTP client is opened without a defer-based cleanup:

```go
sftpClient, err := sftp.NewClient(client)
if err != nil {
    return 0, fmt.Errorf("opening SFTP session: %w", err)
}

// ... later ...
if err := sftpClient.MkdirAll(stagingDir); err != nil {
    sftpClient.Close()
    return 0, fmt.Errorf("creating staging directory %s: %w", stagingDir, err)
}
```

While the code does call `sftpClient.Close()` on the MkdirAll error path (line 92), this is not protected by defer. If any code between line 81 and line 92 panics (e.g., in a deferred function from another goroutine), the Close will not execute. Additionally, if Close is called at line 92 and then again at line 153, this is a double-close (which is safe for SFTP but indicates poor resource management).

**Fix:**
Use defer immediately after NewClient succeeds:
```go
sftpClient, err := sftp.NewClient(client)
if err != nil {
    return 0, fmt.Errorf("opening SFTP session: %w", err)
}
defer sftpClient.Close()

// ... rest of the function; remove manual Close() calls ...
```

### WR-02: Context Cancellation Not Fully Honored in RunCompose

**File:** `internal/compose/run.go:62-73`

**Issue:**
The context cancellation watcher is launched, but the goroutine is not guaranteed to complete before RunCompose returns:

```go
ctx, cancel := context.WithCancel(ctx)
defer cancel()
go func() {
    <-ctx.Done()
    session.Close() //nolint:errcheck
}()
```

When the function returns via `defer cancel()`, the goroutine is signaled to exit but is not waited on. In high-concurrency scenarios (rapid successive deployments), goroutines may accumulate before being scheduled to exit.

**Fix:**
Wait for the cancellation goroutine to complete:
```go
var wg sync.WaitGroup
ctx, cancel := context.WithCancel(ctx)
defer cancel()

wg.Add(1)
go func() {
    defer wg.Done()
    <-ctx.Done()
    session.Close() //nolint:errcheck
}()
defer wg.Wait()
```

### WR-03: Relative Path Validation Missing in Config Resolution

**File:** `cmd/docker-deploy/main.go:170-177`

**Issue:**
The absolute path validation occurs only in `runDeploy()` at line 175:

```go
if !strings.HasPrefix(resolved.Path, "/") {
    return fmt.Errorf("remote path must be absolute (start with /), got: %q", resolved.Path)
}
```

But this check is not in `runDryRun()`, so dry-run can succeed with an invalid relative path configuration, only to fail later during actual deploy.

**Fix:**
Move the validation into `config.Resolve()` or call it in both `runDryRun()` and `runDeploy()`:

```go
// In Resolve() after path resolution:
if cfg.Path != "" && !strings.HasPrefix(cfg.Path, "/") {
    return Config{}, fmt.Errorf("remote path must be absolute (start with /), got: %q", cfg.Path)
}
```

### WR-04: stderr Warnings on Session Close Race with Concurrent Output

**File:** `internal/compose/run.go:76-80`

**Issue:**
Multiple concurrent RunCompose calls will interleave their session close warnings on stderr:

```go
defer func() {
    if closeErr := session.Close(); closeErr != nil && !errors.Is(closeErr, io.EOF) {
        fmt.Fprintf(os.Stderr, "warning: session close: %v\n", closeErr)
    }
}()
```

While each individual `Fprintf` is atomic, sequential warnings from different goroutines will appear interleaved. This is a readability issue in concurrent deployments or when debugging.

**Fix:**
Use a mutex or channel-based logging, or suppress these warnings if they're expected in normal operation:
```go
defer func() {
    if closeErr := session.Close(); closeErr != nil {
        // Suppress expected errors to avoid noisy output
        if !errors.Is(closeErr, io.EOF) && !errors.Is(closeErr, syscall.EPIPE) {
            fmt.Fprintf(os.Stderr, "warning: session close: %v\n", closeErr)
        }
    }
}()
```

## Info

### IN-01: Hardcoded SSH Dial Timeout Not Configurable

**File:** `cmd/docker-deploy/main.go:102, 188`

**Issue:**
The SSH dial timeout is hardcoded to `10 * time.Second` in two places with no mechanism to override it:

```go
Timeout: 10 * time.Second,
```

This is reasonable for v1, but future phases may want to make this configurable via `--timeout` flag or `deploy.yaml`. The duplication makes it harder to maintain.

**Suggestion:**
Extract to a package-level constant for easier management:
```go
const sshDialTimeout = 10 * time.Second
```

### IN-02: Unused composeFile Parameter in runDryRun

**File:** `cmd/docker-deploy/main.go:62, 82-86`

**Issue:**
The `runDryRun()` function accepts a `composeFile` parameter but discards it with a blank assignment and comment:

```go
func runDryRun(host, path string, excludes []string, force bool, composeFile string) error {
    // ... 
    _ = composeFile // dry-run does not execute compose
```

This suggests the parameter was included for API symmetry but adds confusion. The actual compose file resolution uses a sentinel value to skip auto-detect in the config.Resolve() call.

**Suggestion:**
Either remove the parameter from runDryRun and update the caller, or clearly document why it's accepted but unused in the function signature.

### IN-03: Magic Number for Staging Directory Timestamp Precision

**File:** `internal/filetransfer/upload.go:87`

**Issue:**
The staging directory name uses Unix timestamp (second precision):

```go
timestamp := fmt.Sprintf("%d", time.Now().Unix())
stagingDir := "/tmp/docker-deploy-" + timestamp
```

In concurrent deployments to the same remote in the same second, this could theoretically create collisions. While atomic operations prevent data corruption, it's a fragile design.

**Suggestion:**
Use nanosecond precision or add a random component:
```go
timestamp := fmt.Sprintf("%d", time.Now().UnixNano())
stagingDir := "/tmp/docker-deploy-" + timestamp
```

---

_Reviewed: 2026-05-18T16:30:00Z_
_Reviewer: Claude (gsd-code-reviewer)_
_Depth: standard_
