# Phase 13: CLI Subcommands & Deploy UX - Pattern Map

**Mapped:** 2026-05-26
**Files analyzed:** 7 modified files (no new files except test additions)
**Analogs found:** 7 / 7

## File Classification

| Modified File | Role | Data Flow | Closest Analog | Match Quality |
|---------------|------|-----------|----------------|---------------|
| `cmd/docker-deploy/main.go` | CLI entry / command builder | request-response | self (existing subcommand pattern in `buildDeployCmd`) | exact |
| `internal/filetransfer/upload.go` (Plan 13-04 SudoExec) | service / SSH exec | request-response | `internal/filetransfer/upload.go` `sshExec`/`sshExecWithSudoPassword` | exact |
| `internal/filetransfer/upload.go` (Plan 13-05 verbose diff) | service / UX output | request-response | `internal/filetransfer/upload.go` verbose per-file block (lines 163-167) | exact |
| `internal/filetransfer/upload.go` (Plan 13-06 path probe) | service / SSH exec | request-response | `internal/preflight/checks.go` `checkTargetDir` `test -w` pattern | role-match |
| `internal/preflight/checks.go` (Plan 13-07 sudo -l) | service / SSH exec | request-response | `internal/preflight/checks.go` `checkDockerGroup` verbose pattern | exact |
| `Makefile` (Plan 13-02 ldflags) | config / build system | transform | `Makefile` existing build target | exact |
| `.goreleaser.yaml` (Plan 13-02 ldflags) | config / build system | transform | `.goreleaser.yaml` existing ldflags block | exact |

## Pattern Assignments

### `cmd/docker-deploy/main.go` — Plans 13-01, 13-02, 13-03 (version + validate subcommands, ldflags vars)

**Analog:** `cmd/docker-deploy/main.go` — `buildDeployCmd()` and `runDryRun()`/`runDeploy()` patterns

**Existing ldflags var pattern** (line 26):
```go
var version = "dev"
```
New vars extend this block in the same style:
```go
var version   = "dev"
var gitCommit = "unknown"
var buildTime = "unknown"
```

**Subcommand registration pattern** — `buildDeployCmd()` returns `*cobra.Command`; new subcommands follow the same constructor convention and are registered via `cmd.AddCommand()` inside `buildDeployCmd()` (lines 46-77). The function signature convention is:
```go
func buildVersionCmd() *cobra.Command {
    return &cobra.Command{
        Use:          "version",
        Short:        "Print version information",
        SilenceUsage: true,
        RunE: func(cmd *cobra.Command, args []string) error {
            return runVersion()
        },
    }
}
```

**cwd-relative config loading pattern** (lines 112-123, `runDryRun`; duplicated at lines 189-199, `runDeploy`):
```go
cwd, err := os.Getwd()
if err != nil {
    return fmt.Errorf("getting working directory: %w", err)
}
projectName := filepath.Base(cwd)
fileConfig, err := config.LoadFile(cwd)
if err != nil {
    return fmt.Errorf("loading deploy.yaml: %w", err)
}
```
The `validate` subcommand reuses this exact sequence, then calls `config.Resolve()` with zero `FlagOpts{}`.

**Error output conventions** — warnings and errors go to `os.Stderr`; success summary lines go to `os.Stdout`. Example (lines 163-179):
```go
fmt.Fprintf(os.Stderr, "SSH connection failed: %v\n", err)   // error → stderr
fmt.Fprintf(os.Stdout, "Dry-run: connectivity check passed\n") // success → stdout
```
`validate` success: `fmt.Fprintln(os.Stdout, "✓ deploy.yaml is valid")`. Errors: `fmt.Fprintln(os.Stderr, err.Error())`.

**SudoCreds lifecycle at call site** (lines 335-344) — after Plan 13-04, `main.go` replaces `sudoPw := new(string)` with:
```go
creds := new(filetransfer.SudoCreds)
defer creds.Zero()
warnedOnce := new(bool)
fileCount, err := filetransfer.Upload(context.Background(), client, cwd, resolved.Path, resolved.Excludes, creds, warnedOnce, resolved.Verbose)
```
`warnedOnce` rollup at lines 342-344 survives unchanged — only `sudoPw` is replaced.

---

### `internal/filetransfer/upload.go` — Plan 13-04 (SudoExec refactor, sshRun merge)

**Analog:** `internal/filetransfer/upload.go` — `sshExec` (lines 392-403) + `sshExecWithSudoPassword` (lines 408-430) + `sudoRunWithFallback` closure (lines 222-288)

**sshExec pattern — the new `sshRun` base** (lines 392-403):
```go
func sshExec(client *gossh.Client, cmd string) error {
    session, err := client.NewSession()
    if err != nil {
        return fmt.Errorf("creating SSH session: %w", err)
    }
    defer session.Close() //nolint:errcheck

    if err := session.Run(cmd); err != nil {
        return fmt.Errorf("running %q: %w", cmd, err)
    }
    return nil
}
```
`sshRun(client, cmd string, pw []byte) error` merges this with `sshExecWithSudoPassword`: when `pw == nil`, use `session.Run(cmd)`; when `pw != nil`, use `sudo -S -p '' sh -c <cmd>` with stdin pipe.

**sshExecWithSudoPassword pattern — the sudo branch of `sshRun`** (lines 408-430):
```go
func sshExecWithSudoPassword(client *gossh.Client, pw, cmd string) error {
    session, err := client.NewSession()
    // ...
    defer session.Close() //nolint:errcheck

    stdin, err := session.StdinPipe()
    // ...
    sudoCmd := fmt.Sprintf("sudo -S -p '' sh -c %s", ShellQuote(cmd))
    if err := session.Start(sudoCmd); err != nil { ... }
    _, _ = fmt.Fprintln(stdin, pw)
    _ = stdin.Close()
    if err := session.Wait(); err != nil { ... }
    return nil
}
```
In `sshRun`, `pw` is `[]byte` not `string`; write with `stdin.Write(append(pw, '\n'))` not `fmt.Fprintln(stdin, pw)` to avoid string conversion.

**SudoCreds type (new — from CONTEXT.md D-12)**:
```go
type SudoCreds struct{ pw []byte }
func (c *SudoCreds) Zero() { for i := range c.pw { c.pw[i] = 0 }; c.pw = nil }
```
Place at the top of `upload.go` alongside `ShellQuote`.

**sudoRunWithFallback closure — becomes exported `SudoExec`** (lines 222-288). The step order from the closure is preserved; insert cached-password step (D-11) as step 2 between direct and `sudo -n`:
```go
func SudoExec(client *gossh.Client, cmd string, creds *SudoCreds, warnedOnce *bool, verbose bool) error {
    // Step 1: direct
    if verbose { fmt.Fprintf(os.Stderr, "[ssh] %s\n", cmd) }
    if sshRun(client, cmd, nil) == nil {
        if verbose { fmt.Fprintf(os.Stderr, "  → exit 0\n") }
        return nil
    }
    if verbose { fmt.Fprintf(os.Stderr, "  → exit 1 (direct copy failed, trying sudo)\n") }

    // Step 2: cached password (avoids redundant round-trip on calls 2+)
    if creds.pw != nil {
        if sshRun(client, cmd, creds.pw) == nil { return nil }
    }

    // Step 3: passwordless sudo
    sudoCmd := fmt.Sprintf("sudo -n sh -c %s", ShellQuote(cmd))
    if verbose { fmt.Fprintf(os.Stderr, "[ssh] %s\n", sudoCmd) }
    if sshRun(client, sudoCmd, nil) == nil {
        if verbose { fmt.Fprintf(os.Stderr, "  → exit 0\n") }
        return nil
    }
    if verbose { fmt.Fprintf(os.Stderr, "  → exit 1 (passwordless sudo failed)\n") }

    // Step 4: interactive password (up to 3 attempts) — warnedOnce pattern from closure lines 258-263
    if !*warnedOnce {
        *warnedOnce = true
        if verbose { fmt.Fprintf(os.Stderr, "WARNING: passwordless sudo not configured...\n") }
    }
    for attempt := 1; attempt <= 3; attempt++ {
        // promptSudoPassword() — existing function (lines 34-42)
        pw, readErr := promptSudoPassword()
        if readErr != nil { return readErr }
        if sshRun(client, cmd, []byte(pw)) == nil {
            creds.pw = []byte(pw)
            return nil
        }
        if attempt < 3 { fmt.Fprintln(os.Stderr, "Sorry, try again.") }
    }
    return fmt.Errorf("could not write to target directory — no valid auth path available ...")
}
```
**Critical (D-15 / `feedback_sudo_rollback.md`):** All rollback/error paths in `Upload()` that touch `remoteBase` or its siblings MUST call `SudoExec`, not bare `sshRun`. Replace every `sudoRunWithFallback(...)` call (lines 291, 316, 322, 325, 344, 353, 356, 366) with `SudoExec(client, ..., creds, warnedOnce, verbose)`.

---

### `internal/filetransfer/upload.go` — Plan 13-05 (verbose pre-confirm file diff)

**Analog:** `internal/filetransfer/upload.go` — existing verbose per-file block (lines 163-167) + `sshExecOutput` for remote reads

**Existing verbose output pattern** (lines 163-167):
```go
if verbose {
    fmt.Fprintf(os.Stderr, "  -> %s\n", relPath)
}
```
Pre-confirm diff follows the same `if verbose` guard and `os.Stderr` destination.

**existsBefore variable** (line 185) — already computed before the confirm prompt fires; use it to gate `ReadDir` vs. "(none)":
```go
// This block runs inside Upload() after sftpClient is open (step 3) and after
// WalkFiles (step 1) but before the confirm prompt and before any SudoExec calls.
// CONTEXT.md D-17 through D-20.
if verbose && !force {
    fmt.Fprintf(os.Stderr, "Local files (%d):\n", len(files))
    for i, f := range files {
        if i >= 20 {
            fmt.Fprintf(os.Stderr, "  ... and %d more — consider adding patterns to deploy.yaml exclude list\n", len(files)-20)
            break
        }
        fmt.Fprintf(os.Stderr, "  %s\n", f)
    }
    if existsBefore {
        remoteFiles, rdErr := sftpClient.ReadDir(remoteBase)
        if rdErr != nil {
            fmt.Fprintf(os.Stderr, "Remote files: (unable to list: %v)\n", rdErr)
        } else {
            fmt.Fprintf(os.Stderr, "Remote files (%d):\n", len(remoteFiles))
            for i, fi := range remoteFiles {
                if i >= 20 {
                    fmt.Fprintf(os.Stderr, "  ... and %d more — consider adding patterns to deploy.yaml exclude list\n", len(remoteFiles)-20)
                    break
                }
                fmt.Fprintf(os.Stderr, "  %s\n", fi.Name())
            }
        }
    } else {
        fmt.Fprintln(os.Stderr, "Remote files: (none)")
    }
}
```
**Architectural note (RESEARCH.md Pitfall 7):** The confirm prompt currently lives in `main.go` (lines 295-328) and fires before `Upload()` is called. The verbose diff needs both SFTP (open inside `Upload()`) and `WalkFiles` (also inside `Upload()`). The correct resolution per CONTEXT.md `code_context`: move the confirm prompt logic inside `Upload()`, passing `force bool` as a parameter. `Upload()` already receives `force` indirectly via `resolved.Force` — add it as an explicit parameter in Plan 13-05. The existence check session at `main.go:296-304` becomes unnecessary once the confirm prompt moves into `Upload()`.

---

### `internal/filetransfer/upload.go` — Plan 13-06 (path-aware sudo detection)

**Analog:** `internal/preflight/checks.go` — `checkTargetDir` `test -w` pattern (lines 219-262)

**test -w probe pattern from checkTargetDir** (lines 223-229):
```go
path := filetransfer.ShellQuote(cfg.Path)
if err := runCmd(client, "test -w "+path); err == nil {
    // path is writable — no sudo needed
}
```
In `Upload()` (post 13-04), the probe uses the unified `sshRun` with `nil` password and a two-part OR per CONTEXT.md D-22/Pitfall 5:
```go
// Probe: test -w <remoteBase> || test -w <parent>
// Use path.Dir (not filepath.Dir) — remote is always Linux.
probeCmd := fmt.Sprintf("test -w %s || test -w %s",
    ShellQuote(remoteBase), ShellQuote(path.Dir(remoteBase)))
needsSudo := sshRun(client, probeCmd, nil) != nil
```
When `needsSudo == false`, replace all `SudoExec(...)` calls with `sshRun(client, cmd, nil)`. When `needsSudo == true`, proceed with full `SudoExec` fallback chain as normal.

**ShellQuote usage** (line 452) — always wrap remote paths:
```go
func ShellQuote(s string) string {
    return "'" + strings.ReplaceAll(s, "'", "'\\''") + "'"
}
```

**`path` vs `filepath` for remote paths** — `path.Dir(remoteBase)` not `filepath.Dir(remoteBase)`. The remote is always Linux. Local ops use `filepath`; remote path ops use `path`. See upload.go imports (line 8-9) — both `"path"` and `"path/filepath"` are imported.

---

### `internal/preflight/checks.go` — Plan 13-07 (verbose sudo -l in CHECK-04)

**Analog:** `internal/preflight/checks.go` — `checkDockerGroup` function (lines 265-302) and `runOutput` helper (lines 310-317)

**checkDockerGroup pattern** (lines 265-302) — this is CHECK-04; Plan 13-07 adds the `sudo -l` block inside it:
```go
func checkDockerGroup(client SSHRunner, cfg config.Config) (CheckResult, error) {
    out, err := runOutput(client, "id -nG")
    // ... existing logic ...
}
```
`cfg.Verbose` is accessible here — no signature change needed. The `sudo -l` block inserts after the initial `id -nG` call succeeds (or in the warning branch), before return:

```go
// Plan 13-07 addition — D-26 through D-28:
if cfg.Verbose {
    sudoLOut, sudoLErr := runOutput(client, "sudo -l")
    if sudoLErr == nil {
        fmt.Fprintf(os.Stderr, "[sudo -l]\n%s\n", strings.TrimSpace(string(sudoLOut)))
    }
    // D-27: non-zero or error → silently skip (no else branch)
}
```

**runOutput helper** (lines 310-317) — used for all read-only SSH queries; returns `([]byte, error)`:
```go
func runOutput(client SSHRunner, cmd string) ([]byte, error) {
    session, err := client.NewSession()
    if err != nil {
        return nil, fmt.Errorf("creating SSH session: %w", err)
    }
    defer session.Close() //nolint:errcheck
    return session.Output(cmd)
}
```

**Best-effort pattern** — `sudo -l` failure is silently swallowed by having no `else` on the `if sudoLErr == nil` block. This matches how `checkDaemon` handles daemon-down (warning only, no error return).

---

### `Makefile` — Plan 13-02 (ldflags extension)

**Analog:** `Makefile` lines 3-5 — existing build target:
```makefile
build:
	mkdir -p bin
	go build -ldflags "-X main.version=dev" -o bin/docker-deploy ./cmd/docker-deploy/
```
Extended with two additional `-X` flags:
```makefile
build:
	mkdir -p bin
	go build -ldflags "-X main.version=dev \
		-X main.gitCommit=$(shell git rev-parse --short HEAD 2>/dev/null || echo unknown) \
		-X main.buildTime=$(shell date -u +%FT%TZ 2>/dev/null || echo unknown)" \
		-o bin/docker-deploy ./cmd/docker-deploy/
```

---

### `.goreleaser.yaml` — Plan 13-02 (ldflags extension)

**Analog:** `.goreleaser.yaml` lines 6-9 — existing ldflags block:
```yaml
ldflags:
  - -s
  - -w
  - -X main.version={{.Version}}
```
Extended:
```yaml
ldflags:
  - -s
  - -w
  - -X main.version={{.Version}}
  - -X main.gitCommit={{.ShortCommit}}
  - -X main.buildTime={{.Date}}
```
**Note (RESEARCH.md Pitfall 3, Assumption A1):** GoReleaser's `{{.Date}}` is RFC3339 by default, not the Unix `date` style in D-01. If exact D-01 format is required, use `{{.Date | date "Mon Jan 02 15:04:05 2006"}}`. Otherwise RFC3339 is acceptable — D-01 is a style example. Verify at implementation time.

---

## Shared Patterns

### SSH session model (CLAUDE.md)
**Source:** `internal/filetransfer/upload.go` `sshExec` (lines 392-403) and `internal/preflight/checks.go` `runOutput` (lines 310-317)
**Apply to:** All SSH exec calls in Plans 13-04, 13-06, 13-07
```go
session, err := client.NewSession()  // fresh session per exec — NOT reusable
if err != nil {
    return fmt.Errorf("creating SSH session: %w", err)
}
defer session.Close() //nolint:errcheck
```

### ShellQuote on all remote paths
**Source:** `internal/filetransfer/upload.go` `ShellQuote` (lines 452-454)
**Apply to:** Plans 13-06 (`test -w` probe path), any new SSH commands in 13-04 that construct paths
```go
ShellQuote(remoteBase)  // wraps in single-quotes, escapes embedded quotes
```

### stderr for verbose/warnings, stdout for success summaries
**Source:** `cmd/docker-deploy/main.go` throughout; `internal/filetransfer/upload.go` lines 163-167
**Apply to:** Plans 13-02 (`version` → stdout), 13-03 (`validate` success → stdout, errors → stderr), 13-05 (diff → stderr), 13-07 (`sudo -l` → stderr)

### `path` vs `filepath` for remote vs local paths
**Source:** `internal/filetransfer/upload.go` imports (lines 8-9) — both `"path"` and `"path/filepath"` imported
**Apply to:** Plan 13-06 `test -w` probe (`path.Dir(remoteBase)` not `filepath.Dir`)

### Error wrapping with `fmt.Errorf("%w")`
**Source:** Throughout `upload.go` and `checks.go`
**Apply to:** All new error returns in Plans 13-03, 13-04, 13-06
```go
return fmt.Errorf("context description: %w", err)
```

### Test structure for `package main` (unit tests in same package)
**Source:** `cmd/docker-deploy/main_test.go` — `package main`, tests call unexported functions like `buildDeployCmd()`, `rollupMsg()`, `formatCheckResult()`
**Apply to:** New `TestVersionCmd*` and `TestValidateCmd*` tests added in Wave 0 for Plan 13-02/13-03

### Test structure for `package preflight_test` (external test package with fake runner)
**Source:** `internal/preflight/checks_test.go` — `package preflight_test`, uses `fakeSSHClient`/`fakeSession` to inject fake SSH responses
**Apply to:** New `TestCheckDockerGroup_SudoL*` tests for Plan 13-07; reuse existing `fakeSSHClient` and `newClient()` helpers (lines 29-66)

### Test structure for `package filetransfer` (white-box tests with in-process SSH mock)
**Source:** `internal/filetransfer/upload_test.go` — `package filetransfer`, uses `mockSSHServer` + `startMockSSHServer()` for in-process SSH
**Apply to:** New `TestSudoExec*`, `TestUploadVerbose_PreConfirmDiff`, `TestUpload_PathAwareSudo` tests for Plans 13-04/13-05/13-06

---

## No Analog Found

All 7 modified files have close analogs in the codebase. No files fall into this category.

---

## Wave Dependency Note

**Wave 1 (independent):** Plans 13-01, 13-02, 13-03, 13-04, 13-07
**Wave 2 (blocked on 13-04):** Plans 13-05 and 13-06 — both depend on 13-04 completing first
  - Plan 13-05: needs `creds *SudoCreds` signature from 13-04 before adding `force bool` to `Upload()`
  - Plan 13-06: calls `SudoExec` which is exported by Plan 13-04

The planner must encode this as hard dependencies: Plans 13-05 and 13-06 MUST NOT be assigned to a parallel wave with 13-04.

---

## Metadata

**Analog search scope:** `cmd/docker-deploy/`, `internal/filetransfer/`, `internal/preflight/`, `internal/config/`, `Makefile`, `.goreleaser.yaml`
**Files scanned:** 8 source files, 3 test files
**Pattern extraction date:** 2026-05-26
