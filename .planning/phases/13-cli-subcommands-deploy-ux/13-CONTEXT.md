# Phase 13: CLI Subcommands & Deploy UX - Context

**Gathered:** 2026-05-26
**Status:** Ready for planning

<domain>
## Phase Boundary

Six self-contained Go changes delivered as independent plans:

1. **deploy.yaml cwd resolution** — `deploy.yaml` resolved relative to `os.Getwd()` in all subcommands; no hardcoded absolute paths in config resolution logic
2. **`version` subcommand** — `docker deploy version` prints Docker CLI-style version info (version, git commit, build time, OS/arch) and exits 0; ldflags injection via GoReleaser/Makefile
3. **`validate` subcommand** — `docker deploy validate` runs `config.LoadFile` + `config.Resolve()` against the local `deploy.yaml` without dialing SSH; exit 0 on valid, non-zero + field errors on invalid
4. **SudoExec refactor** — extract `sudoRunWithFallback` closure into exported `filetransfer.SudoExec()` function; merge `sshExec`/`sshExecWithSudoPassword` into `sshRun`; change password storage from `*string` to `*SudoCreds` (`[]byte`)
5. **Verbose pre-confirm file diff** — in `--verbose` mode, list remote and local files before the "Replace all contents?" prompt so operators can see what will change
6. **Path-aware sudo detection** — probe `test -w <path>` before `SudoExec` calls; skip all sudo scaffolding entirely on user-writable paths

**Wave structure (deviation from ROADMAP.md):** ROADMAP.md lists all six plans as Wave 1 (parallel). Plans 13-04 and 13-06 have a dependency: Plan 13-04 (SudoExec refactor) must complete before Plan 13-06 (path-aware detection), because 13-06's `needsSudo` gate calls `SudoExec`. The planner must adjust to Wave 1 (13-01, 13-02, 13-03, 13-04, 13-05 independent) → Wave 2 (13-06 blocked on 13-04).

This phase adds NO new network protocols, no new config keys, and no new dependencies.

</domain>

<decisions>
## Implementation Decisions

### `version` subcommand (Plan 13-02)

- **D-01:** Output format (Docker CLI style):
  ```
  Docker Deploy Version v0.6.3
    Git commit:        de40ad0
   Built:             Mon Apr 20 14:57:44 2026
   OS/Arch:           darwin/arm64
  ```
- **D-02:** Three ldflags variables injected at build time: version (semver tag), git commit (short hash), build timestamp. OS/Arch is runtime (`runtime.GOOS + "/" + runtime.GOARCH`).
- **D-03:** Dev/untagged build: `version` stays `"dev"`, git commit shows short hash. Build timestamp can be omitted for dev builds.
- **D-04:** Subcommand only — `docker deploy version`. No `--version` flag added.
- **D-05:** Existing `var version = "dev"` in `main.go` becomes the ldflags injection target; extend with `var gitCommit = "unknown"` and `var buildTime = "unknown"`.

### `validate` subcommand (Plan 13-03)

- **D-06:** Runs `config.LoadFile(cwd)` + `config.Resolve()` — all existing validation is reused: host URL format (ParseHost), absolute path check, health_timeout/interval non-negative, YAML parse. No new validation logic.
- **D-07:** Missing `deploy.yaml` → exit non-zero, error: file not found. `validate` expects a config file to exist.
- **D-08:** No SSH connection made. No compose file existence check.
- **D-09:** Success output: `✓ deploy.yaml is valid` (stdout). Error output: field errors from Resolve() (stderr), exit non-zero.

### SudoExec refactor (Plan 13-04 — must complete before Plan 13-06)

- **D-10:** `sudoRunWithFallback` closure extracted into exported `SudoExec` in `internal/filetransfer/`:
  ```go
  func SudoExec(client *gossh.Client, cmd string, creds *SudoCreds, warnedOnce *bool, verbose bool) error
  ```
- **D-11:** Step order: (1) direct copy → (2) cached password if `creds.pw` already set → (3) passwordless sudo (`sudo -n`) → (4) interactive password (up to 3 attempts). Trying cached password before `sudo -n` avoids a redundant round-trip on calls 2+ within a single deploy.
- **D-12:** Password storage: `*SudoCreds` struct with `[]byte` field (not `*string`). Caller defers `creds.Zero()` after `Upload()` returns. `[]byte` can be zeroed; Go strings cannot.
  ```go
  type SudoCreds struct{ pw []byte }
  func (c *SudoCreds) Zero() { for i := range c.pw { c.pw[i] = 0 }; c.pw = nil }
  ```
- **D-13:** `sshExec` + `sshExecWithSudoPassword` merged into one private `sshRun(client, cmd string, pw []byte) error`. `pw==nil` → `session.Run(cmd)`; `pw!=nil` → `sudo -S -p '' sh -c <cmd>` with stdin pipe.
- **D-14:** `Upload()` closure replaced with `SudoExec(client, cmd, creds, &warnedOnce, verbose)` calls.
- **D-15:** All rollback/error paths in `Upload()` that touch `remoteBase` or its siblings use `SudoExec`, not bare `sshRun`. (Fixes the `feedback_sudo_rollback.md` issue structurally.)
- **D-16:** `tryDirectCopy` and `tryPasswordlessSudo` become internal implementation details of `SudoExec`, not standalone functions.

### Verbose pre-confirm file diff (Plan 13-05)

- **D-17:** Two plain lists displayed to stderr (before the "Replace all contents?" prompt, only in `--verbose` mode):
  ```
  Local files (N):
    docker-compose.yml
    .env
    app/main.go
    ... and M more — consider adding patterns to deploy.yaml exclude list
  Remote files (M):
    docker-compose.yml
    ...
  ```
- **D-18:** Truncate each list at 20 filenames. If more, show `... and N more — consider adding patterns to deploy.yaml exclude list`.
- **D-19:** First deploy (no remote dir yet): show `Remote files: (none)` — always show both sections for consistency.
- **D-20:** Remote file list uses SFTP `ReadDir` on `remoteBase`; local list from `WalkFiles`. Both already available at the point in `Upload()` where the confirm prompt fires.

### Path-aware sudo detection (Plan 13-06 — depends on Plan 13-04)

- **D-21:** Probe `test -w <remoteBase>` (or SFTP-based equivalent) at the start of `Upload()`, before any `SudoExec` calls.
- **D-22:** If probe succeeds (path writable as SSH user, or path doesn't exist but parent is writable) → `needsSudo=false` → bypass all `SudoExec` calls entirely (use direct `sshRun` with `nil` password).
- **D-23:** If probe fails (permission denied) → `needsSudo=true` → proceed with full `SudoExec` fallback chain as normal.
- **D-24:** Probe must not leave partial state if the path doesn't exist yet.
- **D-25:** User-writable path deploys (e.g. `~/myproject`) never prompt for a sudo password.

### Folded Todos

- **SudoExec refactor** (`.planning/todos/pending/2026-05-26-sudo-exec-refactor.md`) — folded into Plan 13-04. Exported `SudoExec`, `SudoCreds` type, `sshRun` merge, and `Upload()` closure replacement fully cover the todo's acceptance criteria.
- **Path-aware sudo detection** (`.planning/todos/pending/2026-05-26-sudo-path-aware-detection.md`) — folded into Plan 13-06. `test -w` probe, `needsSudo` flag, and bypass path cover the todo's acceptance criteria.

</decisions>

<canonical_refs>
## Canonical References

**Downstream agents MUST read these before planning or implementing.**

### Core files to modify

- `cmd/docker-deploy/main.go` — `buildDeployCmd()`, `runDeploy()`, `runDryRun()`, `var version = "dev"` (extend with `gitCommit`, `buildTime`); add `buildVersionCmd()` and `buildValidateCmd()` subcommands here
- `internal/config/config.go` — `LoadFile()`, `Resolve()`, `FlagOpts`, `Config`, `TargetConfig`; `validate` subcommand calls these directly
- `internal/filetransfer/upload.go` — `Upload()`, `sudoRunWithFallback` (to become `SudoExec`), `sshExec`, `sshExecWithSudoPassword` (to merge into `sshRun`); the primary file for Plans 13-04, 13-05, 13-06

### Todo specs (agent MUST read before implementing Plans 13-04 and 13-06)

- `.planning/todos/pending/2026-05-26-sudo-exec-refactor.md` — `SudoExec` function signature, `SudoCreds` type, `sshRun` merge, acceptance criteria
- `.planning/todos/pending/2026-05-26-sudo-path-aware-detection.md` — path-aware detection probe approach, `needsSudo` flag, acceptance criteria

### Build system

- `Makefile` — current `build` target; extend with ldflags for `version`, `gitCommit`, `buildTime`
- `.goreleaser.yaml` — `ldflags:` block; add `-X main.version={{.Version}} -X main.gitCommit={{.ShortCommit}} -X main.buildTime={{.Date}}`

### Memory / feedback

- `.claude/memory/feedback_sudo_rollback.md` (project memory) — Use `SudoExec` (not `sshRun` directly) in all rollback/error paths touching `remoteBase`

</canonical_refs>

<code_context>
## Existing Code Insights

### Reusable Assets

- `var version = "dev"` (`cmd/docker-deploy/main.go:26`) — existing ldflags target; extend with `var gitCommit = "unknown"` and `var buildTime = "unknown"` alongside it
- `config.LoadFile(cwd)` + `config.Resolve()` (`main.go:119–137`, `197–214`) — `validate` subcommand reuses this exact call sequence; no new validation logic
- `sudoRunWithFallback` closure (`upload.go:222–288`) — becomes `SudoExec`; step order is preserved with cached-password step inserted between direct and `sudo -n`
- `sshExec` (`upload.go:392–403`) + `sshExecWithSudoPassword` (`upload.go:408–430`) — merged into `sshRun(client, cmd string, pw []byte) error`
- `WalkFiles` (`internal/filetransfer/filter.go`) — already returns the local file list needed for verbose pre-confirm diff (D-20)

### Established Patterns

- Each SSH exec uses a fresh `client.NewSession()` (CLAUDE.md Rule) — `sshRun` must follow this
- `os.Stderr` for warnings and verbose output; `os.Stdout` for success/summary lines — `validate` success goes to stdout (`✓ deploy.yaml is valid`); errors to stderr
- `filetransfer.ShellQuote()` applied to all paths in SSH exec commands — the `test -w` probe in 13-06 needs this
- `plugin.Run()` wraps the cobra root command — `version` and `validate` are `AddCommand` subcommands on the root `deploy` command, not flags

### Integration Points

- `buildDeployCmd()` in `main.go` — `version` and `validate` subcommands registered via `cmd.AddCommand(buildVersionCmd(), buildValidateCmd())` inside this function
- GoReleaser `ldflags` block injects version/commit/time into `cmd/docker-deploy/main.go` package-level vars
- SFTP client opened inside `Upload()` at step 3 — the verbose pre-confirm diff (Plan 13-05) runs after SFTP is open (remote ReadDir) and after `WalkFiles` (local list), both already available before the confirm prompt

</code_context>

<specifics>
## Specific Ideas

- `version` output exact format (from discussion):
  ```
  Docker Deploy Version v0.6.3
    Git commit:        de40ad0
   Built:             Mon Apr 20 14:57:44 2026
   OS/Arch:           darwin/arm64
  ```
  For dev builds: `Docker Deploy Version dev\n  Git commit: abc1234\n  OS/Arch: darwin/arm64` (omit Built line).

- Verbose pre-confirm truncation message: `... and N more — consider adding patterns to deploy.yaml exclude list`

- `SudoCreds.Zero()` called with `defer` by the caller of `Upload()` in `main.go`, consistent with `golang.org/x/crypto` zeroing conventions.

</specifics>

<deferred>
## Deferred Ideas

- **`ssh_dial_timeout` config field** (`.planning/todos/pending/2026-05-26-ssh-dial-timeout-config-field.md`) — add `SSHDialTimeout` to `TargetConfig`, resolve via deploy.yaml. Out of scope for Phase 13; promoted todo is pending for a future phase.

</deferred>

---

*Phase: 13-CLI-Subcommands-Deploy-UX*
*Context gathered: 2026-05-26*
