---
phase: 03-file-copy
reviewed: 2026-05-14T00:00:00Z
depth: standard
files_reviewed: 6
files_reviewed_list:
  - cmd/docker-deploy/main.go
  - internal/config/config.go
  - internal/config/config_test.go
  - internal/filetransfer/filter.go
  - internal/filetransfer/filter_test.go
  - internal/filetransfer/upload.go
findings:
  critical: 2
  warning: 3
  info: 1
  total: 6
status: issues_found
---

# Phase 03: Code Review Report

**Reviewed:** 2026-05-14
**Depth:** standard
**Files Reviewed:** 6
**Status:** issues_found

## Summary

This phase adds the `filetransfer` package (`filter.go`, `upload.go`) and extends `config.go` with exclude and force fields. The SSH session model is followed correctly (one session per exec, SFTP wraps the existing client). Known-hosts verification is intact. The main structural concerns are: (1) `shellQuote` does not escape embedded single quotes, creating a command injection path for any path value containing a `'` character; (2) file permissions are silently dropped on upload, breaking executable scripts; (3) the atomic swap leaves the service directory absent if the second `mv` fails; (4) `WalkFiles` is called twice in `runDeploy`, creating a TOCTOU race between the count display and the actual upload.

---

## Critical Issues

### CR-01: `shellQuote` does not escape embedded single quotes — command injection on remote

**File:** `internal/filetransfer/upload.go:252-254`

**Issue:** `shellQuote` wraps a string in single quotes without escaping any `'` characters inside it. Every SSH command constructed in `Upload` (mkdir, mv, rm, test -d) and in `main.go:179` uses this function or an equivalent inline pattern. A path value containing a single quote — whether from the `--path` CLI flag, `deploy.yaml`, or the project directory basename — breaks out of the single-quoted context and injects arbitrary shell commands that execute on the remote server.

Demonstration: `remoteBase = "/opt/app'; rm -rf /opt; echo '"` produces:
```
mkdir -p '/opt/app'; rm -rf /opt; echo ''
```
This is a real injection vector: `deploy.yaml` is typically committed to a repository; if a malicious path value is committed, any team member running `docker deploy` executes the injected command on their server under their SSH credentials.

The same vulnerability applies to the inline format string in `main.go:179`:
```go
session.Output(fmt.Sprintf("test -d '%s' && echo exists || echo absent", resolved.Path))
```

**Fix:** Escape embedded single quotes before wrapping. In POSIX shell, a single quote inside a single-quoted string is expressed as `'\''`:

```go
func shellQuote(s string) string {
    // Replace each ' with '\'' to close, escape, and reopen the single-quoted string.
    escaped := strings.ReplaceAll(s, "'", `'\''`)
    return "'" + escaped + "'"
}
```

Also replace the inline format string in `main.go:179` with a call to `shellQuote` from the `filetransfer` package (export it) or duplicate the fix locally.

---

### CR-02: Atomic swap leaves `remoteBase` absent if the second `mv` fails

**File:** `internal/filetransfer/upload.go:184-191`

**Issue:** The three-step swap is:
1. `mv remoteBase oldDir` — removes the live directory
2. `mv stagingDir remoteBase` — installs the new version
3. `rm -rf oldDir` — removes the backup

If step 2 fails (network hiccup, permission error, disk full on remote), the code returns:
```go
return fmt.Errorf("renaming staging dir to target: %w", err)
```

At this point `remoteBase` no longer exists — the old production directory was renamed away in step 1 and nothing replaced it. The running docker-compose stack may still be functional (containers are already running), but any subsequent `docker compose up` or restart will fail because the compose file is gone from the expected location. The caller (`runDeploy`) prints "Deploy failed" and exits, leaving no staged files or the old dir in place.

**Fix:** If step 2 fails, attempt to roll back step 1 before returning:

```go
if err := sshExec(client, fmt.Sprintf("mv %s %s", shellQuote(stagingDir), shellQuote(remoteBase))); err != nil {
    // Attempt rollback: restore the old directory to remoteBase.
    _ = sshExec(client, fmt.Sprintf("mv %s %s", shellQuote(oldDir), shellQuote(remoteBase)))
    return fmt.Errorf("renaming staging dir to target (old dir restored if mv succeeded): %w", err)
}
```

At minimum, the error message should tell the operator where the backup is so they can recover manually:
```go
return fmt.Errorf(
    "renaming staging dir to target: %w\nOld directory is at %s on remote — restore with: mv %s %s",
    err, oldDir, oldDir, remoteBase,
)
```

---

## Warnings

### WR-01: File permissions not preserved — executable scripts lose `+x` on remote

**File:** `internal/filetransfer/upload.go:92`

**Issue:** `sftpClient.OpenFile(remotePath, os.O_WRONLY|os.O_CREATE|os.O_TRUNC)` sends an SSH `SSH_FXP_OPEN` packet with no `attrs` field. The remote server creates the file with its umask-derived permissions (typically `0644`). The local file's mode is never read. Executable files (`entrypoint.sh`, `healthcheck.sh`, custom scripts) lose their execute bit on the remote. Any docker-compose service that uses a shell script as its entrypoint will fail to start after the first deploy.

**Fix:** Read the local file's mode and apply it after upload using `sftpClient.Chmod`:

```go
localInfo, err := localFile.Stat()
if err != nil {
    localFile.Close()
    return fmt.Errorf("stat local file %s: %w", localPath, err)
}
// ... copy contents ...
remoteFile.Close()
localFile.Close()

if err := sftpClient.Chmod(remotePath, localInfo.Mode()); err != nil {
    return fmt.Errorf("setting permissions on %s: %w", remotePath, err)
}
```

---

### WR-02: `runDeploy` calls `WalkFiles` twice — TOCTOU race, wrong file count possible

**File:** `cmd/docker-deploy/main.go:199-206`

**Issue:** `runDeploy` calls `filetransfer.WalkFiles` at line 199 to obtain a count for the success message, then calls `filetransfer.Upload` at line 206 which internally calls `WalkFiles` again. Between the two calls, files on the local filesystem can change (e.g., a build process creates or deletes files). Consequences:

- The "Deploy complete: N files copied" count can differ from the actual number of files uploaded.
- If the filesystem changes such that `Upload`'s internal walk returns 0 files, `Upload` returns an error (`"no files to upload"`) even though the first walk succeeded.

**Fix:** Have `Upload` return the count of uploaded files, and remove the standalone `WalkFiles` call from `runDeploy`:

```go
// In upload.go, change Upload signature to return (int, error)
// Return len(files) on success.

// In main.go:
fileCount, err := filetransfer.Upload(context.Background(), client, cwd, resolved.Path, resolved.Excludes)
if err != nil {
    fmt.Fprintf(os.Stderr, "Deploy failed: %v\n", err)
    return err
}
fmt.Fprintf(os.Stdout, "Deploy complete: %d files copied to %s:%s\n", fileCount, resolved.Host.Hostname, resolved.Path)
```

---

### WR-03: `rm -rf oldDir` failure after a successful swap returns an error and misleads the operator

**File:** `internal/filetransfer/upload.go:190-191`

**Issue:** After both `mv` commands succeed (the new deployment is live), `rm -rf oldDir` is run to clean up the backup. If this cleanup fails, the code returns:
```go
return fmt.Errorf("removing backup dir: %w", err)
```
The caller in `runDeploy` receives this error, prints "Deploy failed:", and returns a non-zero exit code. The deployment itself **succeeded** — the new files are in `remoteBase` and the service can be restarted. The leftover `.old-<timestamp>` directory is harmless disk usage. Treating a cleanup failure as a deployment failure misleads the operator and breaks CI/CD pipelines that gate on exit code.

**Fix:** Demote this to a warning, log it, and return `nil`:

```go
if err := sshExec(client, fmt.Sprintf("rm -rf %s", shellQuote(oldDir))); err != nil {
    // Cleanup failure is non-fatal — deployment succeeded.
    fmt.Fprintf(os.Stderr, "Warning: could not remove backup dir %s: %v (deployment succeeded)\n", oldDir, err)
}
```

---

## Info

### IN-01: `ParseHost` accepts port `0` without error but `0` is not a valid TCP port

**File:** `internal/config/config.go:80-85`

**Issue:** `strconv.Atoi` succeeds for `"0"`, so `ssh://host:0` parses without error and returns `Port: 0`. The callers in `main.go` treat `port == 0` as "use default 22", which silently overrides an explicit `:0` that the user typed. Port `0` is not a valid SSH port.

**Fix:** Add a range check after `Atoi`:

```go
if port < 1 || port > 65535 {
    return Host{}, fmt.Errorf("invalid host URL %q: port %d out of range [1, 65535]", rawURL, port)
}
```

---

_Reviewed: 2026-05-14_
_Reviewer: Claude (gsd-code-reviewer)_
_Depth: standard_
