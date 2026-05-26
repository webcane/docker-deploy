# Phase 13: CLI Subcommands & Deploy UX - Research

**Researched:** 2026-05-26
**Domain:** Go CLI subcommands, ldflags injection, SSH sudo refactoring, UX output improvements
**Confidence:** HIGH

## Summary

Phase 13 is seven self-contained Go changes — no new dependencies, no new config keys, no new network protocols. All changes touch existing files; the only new files are test files for new subcommands.

The research confirms that every change is well-scoped and can proceed directly from the CONTEXT.md decisions. The primary technical risk is in Plan 13-04 (SudoExec refactor): the `Upload()` signature changes because `*string` becomes `*SudoCreds`, which also changes the call site in `main.go`. All downstream call sites must be updated atomically. Plan 13-06 depends on 13-04 completing first because it calls `SudoExec` — the planner must encode this as a wave dependency.

The codebase is clean, all existing tests pass (`go test ./...` green), and the established patterns (one `NewSession()` per exec, `ShellQuote` on all remote paths, stderr for warnings/verbose output, stdout for success lines) are consistent throughout. New plans must follow these patterns without exception.

**Primary recommendation:** Implement plans as self-contained commits in wave order: Wave 1 = Plans 13-01 through 13-05 plus 13-07 (all independent); Wave 2 = Plan 13-06 (depends on 13-04's exported `SudoExec`).

<user_constraints>
## User Constraints (from CONTEXT.md)

### Locked Decisions

**D-01:** `version` output format (Docker CLI style):
```
Docker Deploy Version v0.6.3
  Git commit:        de40ad0
 Built:             Mon Apr 20 14:57:44 2026
 OS/Arch:           darwin/arm64
```

**D-02:** Three ldflags variables: `version` (semver tag), `gitCommit` (short hash), `buildTime`. OS/Arch is runtime (`runtime.GOOS + "/" + runtime.GOARCH`).

**D-03:** Dev/untagged build: `version="dev"`, git commit shows short hash. Build timestamp omitted for dev builds.

**D-04:** Subcommand only — `docker deploy version`. No `--version` flag added.

**D-05:** Existing `var version = "dev"` in `main.go` is the ldflags injection target; extend with `var gitCommit = "unknown"` and `var buildTime = "unknown"`.

**D-06:** `validate` runs `config.LoadFile(cwd)` + `config.Resolve()` — all existing validation reused. No new validation logic.

**D-07:** Missing `deploy.yaml` in validate → exit non-zero, error: file not found.

**D-08:** No SSH connection made by validate. No compose file existence check.

**D-09:** validate success: `✓ deploy.yaml is valid` (stdout). Errors: field errors from Resolve() (stderr), exit non-zero.

**D-10:** `SudoExec` function signature:
```go
func SudoExec(client *gossh.Client, cmd string, creds *SudoCreds, warnedOnce *bool, verbose bool) error
```

**D-11:** Step order in SudoExec: (1) direct → (2) cached password if `creds.pw` already set → (3) passwordless sudo (`sudo -n`) → (4) interactive password (up to 3 attempts).

**D-12:** Password storage: `*SudoCreds` with `[]byte` field. Caller defers `creds.Zero()` after `Upload()` returns.
```go
type SudoCreds struct{ pw []byte }
func (c *SudoCreds) Zero() { for i := range c.pw { c.pw[i] = 0 }; c.pw = nil }
```

**D-13:** `sshExec` + `sshExecWithSudoPassword` merged into private `sshRun(client, cmd string, pw []byte) error`. `pw==nil` → `session.Run(cmd)`; `pw!=nil` → `sudo -S -p '' sh -c <cmd>` with stdin pipe.

**D-14:** `Upload()` closure replaced with `SudoExec(client, cmd, creds, &warnedOnce, verbose)` calls.

**D-15:** All rollback/error paths in `Upload()` touching `remoteBase` or siblings use `SudoExec`, not bare `sshRun`.

**D-16:** `tryDirectCopy` and `tryPasswordlessSudo` become internal implementation details of `SudoExec`, not standalone functions.

**D-17:** Verbose pre-confirm: two plain lists to stderr before "Replace all contents?" prompt, only in `--verbose` mode.

**D-18:** Truncate each list at 20 filenames. Show `... and N more — consider adding patterns to deploy.yaml exclude list` if more.

**D-19:** First deploy (no remote dir yet): show `Remote files: (none)` — always show both sections.

**D-20:** Remote file list: SFTP `ReadDir` on `remoteBase`. Local list: `WalkFiles`. Both available before confirm prompt.

**D-21:** Probe `test -w <remoteBase>` at start of `Upload()` before any `SudoExec` calls.

**D-22:** If probe succeeds → `needsSudo=false` → bypass all `SudoExec` calls (use direct `sshRun` with `nil` password).

**D-23:** If probe fails (permission denied) → `needsSudo=true` → proceed with full `SudoExec` fallback chain.

**D-24:** Probe must not leave partial state if path doesn't exist yet.

**D-25:** User-writable path deploys (e.g. `~/myproject`) never prompt for sudo password.

**D-26:** When `--verbose`, run `sudo -l` on the remote during CHECK-04 preflight and print output to stderr.

**D-27:** Best-effort only — if `sudo -l` returns non-zero or fails, silently skip. Never block deploy.

**D-28:** Output to stderr, prefix with `[sudo -l]`.

**D-29:** Plan 13-07 is independent of 13-04/13-06 (touches `preflight/checks.go`, not `filetransfer/upload.go`).

**Wave structure:**
- Wave 1: Plans 13-01, 13-02, 13-03, 13-04, 13-05, 13-07 (independent, can run in parallel)
- Wave 2: Plan 13-06 (depends on 13-04's exported `SudoExec`)

### Claude's Discretion

None noted in CONTEXT.md.

### Deferred Ideas (OUT OF SCOPE)

- `ssh_dial_timeout` config field (`.planning/todos/pending/2026-05-26-ssh-dial-timeout-config-field.md`) — out of scope for Phase 13.
</user_constraints>

<phase_requirements>
## Phase Requirements

| ID | Description | Research Support |
|----|-------------|------------------|
| 13-01 | deploy.yaml resolved relative to `os.Getwd()` in all subcommands | Both `runDeploy` and `runDryRun` already call `os.Getwd()` and pass `cwd` to `config.LoadFile(cwd)` — the cwd-relative pattern is established. `validate` subcommand must follow the same pattern. |
| 13-02 | `docker deploy version` subcommand with ldflags injection | `var version = "dev"` already exists at `main.go:26`. GoReleaser already has `-X main.version={{.Version}}` in ldflags. Two new vars added alongside; Makefile build target extended with all three. |
| 13-03 | `docker deploy validate` subcommand — no SSH, reuses config validation | `config.LoadFile(cwd)` + `config.Resolve()` call sequence is established at `main.go:119-137` and `197-214`. `validate` reuses this verbatim; D-07 special-cases missing file to exit non-zero. |
| 13-04 | SudoExec refactor — exported function, SudoCreds type, sshRun merge | `sudoRunWithFallback` closure at `upload.go:222-288` extracted to `SudoExec`. `sshExec` at `upload.go:392-403` and `sshExecWithSudoPassword` at `upload.go:408-430` merged into `sshRun`. Caller in `main.go` changes `*string` → `*SudoCreds` with deferred `Zero()`. |
| 13-05 | Verbose pre-confirm file diff before "Replace all contents?" prompt | SFTP client already open at diff point in `Upload()`; `WalkFiles` already called at step 1. Remote `ReadDir` is an SFTP call on `remoteBase`. Truncation at 20 per D-18. |
| 13-06 | Path-aware sudo detection — `test -w` probe, `needsSudo` flag | Depends on Plan 13-04. Probe runs before any `SudoExec` calls. `ShellQuote` used on path. No partial state if path absent (probe is read-only). |
| 13-07 | Verbose `sudo -l` output during CHECK-04 preflight | `checkDockerGroup` in `preflight/checks.go` is CHECK-04. Best-effort `runOutput(client, "sudo -l")` call in the verbose branch; output prefixed `[sudo -l]` to stderr. Failure silently skipped. |
</phase_requirements>

## Architectural Responsibility Map

| Capability | Primary Tier | Secondary Tier | Rationale |
|------------|-------------|----------------|-----------|
| `version` subcommand output | CLI (main.go) | Build system (Makefile/.goreleaser.yaml) | Output is assembled from package-level vars injected at build time |
| `validate` subcommand | CLI (main.go) + Config layer | — | Calls existing config package; no network tier involved |
| deploy.yaml cwd resolution | CLI (main.go) | Config package | `os.Getwd()` called in command handlers; `LoadFile(cwd)` accepts the result |
| SudoExec refactor | File transfer layer (filetransfer/) | CLI (main.go) call site | SudoExec logic lives in filetransfer; main.go holds the SudoCreds lifecycle |
| Verbose pre-confirm diff | File transfer layer (filetransfer/upload.go) | — | Both local WalkFiles and remote SFTP ReadDir are available inside Upload() |
| Path-aware sudo detection | File transfer layer (filetransfer/upload.go) | — | Probe runs inside Upload() before first SudoExec call |
| Verbose `sudo -l` output | Preflight layer (preflight/checks.go) | — | CHECK-04 is checkDockerGroup; sudo -l fits naturally in that check's verbose branch |

## Standard Stack

### Core (no new dependencies)

| Library | Version | Purpose | Why Standard |
|---------|---------|---------|--------------|
| `github.com/spf13/cobra` | existing | Subcommand registration via `cmd.AddCommand()` | Already the project CLI framework [VERIFIED: go.mod in codebase] |
| `golang.org/x/crypto/ssh` | existing | `gossh.Client` parameter type for `SudoExec` | Project SSH library [VERIFIED: upload.go imports] |
| `github.com/pkg/sftp` | existing | `sftpClient.ReadDir()` for verbose pre-confirm remote listing | Already used in Upload() [VERIFIED: upload.go imports] |
| `runtime` | stdlib | `runtime.GOOS + "/" + runtime.GOARCH` for version output | No import needed — stdlib [ASSUMED] |

**No new dependencies are added in Phase 13.** [VERIFIED: CONTEXT.md domain section]

### Build system changes

| File | Current State | Required Change |
|------|--------------|-----------------|
| `Makefile` build target | `-X main.version=dev` only | Add `-X main.gitCommit=$(git rev-parse --short HEAD) -X main.buildTime=$(date -u +%FT%TZ)` |
| `.goreleaser.yaml` ldflags | `-X main.version={{.Version}}` only | Add `-X main.gitCommit={{.ShortCommit}} -X main.buildTime={{.Date}}` |

[VERIFIED: Makefile and .goreleaser.yaml read directly]

## Architecture Patterns

### System Architecture Diagram

```
┌─────────────────────────────────────────────────┐
│  docker deploy <subcommand>                     │
│  (cobra root command via plugin.Run())          │
└─────────┬───────────────────────────────────────┘
          │ AddCommand()
    ┌─────┼──────────────────────────────────────────┐
    │     │                                          │
    ▼     ▼                                          ▼
[version] [validate]                           [deploy / dry-run]
    │         │                                      │
    │    os.Getwd()                            os.Getwd()
    │    config.LoadFile(cwd)                  config.LoadFile(cwd)
    │    config.Resolve()                      config.Resolve()
    │    exit 0 / non-zero                          │
    │                                               │
    │                                    preflight.RunPreflightChecks()
    │                                         │ [13-07: sudo -l in CHECK-04 verbose]
    │                                         │
    │                                    filetransfer.Upload()
    │                                         │ [13-06: test -w probe → needsSudo]
    │                                         │ [13-05: verbose diff before confirm]
    │                                         │ [13-04: SudoExec replaces closure]
    │                                         │
    │                                    compose.RunCompose()
    │                                    health.PollHealth()
    │
    ▼
[runtime.GOOS/GOARCH + ldflags vars → formatted output]
```

### Recommended Project Structure (no changes)

```
cmd/docker-deploy/
└── main.go              # add buildVersionCmd(), buildValidateCmd()
                         # add var gitCommit, var buildTime
                         # change sudoPw *string → creds *SudoCreds + defer Zero()

internal/filetransfer/
└── upload.go            # Plans 13-04, 13-05, 13-06
                         # SudoExec exported, SudoCreds type, sshRun merged
                         # verbose pre-confirm diff, test -w probe

internal/preflight/
└── checks.go            # Plan 13-07: sudo -l in checkDockerGroup verbose branch
```

### Pattern 1: Cobra Subcommand Registration

**What:** New subcommands are registered as `cmd.AddCommand(buildXCmd())` inside `buildDeployCmd()`.
**When to use:** For all new subcommands — `version` and `validate`.
**Example:**
```go
// Source: cmd/docker-deploy/main.go existing pattern (buildDeployCmd)
func buildDeployCmd() *cobra.Command {
    cmd := &cobra.Command{Use: "deploy", ...}
    // ... flags ...
    cmd.AddCommand(buildVersionCmd())
    cmd.AddCommand(buildValidateCmd())
    return cmd
}

func buildVersionCmd() *cobra.Command {
    return &cobra.Command{
        Use:   "version",
        Short: "Print version information",
        RunE: func(cmd *cobra.Command, args []string) error {
            return runVersion()
        },
    }
}
```
[VERIFIED: existing `buildDeployCmd()` pattern in main.go]

### Pattern 2: ldflags Variable Injection

**What:** Package-level `var` declarations at the top of `main.go` are injection targets for `-X` ldflags.
**When to use:** For `version`, `gitCommit`, `buildTime`.
**Example:**
```go
// Source: cmd/docker-deploy/main.go:26 (existing)
var version = "dev"
var gitCommit = "unknown"   // NEW — injected by -X main.gitCommit=<hash>
var buildTime = "unknown"   // NEW — injected by -X main.buildTime=<timestamp>
```

Makefile dev build extension:
```makefile
build:
    mkdir -p bin
    go build -ldflags "-X main.version=dev \
        -X main.gitCommit=$(shell git rev-parse --short HEAD 2>/dev/null || echo unknown) \
        -X main.buildTime=$(shell date -u +%FT%TZ 2>/dev/null || echo unknown)" \
        -o bin/docker-deploy ./cmd/docker-deploy/
```

GoReleaser extension:
```yaml
ldflags:
  - -s
  - -w
  - -X main.version={{.Version}}
  - -X main.gitCommit={{.ShortCommit}}
  - -X main.buildTime={{.Date}}
```
[VERIFIED: .goreleaser.yaml read directly; GoReleaser template vars `.ShortCommit` and `.Date` are standard GoReleaser variables] [ASSUMED: GoReleaser `.Date` format matches the `Mon Jan 02 15:04:05 2006` style shown in D-01 — verify against GoReleaser docs if exact format matters]

### Pattern 3: cwd-Relative Config Loading

**What:** Every subcommand that touches config must call `os.Getwd()` first and pass the result to `config.LoadFile(cwd)`.
**When to use:** In `buildValidateCmd()` RunE; already done in `runDeploy` and `runDryRun`.
**Example:**
```go
// Source: cmd/docker-deploy/main.go:112-119 (runDryRun pattern)
cwd, err := os.Getwd()
if err != nil {
    return fmt.Errorf("getting working directory: %w", err)
}
projectName := filepath.Base(cwd)
fileConfig, err := config.LoadFile(cwd)
```
[VERIFIED: main.go read directly]

### Pattern 4: SudoExec Extracted Function

**What:** The `sudoRunWithFallback` closure becomes an exported `SudoExec` function in the `filetransfer` package. The `SudoCreds` type holds the password as `[]byte` for safe zeroing.
**When to use:** Everywhere `sudoRunWithFallback` was called inside `Upload()`, and in any new code that needs sudo-capable SSH exec.
**Example:**
```go
// Source: CONTEXT.md D-10, D-11, D-12 decisions

type SudoCreds struct{ pw []byte }
func (c *SudoCreds) Zero() { for i := range c.pw { c.pw[i] = 0 }; c.pw = nil }

func SudoExec(client *gossh.Client, cmd string, creds *SudoCreds, warnedOnce *bool, verbose bool) error {
    // Step 1: direct
    if sshRun(client, cmd, nil) == nil { return nil }
    // Step 2: cached password
    if creds.pw != nil {
        if sshRun(client, cmd, creds.pw) == nil { return nil }
    }
    // Step 3: passwordless sudo
    sudoCmd := fmt.Sprintf("sudo -n sh -c %s", ShellQuote(cmd))
    if sshRun(client, sudoCmd, nil) == nil { return nil }
    // Step 4: interactive (up to 3 attempts)
    // ... prompt loop ...
}
```
[VERIFIED: decision D-11 from CONTEXT.md; step order matches existing `sudoRunWithFallback` with cached-password step inserted]

### Pattern 5: sshRun Unified Primitive

**What:** `sshExec` and `sshExecWithSudoPassword` are merged into one private `sshRun`.
**When to use:** All SSH command execution inside the `filetransfer` package after Plan 13-04.
**Example:**
```go
// Source: CONTEXT.md D-13
func sshRun(client *gossh.Client, cmd string, pw []byte) error {
    session, err := client.NewSession()
    if err != nil {
        return fmt.Errorf("creating SSH session: %w", err)
    }
    defer session.Close()
    if pw == nil {
        return session.Run(cmd)
    }
    // sudo -S path with stdin pipe
    stdin, err := session.StdinPipe()
    if err != nil {
        return fmt.Errorf("opening stdin pipe: %w", err)
    }
    sudoCmd := fmt.Sprintf("sudo -S -p '' sh -c %s", ShellQuote(cmd))
    if err := session.Start(sudoCmd); err != nil {
        return fmt.Errorf("starting sudo command: %w", err)
    }
    _, _ = stdin.Write(append(pw, '\n'))
    _ = stdin.Close()
    return session.Wait()
}
```
[VERIFIED: existing `sshExec` and `sshExecWithSudoPassword` read directly from upload.go]

### Pattern 6: SFTP ReadDir for Remote File List

**What:** `sftpClient.ReadDir(remoteBase)` returns `[]os.FileInfo` for the remote directory's immediate children.
**When to use:** In Plan 13-05 verbose pre-confirm diff after SFTP client is open (step 3 in `Upload()`).
**Note:** `ReadDir` is non-recursive — returns only the top level. This is sufficient for the pre-confirm diff which shows the top-level contents. Deep trees should show the truncation message.
[VERIFIED: `github.com/pkg/sftp` already imported in upload.go; `sftpClient` variable available before the confirm prompt]

### Pattern 7: test -w Probe for Path-Aware Sudo Detection

**What:** `test -w <path>` exits 0 if the SSH user can write to the path. When path doesn't exist, also probe `test -w <parent>` to handle first-deploy case.
**When to use:** At the top of `Upload()` in Plan 13-06, after SFTP client opens but before any `SudoExec` calls.
**Example:**
```go
// Source: CONTEXT.md D-21; ShellQuote pattern from existing upload.go
probeCmd := fmt.Sprintf("test -w %s || test -w %s", ShellQuote(remoteBase), ShellQuote(path.Dir(remoteBase)))
needsSudo := sshRun(client, probeCmd, nil) != nil
```
[VERIFIED: `ShellQuote` and `sshRun` (post-13-04) pattern from upload.go; `path.Dir` for Linux remote paths uses `path` not `filepath`]

### Anti-Patterns to Avoid

- **Using `filepath.Dir` instead of `path.Dir` for remote paths:** Remote is always Linux. Local path ops use `filepath`; remote path ops use `path`. [VERIFIED: comment at `main.go:73` and `upload.go` import list]
- **Calling `sshExec` directly in rollback paths after 13-04:** After the SudoExec refactor, all rollback paths that touch `remoteBase` must use `SudoExec`. See `feedback_sudo_rollback.md` — using `sshExec` directly causes silent failures on root-owned directories. [VERIFIED: CONTEXT.md D-15 and memory file]
- **Reusing SSH sessions:** Each `sshRun` / `SudoExec` call must open a fresh `NewSession()`. Sessions are NOT reusable per CLAUDE.md. [VERIFIED: CLAUDE.md SSH session model section]
- **Adding `--version` flag to the root command:** D-04 is subcommand only — no flag variant. [VERIFIED: CONTEXT.md D-04]
- **Blocking deploy on `sudo -l` failure:** `sudo -l` in Plan 13-07 is best-effort only. Non-zero exit must be silently swallowed. [VERIFIED: CONTEXT.md D-27]
- **Blocking deploy on verbose pre-confirm diff errors:** SFTP `ReadDir` failure (e.g., path not yet created) must fall back to `(none)` display, not return an error. [VERIFIED: CONTEXT.md D-19]

## Don't Hand-Roll

| Problem | Don't Build | Use Instead | Why |
|---------|-------------|-------------|-----|
| Shell-safe remote paths | Custom quoting | `filetransfer.ShellQuote()` | Already implemented, tested (shellquote_test.go) [VERIFIED] |
| Local file enumeration | Custom walker | `filetransfer.WalkFiles()` | Already implements exclude matching, sorting [VERIFIED] |
| Cobra subcommand structure | Custom flag parsing | `cmd.AddCommand()` | Standard cobra pattern used throughout [VERIFIED] |
| Password zeroing | Custom struct | `SudoCreds.Zero()` / `[]byte` | `golang.org/x/crypto` convention — strings can't be zeroed [VERIFIED: todo spec] |

## Common Pitfalls

### Pitfall 1: Upload Signature Change Breaks main.go Call Site

**What goes wrong:** Plan 13-04 changes `Upload()` from `sudoPw *string, warnedOnce *bool` to `creds *SudoCreds, warnedOnce *bool`. The call in `main.go:337` uses `sudoPw := new(string)` — this will not compile after the refactor.
**Why it happens:** The refactor touches both the filetransfer package and its caller in main.go.
**How to avoid:** Plan 13-04 must update `main.go` in the same commit: replace `sudoPw := new(string)` with `creds := new(filetransfer.SudoCreds)` and add `defer creds.Zero()`.
**Warning signs:** Compile error on `Upload()` call after changing the function signature.
[VERIFIED: main.go:335-337 read directly]

### Pitfall 2: warnedOnce Rollup Logic in main.go

**What goes wrong:** After Plan 13-04, `Upload()` no longer receives `*string` for password — but `main.go` still reads `*warnedOnce` to add a rollup warning. This logic survives the refactor unchanged because `warnedOnce *bool` remains a parameter.
**Why it happens:** Easy to accidentally remove the rollup check during the refactor.
**How to avoid:** Keep `warnedOnce *bool` in `Upload()` signature and in `SudoExec`. Only `sudoPw *string` is replaced by `*SudoCreds`.
[VERIFIED: main.go:342-344 warnedOnce rollup logic read directly]

### Pitfall 3: GoReleaser `.Date` Format May Not Match D-01

**What goes wrong:** D-01 shows `Built: Mon Apr 20 14:57:44 2026` (Unix `date` style). GoReleaser's `{{.Date}}` produces RFC3339 format (e.g. `2026-04-20T14:57:44Z`) by default.
**Why it happens:** GoReleaser uses Go's `time.Format` via a template; the default is RFC3339.
**How to avoid:** Use GoReleaser's date format template: `{{.Date | date "Mon Jan 02 15:04:05 2006"}}` if the exact format from D-01 is required. Alternatively, accept RFC3339 — D-01 is a style example not a locked format.
**Warning signs:** Release build shows ISO date format instead of Unix-style.
[ASSUMED: GoReleaser date template syntax — verify against GoReleaser docs if exact format required]

### Pitfall 4: validate Subcommand Exits Non-Zero Correctly

**What goes wrong:** `cobra.Command.RunE` returns an error; cobra prints the error AND usage. For a validate subcommand the usage print is noise.
**Why it happens:** Cobra default behavior on `RunE` error is to print usage.
**How to avoid:** Set `cmd.SilenceUsage = true` on the validate command so only the error message appears on stderr. This matches the established pattern for the deploy command (or adopt the same pattern if needed).
[ASSUMED: Need to verify whether existing deploy command sets SilenceUsage — check at implementation time]

### Pitfall 5: path-aware Probe on Non-Existent Path

**What goes wrong:** `test -w /opt/newproject` fails when `/opt/newproject` doesn't exist, even if the user could create it. D-22 says probe should succeed if "path doesn't exist but parent is writable".
**Why it happens:** `test -w` only checks existing paths.
**How to avoid:** Probe is a two-part OR: `test -w <path> || test -w <parent-path>`. If either succeeds, `needsSudo=false`. The parent check covers first-deploy user-writable cases.
**Warning signs:** First deploy to `~/myproject` (which doesn't exist yet but `~` is writable) incorrectly sets `needsSudo=true`.
[VERIFIED: CONTEXT.md D-22 "path doesn't exist but parent is writable"]

### Pitfall 6: SFTP ReadDir on Non-Existent remoteBase

**What goes wrong:** Plan 13-05 calls `sftpClient.ReadDir(remoteBase)` for the verbose pre-confirm diff. On first deploy, `remoteBase` doesn't exist — `ReadDir` returns an error.
**Why it happens:** SFTP ReadDir requires the directory to exist.
**How to avoid:** Check `existsBefore` (already computed by this point in `Upload()`). If `!existsBefore`, display `Remote files: (none)` without calling `ReadDir`.
[VERIFIED: `existsBefore` variable in upload.go:185 is computed before the confirm prompt insertion point]

### Pitfall 7: Verbose Pre-Confirm Must Precede the Prompt

**What goes wrong:** Inserting the file diff output after the confirm prompt means the user sees "Replace all contents? [y/N]" before the file list — backwards.
**Why it happens:** The confirm prompt is at `main.go:307-327` (checking remote dir existence). The diff output from `Upload()` would come after. But the diff needs to run inside `Upload()` after SFTP opens.
**How to avoid:** The confirm prompt lives in `main.go` before `Upload()` is called. The verbose diff must also happen before the prompt. Options: (a) move the diff out of `Upload()` to `main.go` between the existence check and the prompt; (b) pass a pre-prompt hook to `Upload()`. The simplest approach: add the remote `ReadDir` call in `main.go` in the same block as the existence check (after `sftpClient` open), print the diff to stderr, then show the prompt. This keeps `Upload()` unchanged.

Wait — SFTP client is opened *inside* `Upload()`. So either the diff runs inside `Upload()` but the prompt is outside, or the SFTP client must be opened earlier in `main.go`.

**The correct approach per D-20:** The diff runs inside `Upload()` — but only if D-20 placement is correct. Review: the confirm prompt in `main.go` happens at step 7 before calling `Upload()`. But `Upload()` opens SFTP at step 3. The diff (D-20) says it runs before the confirm prompt while SFTP is already open. This is contradictory unless the prompt is moved inside `Upload()`.

**Resolution:** The confirm prompt must be moved into `Upload()` or the SFTP client opened before `Upload()` in `main.go`. Since the CONTEXT.md says both SFTP ReadDir and WalkFiles are "already available at the point in `Upload()` where the confirm prompt fires" (code_context section), the confirm prompt logic needs to move inside `Upload()`. This is a significant structural change to document.

**Warning signs:** Diff appearing after the user has already answered the prompt.
[VERIFIED: upload.go and main.go read directly — confirm prompt is currently in main.go:295-328, Upload() is called at main.go:337]

### Pitfall 8: SudoCreds.Zero() Must Not Panic on nil creds

**What goes wrong:** `defer creds.Zero()` in main.go — if `new(filetransfer.SudoCreds)` is called, creds is never nil and this is safe. But `Zero()` must guard against nil `c.pw` (already nil = no-op).
**Why it happens:** `Zero()` is called whether or not a password was ever stored.
**How to avoid:** The `for i := range c.pw { ... }; c.pw = nil` pattern is safe on a nil slice — range over nil slice is zero iterations, and setting nil to nil is harmless.
[VERIFIED: CONTEXT.md D-12 implementation shown]

## Code Examples

### Version Subcommand Output

```go
// Source: CONTEXT.md D-01, D-03
func runVersion() error {
    if buildTime != "unknown" {
        fmt.Fprintf(os.Stdout, "Docker Deploy Version %s\n  Git commit:        %s\n Built:             %s\n OS/Arch:           %s/%s\n",
            version, gitCommit, buildTime, runtime.GOOS, runtime.GOARCH)
    } else {
        // Dev build: omit Built line
        fmt.Fprintf(os.Stdout, "Docker Deploy Version %s\n  Git commit:        %s\n OS/Arch:           %s/%s\n",
            version, gitCommit, runtime.GOOS, runtime.GOARCH)
    }
    return nil
}
```

### Validate Subcommand (D-06 through D-09)

```go
// Source: CONTEXT.md decisions D-06 to D-09; config.LoadFile + Resolve pattern from main.go
func runValidate() error {
    cwd, err := os.Getwd()
    if err != nil {
        return fmt.Errorf("getting working directory: %w", err)
    }

    // D-07: missing deploy.yaml is an error (not a silent zero FileConfig)
    if _, statErr := os.Stat(filepath.Join(cwd, "deploy.yaml")); os.IsNotExist(statErr) {
        fmt.Fprintln(os.Stderr, "deploy.yaml not found")
        return fmt.Errorf("deploy.yaml not found")
    }

    projectName := filepath.Base(cwd)
    fileConfig, err := config.LoadFile(cwd)
    if err != nil {
        fmt.Fprintln(os.Stderr, err.Error())
        return err
    }

    _, err = config.Resolve(config.FlagOpts{}, fileConfig, projectName, cwd)
    if err != nil {
        fmt.Fprintln(os.Stderr, err.Error())
        return err
    }

    fmt.Fprintln(os.Stdout, "✓ deploy.yaml is valid")
    return nil
}
```

### Verbose pre-confirm diff inside Upload (D-17 through D-20)

```go
// Source: CONTEXT.md D-17 to D-20; sftpClient.ReadDir and WalkFiles both available
// This block runs after sftpClient is open (step 3) and after WalkFiles (step 1),
// but only when the confirm prompt is triggered (existsBefore check) and verbose=true.

if verbose && !force {
    // Local files
    fmt.Fprintf(os.Stderr, "Local files (%d):\n", len(files))
    for i, f := range files {
        if i >= 20 {
            fmt.Fprintf(os.Stderr, "  ... and %d more — consider adding patterns to deploy.yaml exclude list\n", len(files)-20)
            break
        }
        fmt.Fprintf(os.Stderr, "  %s\n", f)
    }
    // Remote files
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

### sudo -l in CHECK-04 (D-26 through D-28)

```go
// Source: CONTEXT.md D-26 to D-28; preflight/checks.go checkDockerGroup pattern
// Added inside checkDockerGroup() in the verbose path (cfg.Verbose — no signature change needed)

if cfg.Verbose {
    sudoLOut, sudoLErr := runOutput(client, "sudo -l")
    if sudoLErr == nil {
        fmt.Fprintf(os.Stderr, "[sudo -l]\n%s\n", strings.TrimSpace(string(sudoLOut)))
    }
    // D-27: non-zero or error → silently skip
}
```

**Note:** `RunPreflightChecks` currently does not receive `verbose bool`. Plan 13-07 must add it to the `RunPreflightChecks` signature and thread it to `checkDockerGroup`. The call site in `main.go:272` must also pass `resolved.Verbose`.
[VERIFIED: checks.go RunPreflightChecks signature read directly]

## State of the Art

| Old Approach | Current Approach | When Changed | Impact |
|--------------|------------------|--------------|--------|
| `sudoRunWithFallback` closure in Upload | Exported `SudoExec` function | Plan 13-04 | Enables reuse, testability, rollback safety |
| `*string` password storage | `*SudoCreds` with `[]byte` | Plan 13-04 | Password can be zeroed after use |
| `sshExec` + `sshExecWithSudoPassword` separate | `sshRun(pw []byte)` unified | Plan 13-04 | Single codepath, simpler maintenance |
| Always runs sudo scaffold | `needsSudo` gated on `test -w` probe | Plan 13-06 | Home dir deploys no longer prompt for sudo |

## Assumptions Log

| # | Claim | Section | Risk if Wrong |
|---|-------|---------|---------------|
| A1 | GoReleaser `.Date` template variable produces a format compatible with D-01's output style, or can be formatted with `{{.Date \| date "..."}}` | Standard Stack / Pitfall 3 | Release build shows ISO date; cosmetic mismatch with D-01 |
| A2 | GoReleaser `.ShortCommit` template variable provides the 7-char short git hash equivalent to `git rev-parse --short HEAD` | Standard Stack | Wrong hash format in release binary |
| A3 | `cobra.Command.SilenceUsage = true` is needed on validate/version commands to suppress usage on error | Pitfall 4 | Noisy error output with usage block when validation fails |
| A4 | `checkDockerGroup` is the function that implements CHECK-04 (D-26 says "CHECK-04 in `internal/preflight/checks.go`") | Code Examples | Wrong check gets sudo -l; easy to verify by reading checks.go |

**Note on A4:** This is [VERIFIED] — `checkDockerGroup` was read directly from checks.go lines 265-302 and is clearly CHECK-04 (checks docker group membership).

## Open Questions (RESOLVED)

1. **Confirm prompt architecture for Plan 13-05**
   - What we know: The confirm prompt is in `main.go:295-328`; SFTP client is opened inside `Upload()` after the prompt is shown.
   - What's unclear: D-20 says the diff runs inside `Upload()`, but the prompt currently fires before `Upload()` is called. Either (a) the confirm prompt moves inside `Upload()` or (b) a pre-prompt hook is passed to `Upload()`.
   - Recommendation: Move the confirm prompt into `Upload()` — it has access to both `existsBefore`, SFTP client, and `WalkFiles`. The `force` parameter is already threaded through. This is cleaner than passing hooks. The planner must assign this architectural move explicitly to Plan 13-05 rather than leaving it implicit.

2. **verbose parameter threading in RunPreflightChecks for Plan 13-07**
   - What we know: `RunPreflightChecks` signature is `(ctx, client SSHRunner, cfg config.Config)`. No `verbose` parameter exists.
   - What's unclear: Add `verbose bool` as a 4th parameter, or use `cfg.Verbose`?
   - Recommendation: Use `cfg.Verbose` — it is already in the `config.Config` struct passed to `RunPreflightChecks`. No signature change needed; just read `cfg.Verbose` inside `checkDockerGroup`. But `checkDockerGroup` currently receives only `client SSHRunner` and `cfg config.Config`. `cfg.Verbose` is accessible — thread it.

## Environment Availability

Step 2.6: SKIPPED — Phase 13 is pure Go code changes. No external tools, services, or CLIs beyond what the existing build already requires.

## Validation Architecture

### Test Framework
| Property | Value |
|----------|-------|
| Framework | Go testing stdlib |
| Config file | none (go test ./...) |
| Quick run command | `go test ./cmd/docker-deploy/... ./internal/filetransfer/... ./internal/preflight/... ./internal/config/...` |
| Full suite command | `go test ./...` |

### Phase Requirements → Test Map

| Req ID | Behavior | Test Type | Automated Command | File Exists? |
|--------|----------|-----------|-------------------|-------------|
| 13-01 | deploy.yaml resolved from cwd | unit | `go test ./internal/config/... -run TestLoadFile` | ✅ config_test.go |
| 13-02 | version subcommand registered and prints correct output | unit | `go test ./cmd/docker-deploy/... -run TestVersionCmd` | ❌ Wave 0 |
| 13-03 | validate subcommand exits 0 on valid config, non-zero on invalid | unit | `go test ./cmd/docker-deploy/... -run TestValidateCmd` | ❌ Wave 0 |
| 13-04 | SudoExec exported, SudoCreds type, sshRun merged | unit | `go test ./internal/filetransfer/... -run TestSudoExec` | ❌ Wave 0 |
| 13-04 | Upload() call site compiles with *SudoCreds | compilation | `go build ./...` | n/a |
| 13-05 | Verbose pre-confirm diff shows local/remote files | unit | `go test ./internal/filetransfer/... -run TestUploadVerbose_PreConfirmDiff` | ❌ Wave 0 |
| 13-06 | needsSudo=false for writable path, =true for root-owned | unit | `go test ./internal/filetransfer/... -run TestUpload_PathAwareSudo` | ❌ Wave 0 |
| 13-07 | sudo -l output printed in verbose preflight, skipped on error | unit | `go test ./internal/preflight/... -run TestCheckDockerGroup_SudoL` | ❌ Wave 0 |

### Sampling Rate
- **Per task commit:** `go test ./...`
- **Per wave merge:** `go test ./...`
- **Phase gate:** Full suite green before `/gsd-verify-work`

### Wave 0 Gaps
- [ ] `cmd/docker-deploy/main_test.go` — add `TestVersionCmd*` tests (subcommand registered, output format)
- [ ] `cmd/docker-deploy/main_test.go` — add `TestValidateCmd*` tests (valid config exit 0, missing file exit non-zero, bad YAML exit non-zero)
- [ ] `internal/filetransfer/upload_test.go` — add `TestSudoExec*` tests (direct success, cached creds, passwordless sudo, all paths exhausted)
- [ ] `internal/filetransfer/upload_test.go` — add `TestUploadVerbose_PreConfirmDiff` test
- [ ] `internal/filetransfer/upload_test.go` — add `TestUpload_PathAwareSudo` test (writable probe skips sudo, non-writable triggers sudo)
- [ ] `internal/preflight/checks_test.go` — add `TestCheckDockerGroup_SudoL*` tests (verbose=true prints output, sudo -l failure is silently skipped)

## Security Domain

### Applicable ASVS Categories

| ASVS Category | Applies | Standard Control |
|---------------|---------|-----------------|
| V2 Authentication | no | No new auth introduced |
| V3 Session Management | no | No new session management |
| V4 Access Control | no | No new access control |
| V5 Input Validation | yes | `config.Resolve()` validates all config fields; `ShellQuote()` on all remote paths |
| V6 Cryptography | partial | `SudoCreds.Zero()` zeroes password bytes after use — `[]byte` zeroing is the Go crypto convention |

### Known Threat Patterns for This Stack

| Pattern | STRIDE | Standard Mitigation |
|---------|--------|---------------------|
| Password exposure in process listings | Information Disclosure | `sudo -S -p '' sh -c <cmd>` with stdin pipe (existing pattern, preserved in `sshRun`) [VERIFIED: sshExecWithSudoPassword:420] |
| Path traversal in remote paths | Tampering | `ShellQuote()` + absolute path validation in `config.Resolve()` [VERIFIED: config.go:323] |
| `sudo -l` stderr exposure | Information Disclosure | Output goes to stderr only in verbose mode; best-effort only — no credentials in `sudo -l` output |

## Sources

### Primary (HIGH confidence)
- Codebase read directly — `cmd/docker-deploy/main.go`, `internal/filetransfer/upload.go`, `internal/preflight/checks.go`, `internal/config/config.go`, `Makefile`, `.goreleaser.yaml`
- `.planning/phases/13-cli-subcommands-deploy-ux/13-CONTEXT.md` — all decisions (D-01 through D-29)
- `.planning/todos/pending/2026-05-26-sudo-exec-refactor.md` and `2026-05-26-sudo-path-aware-detection.md`

### Secondary (MEDIUM confidence)
- `go test ./...` run output — confirmed all tests pass before Phase 13 begins

### Tertiary (LOW confidence)
- None — all claims derived from direct codebase inspection or CONTEXT.md decisions

## Metadata

**Confidence breakdown:**
- Standard stack: HIGH — no new dependencies; all libraries already in use
- Architecture: HIGH — all patterns verified by reading actual source files
- Pitfalls: HIGH for verified pitfalls; ASSUMED where indicated
- Assumptions log: 3 items require verification at implementation time (GoReleaser template format, SilenceUsage behavior, verbose threading approach)

**Research date:** 2026-05-26
**Valid until:** Stable — this is internal Go code with no external API dependencies
