---
phase: 03-file-copy
reviewed: 2026-05-15T00:00:00Z
depth: standard
files_reviewed: 8
files_reviewed_list:
  - cmd/docker-deploy/main.go
  - internal/config/config.go
  - internal/config/config_test.go
  - internal/filetransfer/filter.go
  - internal/filetransfer/filter_test.go
  - internal/filetransfer/shellquote_test.go
  - internal/filetransfer/upload.go
  - internal/filetransfer/upload_test.go
findings:
  critical: 2
  warning: 4
  info: 2
  total: 8
status: issues_found
---

# Phase 03: Code Review Report

**Reviewed:** 2026-05-15
**Depth:** standard
**Files Reviewed:** 8
**Status:** issues_found

## Summary

This is a full standard-depth review of all Phase 3 source files: the main entry point, config package, and the filetransfer package (filter, upload, and their tests).

The ShellQuote injection fix from the prior gap-closure plan is correct and well-tested. The first-deploy mv-nesting bug fix is also correct and covered by a regression test.

Two blockers remain: the sudo password is leaked into error messages via the `sshExec` error format string, and remote file permissions are never set from the source file — executable scripts will silently lose their execute bit after upload. Four warnings cover the lazy-sudo rollback reliability issue (carried from the prior review), missing scanner error handling, lack of port range validation, and a double filesystem walk that can produce an inaccurate file count in the success message.

---

## Critical Issues

### CR-01: Sudo password exposed in error messages

**File:** `internal/filetransfer/upload.go:159` and `upload.go:243`

**Issue:** When a sudo-authenticated SSH command fails, the error returned by `sshExec` includes the full command string via `fmt.Errorf("running %q: %w", cmd, err)`. The `cmd` at line 159 is:

```
echo 'secretpassword' | sudo -S -p '' sh -c 'mv ...'
```

That error is returned to the caller and may be printed to stderr, logged by the calling tool, or written to CI/CD logs. Any error in the `sudoRun` closure that reaches line 159 (the case where `sudoPw` has already been collected and a subsequent op fails) will expose the cleartext password in the error chain.

Concrete path: if `mkdir -p` succeeds with sudo (so `sudoPw` is set), then `mv stagingDir remoteBase` fails, `sudoRun` at line 191 returns `sshExec(...)` which wraps the full command including the password in the error string. That error is then wrapped again at line 189 with `"renaming existing target to backup: ..."` and returned up the call stack.

**Fix:** Strip the password from the command before including it in the error. One approach is to have `sudoRun` produce its own error message rather than surfacing the raw `sshExec` error:

```go
// Line 159 — replace with a sanitized error:
if err := sshExec(client, fmt.Sprintf("echo %s | sudo -S -p '' sh -c %s",
    ShellQuote(sudoPw), ShellQuote(cmd))); err != nil {
    return fmt.Errorf("sudo command failed (password authenticated): %w",
        stripCommandFromError(err))
}

// Or more simply: wrap with a message that omits the command:
return fmt.Errorf("running command with sudo: exit status non-zero")
```

Alternatively, do not use `%q` in `sshExec`'s error for sudo-style commands. A separate `sshExecSudo` helper that sanitizes its error output would avoid exposing the secret:

```go
func sshExecSudo(client *gossh.Client, _ string) error {
    // same as sshExec but error says "sudo command failed" not the full cmd
}
```

---

### CR-02: Remote file permissions not preserved — executable bit silently lost

**File:** `internal/filetransfer/upload.go:93`

**Issue:** Remote files are created with `sftpClient.OpenFile(remotePath, os.O_WRONLY|os.O_CREATE|os.O_TRUNC)`. The `pkg/sftp` `OpenFile` function does not accept a mode parameter — it uses 0666 as the implicit creation mode, which the remote server's umask reduces to 0644 typically. The source file's permission bits are never read or applied to the remote file.

This means any executable file in the project — entrypoint shell scripts, migration scripts, `docker-entrypoint.sh`, Python CLI scripts, Go binaries — will be uploaded as 0644 and will fail to execute on the remote host. Docker will also fail to start containers whose entrypoints are non-executable. This is a silent data corruption: the content is correct but the file is broken.

**Fix:** Read the source file's mode before opening it, then apply it to the remote file after creation:

```go
// Get source file info for permissions.
localInfo, err := os.Stat(localPath)
if err != nil {
    return fmt.Errorf("stat local file %s: %w", localPath, err)
}

localFile, err := os.Open(localPath)
if err != nil {
    return fmt.Errorf("opening local file %s: %w", localPath, err)
}

remoteFile, err := sftpClient.OpenFile(remotePath, os.O_WRONLY|os.O_CREATE|os.O_TRUNC)
if err != nil {
    localFile.Close()
    return fmt.Errorf("creating remote file %s: %w", remotePath, err)
}

if _, err := io.Copy(remoteFile, localFile); err != nil {
    remoteFile.Close()
    localFile.Close()
    return fmt.Errorf("copying %s to remote: %w", relPath, err)
}
remoteFile.Close()
localFile.Close()

// Preserve source file permissions.
if err := sftpClient.Chmod(remotePath, localInfo.Mode().Perm()); err != nil {
    return fmt.Errorf("setting permissions on remote file %s: %w", remotePath, err)
}
```

---

## Warnings

### WR-01: Lazy-sudo rollback may hang or fail silently in non-interactive contexts

**File:** `internal/filetransfer/upload.go:130-196`

**Issue:** The `sudoRun` closure collects the sudo password lazily — only on the first command that fails without elevation. If the atomic swap's step 2 (`mv stagingDir remoteBase`, line 191) fails and `sudoPw` has not yet been set, the rollback at line 193 calls `sudoRun` which will prompt interactively for the password. In a CI/CD context where stdin is not a TTY, `term.ReadPassword` will fail or block indefinitely, and the blank-assigned `rbErr` at line 193 means rollback failure is silently swallowed.

The result: `remoteBase` is absent (the old contents are in `oldDir`), no actionable error message is produced, and the operator has no recovery path without inspecting the remote manually.

**Fix:** Either collect sudo credentials eagerly before the swap sequence, or change the rollback to use the non-sudo `sshExec` directly (best-effort) and emit a manual recovery command in the error:

```go
if err := sudoRun(fmt.Sprintf("mv %s %s", ShellQuote(stagingDir), ShellQuote(remoteBase))); err != nil {
    // Best-effort rollback without prompting (no new sudo prompt during error path).
    _ = sshExec(client, fmt.Sprintf("mv %s %s", ShellQuote(oldDir), ShellQuote(remoteBase)))
    return fmt.Errorf(
        "placing new version failed (rolled back where possible).\n"+
        "If rollback failed, restore manually:\n"+
        "  ssh %s 'sudo mv %s %s'\n"+
        "Original error: %w",
        client.RemoteAddr().String(), ShellQuote(oldDir), ShellQuote(remoteBase), err)
}
```

---

### WR-02: File count in success message can be wrong (double WalkFiles)

**File:** `cmd/docker-deploy/main.go:199-212`

**Issue:** `runDeploy` calls `filetransfer.WalkFiles` at line 199 to get `fileCount` for the success message, then calls `filetransfer.Upload` at line 206 which internally calls `WalkFiles` again. Between the two calls the filesystem can change (files added, removed, or excluded). The success message at line 212 may report a count that does not match what was actually uploaded.

Additionally this is a redundant filesystem walk that touches every non-excluded file twice.

**Fix:** Have `Upload` return the count of files it actually transferred, or expose the count through a helper that `runDeploy` can use without a second walk:

```go
// Option A: Upload returns count
n, err := filetransfer.Upload(ctx, client, cwd, resolved.Path, resolved.Excludes)
if err != nil { ... }
fmt.Fprintf(os.Stdout, "Deploy complete: %d files copied to ...\n", n)
```

---

### WR-03: `bufio.Scanner.Scan()` error not checked at confirmation prompt

**File:** `cmd/docker-deploy/main.go:188-193`

**Issue:** The replace-confirmation prompt calls `scanner.Scan()` without checking its return value or `scanner.Err()`. If stdin is closed, redirected from `/dev/null`, or signals EOF (e.g., in a Docker container or CI pipeline), `Scan()` returns `false`, `scanner.Text()` returns `""`, and the empty string correctly falls through to the "No" path — so the default behavior is safe. However, the I/O error is silently discarded. An operator who unknowingly runs the tool with stdin closed gets a silent "deploy cancelled" with no explanation.

**Fix:** Check `scanner.Err()` after `Scan()`:

```go
scanner := bufio.NewScanner(os.Stdin)
if !scanner.Scan() {
    if err := scanner.Err(); err != nil {
        return fmt.Errorf("reading confirmation: %w", err)
    }
    // EOF on stdin — treat as "No" but inform the user.
    fmt.Fprintln(os.Stderr, "No input received — deploy cancelled.")
    return nil
}
answer := strings.TrimSpace(scanner.Text())
```

---

### WR-04: Port number not validated for valid TCP range

**File:** `internal/config/config.go:80-85`

**Issue:** `ParseHost` validates the port string as an integer but does not check that it falls within the valid TCP port range (1–65535). A URL like `ssh://user@host:0` or `ssh://user@host:99999` would parse without error and produce a `Host` with `Port: 0` or `Port: 99999`. The `Port: 0` case is silently overridden to 22 by the callers in `main.go` (lines 86-88, 151-153), which hides the user's misconfiguration instead of reporting it. Port 99999 would reach the SSH dial and produce a confusing OS-level error.

**Fix:** Add a range check after `Atoi`:

```go
port, err = strconv.Atoi(portStr)
if err != nil {
    return Host{}, fmt.Errorf("invalid host URL %q: port %q is not a valid integer", rawURL, portStr)
}
if port < 1 || port > 65535 {
    return Host{}, fmt.Errorf("invalid host URL %q: port %d is out of range (1-65535)", rawURL, port)
}
```

---

## Info

### IN-01: `deploy.yaml` is included in the upload — may be unintentional

**File:** `internal/filetransfer/filter.go:18-20` and `internal/config/config.go:18-20`

**Issue:** The built-in exclude list (`defaultExcludes`) does not include `deploy.yaml`. This file contains the SSH host, target path, and optionally `force: true`. It is uploaded to the remote server as part of every deploy. On the remote, it serves no purpose and may expose internal infrastructure details (host addresses, paths) to any process or user with read access to the deployed directory.

**Suggestion:** Add `deploy.yaml` to `defaultExcludes`:

```go
var defaultExcludes = []string{
    ".git/", "node_modules/", "vendor/", "*.log", ".DS_Store", "__pycache__/",
    "deploy.yaml",
}
```

This is an info item because the project specification does not call out this exclusion explicitly, and some operators may intentionally want `deploy.yaml` on the remote for documentation purposes. The decision should be explicit.

---

### IN-02: `sftpClient.Close()` return value discarded on success path

**File:** `internal/filetransfer/upload.go:115`

**Issue:** `sftpClient.Close()` at line 115 (the success path, before SSH mv commands) discards its return value. An SFTP close failure can indicate that the in-flight request queue was not drained. In practice the individual `remoteFile.Close()` calls at line 106 handle file-level flushing, so this is low-risk. However, the error is entirely invisible.

**Suggestion:** Log or return the close error:

```go
if err := sftpClient.Close(); err != nil {
    fmt.Fprintf(os.Stderr, "Warning: closing SFTP session: %v\n", err)
}
```

---

_Reviewed: 2026-05-15_
_Reviewer: Claude (gsd-code-reviewer)_
_Depth: standard_
