---
phase: 13-cli-subcommands-deploy-ux
reviewed: 2026-05-26T00:00:00Z
depth: standard
files_reviewed: 9
files_reviewed_list:
  - internal/config/config_test.go
  - cmd/docker-deploy/main.go
  - cmd/docker-deploy/main_test.go
  - Makefile
  - .goreleaser.yaml
  - internal/filetransfer/upload.go
  - internal/filetransfer/upload_test.go
  - internal/preflight/checks.go
  - internal/preflight/checks_test.go
findings:
  critical: 3
  warning: 5
  info: 3
  total: 11
status: issues_found
---

# Phase 13: Code Review Report

**Reviewed:** 2026-05-26
**Depth:** standard
**Files Reviewed:** 9
**Status:** issues_found

## Summary

Phase 13 adds the `version` and `validate` subcommands, refactors sudo credential
handling into `SudoCreds`/`SudoExec`, adds a `test -w` path probe to detect
whether elevation is needed before deploying, adds verbose pre-confirm diff output
in `Upload()`, and wires verbose `sudo -l` output into `checkDockerGroup`.

The core logic is structurally sound. Three correctness bugs were found: a
deploy-cancelled sentinel that causes `docker compose up` and health polling to
execute on a user-declined deploy, a staging directory that is leaked (never
cleaned up) on the two confirm-prompt cancellation paths, and a deprecated
`os.IsNotExist` call that can misclassify errors on wrapped paths. Five
warnings cover a verbose `sudo -l` information-disclosure risk, incorrect
comment about `warnedOnce` in verbose mode, a race-prone `os.Stdin/os.Stderr`
global mutation pattern in tests, the Makefile hard-coding `buildTime` even on
`dev` builds (so the "omit Built: on dev" branch is never exercised), and
missing `.env` backup path validation.

---

## Critical Issues

### CR-01: Deploy-cancelled returns (0, nil) but `runDeploy` continues to run `docker compose up`

**File:** `internal/filetransfer/upload.go:417`, `internal/filetransfer/upload.go:422`

**Issue:** When the user declines the repeat-deploy confirmation prompt ("N" or
empty input), `Upload()` returns `(0, nil)` — zero files copied, no error.
`runDeploy` in `cmd/docker-deploy/main.go` only guards on `err != nil` (line 402).
It does not check the `fileCount == 0` sentinel, so execution continues past the
guard: `compose.RunCompose()` is called at line 413 and `health.PollHealth()` at
line 418. This causes `docker compose up` to run against whatever is already at
the remote path — or worse, against an empty directory if this was the first
deployment attempt — every time a user answers "N" at the prompt.

```go
// upload.go — current (lines 416-422)
fmt.Fprintln(os.Stderr, "No input received — deploy cancelled.")
return 0, nil   // <-- nil error; runDeploy does not see a cancellation
// ...
fmt.Fprintln(os.Stderr, "Deploy cancelled.")
return 0, nil   // <-- same problem
```

**Fix option A — use a sentinel error (preferred, cleanest):**
```go
// In upload.go — define a package-level sentinel:
var ErrDeployCancelled = errors.New("deploy cancelled by user")

// In Upload(), replace both `return 0, nil` with:
return 0, ErrDeployCancelled

// In runDeploy() (main.go, after the Upload call):
fileCount, err := filetransfer.Upload(...)
if errors.Is(err, filetransfer.ErrDeployCancelled) {
    return nil   // silent exit — user chose not to deploy
}
if err != nil {
    fmt.Fprintf(os.Stderr, "Deploy failed: %v\n", err)
    return err
}
```

**Fix option B — check fileCount:** Add `if fileCount == 0 { return nil }` in
`runDeploy` after the Upload guard, but this is ambiguous (a project with all files
excluded legitimately returns 0 — Upload already errors in that case, but future
code could confuse the two meanings).

---

### CR-02: Staging directory leaked on confirm-prompt cancellation

**File:** `internal/filetransfer/upload.go:416-422`

**Issue:** When the user cancels at the confirm prompt (either "N" or EOF path),
`Upload()` returns without removing `stagingDir` from `/tmp` on the remote server.
The staging directory was fully populated with all project files before the prompt
was shown (step 6 uploads first, then checks existence, then prompts). A leak of
`/tmp/docker-deploy-<nanoseconds>` on every declined deploy is not inherently
dangerous (world-readable `/tmp`), but repeated cancellations accumulate
gigabyte-scale staging directories containing `.env` files and other secrets on
the remote host.

```go
// upload.go — current (line 416-422)
fmt.Fprintln(os.Stderr, "No input received — deploy cancelled.")
return 0, nil   // staging dir NOT cleaned up

fmt.Fprintln(os.Stderr, "Deploy cancelled.")
return 0, nil   // staging dir NOT cleaned up
```

**Fix:** Clean up the staging directory before returning on both cancellation paths:
```go
fmt.Fprintln(os.Stderr, "No input received — deploy cancelled.")
_ = sshRun(client, fmt.Sprintf("rm -rf %s", ShellQuote(stagingDir)), nil)
return 0, ErrDeployCancelled  // (using the sentinel from CR-01)

// ...
fmt.Fprintln(os.Stderr, "Deploy cancelled.")
_ = sshRun(client, fmt.Sprintf("rm -rf %s", ShellQuote(stagingDir)), nil)
return 0, ErrDeployCancelled
```

---

### CR-03: `os.IsNotExist` is deprecated for wrapped errors — can silently pass through genuine file errors

**File:** `cmd/docker-deploy/main.go:148`

**Issue:** `os.IsNotExist(err)` does not unwrap error chains. Since Go 1.13 the
correct idiom is `errors.Is(err, fs.ErrNotExist)`. The standard library's
`os.Stat` returns a `*fs.PathError` whose `Err` field is `syscall.ENOENT`; when
that error is further wrapped (e.g. by a middleware or future refactor), `os.IsNotExist`
returns false and the condition falls through. The same pattern also appears in
`internal/config/config.go:156` (not in this review scope but present in the
package for reference). In the current code `os.Stat` is called directly so the
risk is low in practice, but the pattern is incorrect per Go's own docs and will
become a bug if the call is ever refactored to go through a wrapping layer.

```go
// main.go:148 — current
if _, err := os.Stat(filepath.Join(cwd, "deploy.yaml")); os.IsNotExist(err) {
```

**Fix:**
```go
import "errors"
import "io/fs"

if _, err := os.Stat(filepath.Join(cwd, "deploy.yaml")); errors.Is(err, fs.ErrNotExist) {
```

---

## Warnings

### WR-01: `sudo -l` output printed to stderr is an information disclosure in production

**File:** `internal/preflight/checks.go:280-282`

**Issue:** When `cfg.Verbose=true`, `checkDockerGroup` runs `sudo -l` on the
remote host and prints its full output to the caller's stderr:
```go
fmt.Fprintf(os.Stderr, "[sudo -l]\n%s\n", strings.TrimSpace(string(sudoLOut)))
```
`sudo -l` lists every sudoers rule the SSH user is permitted to run (often
including command patterns, hostnames, and NOPASSWD grants). In an environment
where the terminal output is logged (CI, Docker build logs, terminal recordings),
this exposes the remote host's privilege-escalation topology. An attacker who
gains access to those logs knows exactly which commands can be run with elevated
privileges without a password. This is an operator-consent issue: `--verbose`
implies "show me more diagnostics", not "dump my sudoers policy to any log sink".

**Fix:** Either (a) remove the `sudo -l` output entirely, or (b) gate it behind
a separate `--debug` flag that is clearly documented as producing sensitive output,
or (c) truncate/redact lines containing `NOPASSWD`:
```go
// Option: omit NOPASSWD lines
for _, line := range strings.Split(strings.TrimSpace(string(sudoLOut)), "\n") {
    if !strings.Contains(line, "NOPASSWD") {
        fmt.Fprintf(os.Stderr, "  %s\n", line)
    }
}
```

---

### WR-02: Doc comment for `Upload` incorrectly states `warnedOnce` is "never set to true" in verbose mode

**File:** `internal/filetransfer/upload.go:184`

**Issue:** The `Upload` function's doc comment says:
> When verbose=true: ... warnedOnce is never set to true so every sudo warning prints.

This is factually wrong. `SudoExec` unconditionally sets `*warnedOnce = true` on
the first interactive password attempt regardless of the `verbose` parameter
(lines 145-146). The verbose flag only controls whether the warning is printed
inline at that point; `warnedOnce` is still set. The caller in `runDeploy` checks
`*warnedOnce` after `Upload` returns (line 406) and appends to the rollup
`warnings` slice — in verbose mode this appends the rollup message even though
the warning was already printed inline, causing the warning to appear twice
(once inline, once in the rollup).

**Fix:** Correct the comment to accurately describe the behavior. If
double-emission in verbose mode is unintended, gate the rollup append:
```go
// runDeploy, after Upload():
if *warnedOnce && !resolved.Verbose {
    warnings = append(warnings, "WARNING: passwordless sudo not configured; ...")
}
```

---

### WR-03: Tests mutate global `os.Stdin`/`os.Stderr` without parallel-safety

**File:** `cmd/docker-deploy/main_test.go:239-249`, `internal/filetransfer/upload_test.go:433-441`, `internal/filetransfer/upload_test.go:892-895`

**Issue:** Multiple tests redirect `os.Stderr` and `os.Stdin` by directly
assigning to the global `os.Stderr`/`os.Stdin` package variables, then restore
them via `defer` or explicit reassignment. If `go test -parallel` is enabled or
tests within the package ever run concurrently (e.g. via `t.Parallel()`), these
global mutations race. The `TestValidateCmd_ValidConfig` test in `main_test.go`
is particularly fragile: it temporarily replaces `os.Stdout` (line 244), which
is a different file descriptor from the pipe created at line 240, and any other
goroutine writing to `os.Stdout` during that window will write to the pipe
instead of the real stdout.

**Fix:** Prefer capturing output by passing an `io.Writer` parameter to the
function under test (the `runVersionTo(w io.Writer)` pattern already used in
this codebase is the correct model). Alternatively, mark all tests that mutate
globals with `t.Setenv`-equivalent guards or document that the test file must
not be run with `-parallel`.

---

### WR-04: Makefile `build` target always sets `buildTime` — `dev` vs tagged branch in `runVersionTo` is untestable via `make build`

**File:** `Makefile:7`

**Issue:** The `build` target always injects a real timestamp:
```makefile
-X main.buildTime=$(shell date -u +%FT%TZ 2>/dev/null || echo unknown)
```
`runVersionTo` branches on `buildTime != "unknown"` to decide whether to print
the "Built:" line (main.go:107). The dev build path that omits "Built:" is only
reachable when `buildTime == "unknown"`, but `make build` never produces that
value. The branch is only exercised by the unit test in `main_test.go` which
manipulates the package-level variable directly. This means the actual `docker
deploy version` output from a `make build` binary always includes "Built:", even
for dev builds, contradicting the intended UX described in D-03.

**Fix:** Reserve `buildTime` injection for release builds only:
```makefile
build:
    mkdir -p bin
    go build -ldflags "-X main.version=dev \
        -X main.gitCommit=$(shell git rev-parse --short HEAD 2>/dev/null || echo unknown)" \
        -o bin/docker-deploy ./cmd/docker-deploy/
```
Leave `buildTime` as its zero value `"unknown"` for local `make build`. Only
`.goreleaser.yaml` (where `{{.Date}}` is always set for a tagged release) should
inject a real timestamp.

---

### WR-05: `.env` backup path in `/tmp` is constructed from the same `timestamp` as the staging dir — timestamp collision on same-second deploys is possible but the `.env` backup is never validated for path safety

**File:** `internal/filetransfer/upload.go:451`

**Issue:** The `.env` backup path is constructed as:
```go
envBackupPath = "/tmp/docker-deploy-env-" + timestamp
```
where `timestamp` is `fmt.Sprintf("%d", time.Now().UnixNano())` set at the top of
`Upload()`. The `/tmp/docker-deploy-<ts>` staging dir uses the same timestamp. If
by any chance two concurrent deploys race to millisecond precision (nanosecond
collision is unlikely but the OS only guarantees nanosecond resolution, not
uniqueness), the `.env` backup and the staging dir would have distinct names (one
uses `-env-` prefix) so they do not collide with each other, but a previous
uncleaned staging dir from a cancelled deploy (see CR-02) could collide with a
new staging dir. More practically, when the `.env` backup `cp` to `/tmp`
succeeds, there is no check that the backup file actually contains data before the
atomic swap proceeds; a zero-byte backup would silently wipe the remote `.env`.
The `cp` command does not fail for zero-byte source files.

**Fix:** After the `cp envPath → envBackupPath` succeeds, verify the backup
is non-empty:
```go
verifyCmd := fmt.Sprintf("test -s %s", ShellQuote(envBackupPath))
if verErr := sshRun(client, verifyCmd, nil); verErr != nil {
    fmt.Fprintf(os.Stderr, "WARNING: .env backup appears empty; .env will not be preserved\n")
    _ = sshRun(client, fmt.Sprintf("rm -f %s", ShellQuote(envBackupPath)), nil)
    envBackupPath = ""
}
```

---

## Info

### IN-01: Redundant double-print in `runValidate` — error is printed to stderr and then returned

**File:** `cmd/docker-deploy/main.go:159-161`, `165-167`

**Issue:** When `LoadFile` or `Resolve` returns an error, `runValidate` explicitly
prints it to stderr and then returns it:
```go
fmt.Fprintln(os.Stderr, err.Error())
return err
```
Cobra's default error handling in `RunE` also prints the error to stderr (unless
`SilenceErrors` is set). This results in the error message appearing twice in the
terminal output for the `validate` subcommand error case.

**Fix:** Either set `cmd.SilenceErrors = true` on the validate command, or remove
the explicit `fmt.Fprintln(os.Stderr, err.Error())` calls and let Cobra handle
printing. The existing `SilenceUsage = true` only suppresses the usage block, not
the error message.

---

### IN-02: `runDryRun` passes a hard-coded sentinel `"docker-compose.yml"` as `ComposeFile` to skip auto-detection — this is not obvious and is fragile

**File:** `cmd/docker-deploy/main.go:229`

**Issue:** The comment explains the intent: "sentinel: skips auto-detect; value
is unused in dry-run". However, `config.Resolve()` will behave incorrectly if a
future caller passes an empty `ComposeFile` flag — it would trigger auto-detection
which requires the local directory to have a compose file, making `--dry-run` fail
in projects that lack a local compose file. The sentinel is the right idea but
using an arbitrary real filename as the sentinel is fragile. If `Resolve()` ever
validates that the `ComposeFile` value exists locally, this sentinel will start
failing.

**Fix:** Add a dedicated `SkipComposeFileCheck bool` to `FlagOpts`, or allow
`config.Resolve` to accept an explicit `skipAutoDetect` parameter:
```go
// FlagOpts in config.go
type FlagOpts struct {
    // ...
    ComposeFile         string
    SkipComposeDetect   bool // set by dry-run: skip local compose file validation
}
```

---

### IN-03: `TestUploadAuthFallback_PasswordlessSudo` is structurally identical to `TestUploadAuthFallback_DirectCopy` — the test name is misleading

**File:** `internal/filetransfer/upload_test.go:310-328`

**Issue:** Both `TestUploadAuthFallback_DirectCopy` and
`TestUploadAuthFallback_PasswordlessSudo` use `newMockSSHServer(nil)` with no
`cmdExitCode` override, meaning all commands exit 0. The "passwordless sudo" test
never actually exercises the sudo fallback path — direct copy succeeds and sudo
is never invoked. The test name implies a fallback scenario is being validated but
it is observationally identical to the direct-copy test.

**Fix:** Set `srv.cmdExitCode` to fail direct commands while passing `sudo -n`
commands to actually exercise the passwordless sudo path:
```go
srv.cmdExitCode = func(cmd string, stdin []byte) uint32 {
    if strings.Contains(cmd, "test -w") { return 1 }   // force needsSudo
    if strings.Contains(cmd, "sudo -n") { return 0 }   // passwordless succeeds
    if strings.Contains(cmd, "sudo")    { return 1 }
    return 1  // direct commands fail
}
```

---

_Reviewed: 2026-05-26_
_Reviewer: Claude (gsd-code-reviewer)_
_Depth: standard_
