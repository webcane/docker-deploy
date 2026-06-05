# Phase 4: Core Deploy Loop - Context

**Gathered:** 2026-05-15
**Status:** Ready for planning

<domain>
## Phase Boundary

After files are copied to the remote host, execute `docker compose up -d --remove-orphans` via SSH and stream its output to the local terminal. The plugin exits with a non-zero code if any step fails (copy, compose exec, SSH connectivity loss). This phase wires the file copy from Phase 3 to the compose execution, completing the full deploy cycle.

**In scope:**
- Compose file resolution (auto-detect locally, `--compose-file` flag, `compose_file` config key)
- Remote compose execution via SSH (`docker compose -f <path>/<file> up -d --remove-orphans`)
- Output streaming: PTY when local stdout is a TTY (colors + progress); piped stdout/stderr when not
- Exit code propagation and failure UX
- Extending Config struct with `ComposeFile` field

**Out of scope:**
- Pre-flight checks (Phase 5)
- Health polling (Phase 5)
- `--verbose` flag implementation (Phase 7 — but code should be structured to accept it easily)
- `--pull` / image management flags (Phase 7 or later)

</domain>

<decisions>
## Implementation Decisions

### Output Streaming

- **D-01:** When local `os.Stdout` is a TTY: allocate a PTY on the SSH session (`RequestPty`) before running compose. Inherit the local terminal's columns × rows and pass them to `RequestPty`. In PTY mode stdout and stderr merge through the PTY — both are forwarded to `os.Stdout`. This is accepted as a PTY limitation.
- **D-02:** When local `os.Stdout` is not a TTY (piped/redirected): no PTY. Use `session.StdoutPipe()` forwarded to `os.Stdout` and `session.StderrPipe()` forwarded to `os.Stderr`. Two goroutines, separate streams, no color codes.
- **D-03:** TTY detection: use `golang.org/x/term` `IsTerminal(int(os.Stdout.Fd()))` (already a transitive dep via `golang.org/x/crypto`).

### Compose Command Flags

- **D-04:** Default compose command: `docker compose -f <remote_path>/<compose_file> up -d --remove-orphans`
- **D-05:** `--remove-orphans` is always included — removes stale containers from previous compose file versions on every deploy.
- **D-06:** No pull flags by default. Remote uses cached images. `--pull` management deferred to Phase 7.

### Compose File Resolution

- **D-07:** Compose file is resolved in priority order:
  1. `--compose-file <filename>` CLI flag
  2. `compose_file: <filename>` in `deploy.yaml`
  3. Auto-detect in local project root: try `compose.yaml` first, then `docker-compose.yml`
- **D-08:** If no compose file is found by any of the three methods → fail with a clear error: `no compose file found; use --compose-file to specify one`.
- **D-09:** The resolved filename (not full path) is what gets appended to the remote path for the `-f` argument. The file was already copied to `<remote_path>/` in Phase 3.
- **D-10:** Add `ComposeFile string` field to the `Config` struct (alongside `Host`, `Path`, `Excludes`, `Force`). `Resolve()` signature extended to accept the `--compose-file` flag value.

### Command Invocation Pattern

- **D-11:** Remote command form: `docker compose -f <ShellQuote(remote_path)>/<compose_file> up -d --remove-orphans`
  - Use `filetransfer.ShellQuote()` for the remote path (already exported in Phase 3).
  - `compose_file` is just a filename (no slashes) — no quoting needed if validated to be a basename only.
  - Do NOT use `cd && compose` — explicit `-f` is preferred for clarity and debuggability.

### Compose Failure UX

- **D-12:** On compose failure: since output is already streamed to the terminal throughout execution, no output replay is needed. Append a terse error line to `os.Stderr`: `Deploy failed: docker compose exited with code N`.
- **D-13:** When `--verbose` is added (Phase 7), the failure message should expand with more detail (e.g., last N lines of compose output, exit status breakdown). The compose runner must be structured so Phase 7 can augment this without rewriting it.
- **D-14:** All plugin-level error messages (SSH failure, compose failure, config errors) go to `os.Stderr`. `os.Stdout` carries only compose output and the final deploy-complete line.

### Claude's Discretion

- PTY terminal type string to pass to `RequestPty`: use `"xterm-256color"` — industry standard, widely supported.
- Whether to use `session.Run()` or `session.Start()` + `session.Wait()` for the compose session: use `session.Start()` + goroutine drains + `session.Wait()` — gives control over stream teardown order.
- Exit code extraction from `*gossh.ExitError`: use `e.ExitStatus()`.

</decisions>

<canonical_refs>
## Canonical References

**Downstream agents MUST read these before planning or implementing.**

### Phase Requirements
- `REQUIREMENTS.md` — DEPLOY-01, DEPLOY-04, DEPLOY-05, DEPLOY-06 are the four requirements this phase satisfies
- `.planning/ROADMAP.md` §Phase 4 — Success criteria (3 items)

### Existing Implementation (read before extending)
- `cmd/docker-deploy/main.go` — `runDeploy()` is the entry point; Phase 4 adds compose execution after step 9 (`Upload()`)
- `internal/ssh/client.go` — `Dial()` returns `*gossh.Client`; session model: `client.NewSession()` per command, sessions are NOT reusable
- `internal/config/config.go` — `Config` struct and `Resolve()` to extend with `ComposeFile` field
- `internal/filetransfer/upload.go` — `ShellQuote()` already exported; reuse it for remote path quoting

### Critical Constraints (from CLAUDE.md)
- `CLAUDE.md` — Rule 1 (no InsecureIgnoreHostKey), Rule 3 (separate session per SSH command), Rule 5 (docker compose v2 only)

</canonical_refs>

<code_context>
## Existing Code Insights

### Reusable Assets
- `filetransfer.ShellQuote(s string) string` — exported in Phase 3; use for quoting the remote path in the compose command
- `sshpkg.Dial(ctx, cfg)` — returns `*gossh.Client`; compose session is `client.NewSession()` on the same client used for Upload
- `golang.org/x/term` — already a transitive dep; use `term.IsTerminal(int(os.Stdout.Fd()))` for TTY detection

### Established Patterns
- **One session per SSH command**: Phase 3 already uses separate sessions for existence check vs mv/rm. Compose execution is one more session following this same pattern.
- **Session lifecycle**: create session → configure → run → close. Never reuse a session after `Run()`/`Wait()`.
- **Error wrapping**: `fmt.Errorf("context: %w", err)` throughout; compose errors should follow same pattern.
- **Stderr for plugin messages**: `fmt.Fprintf(os.Stderr, ...)` for all plugin-level output (SSH errors, config errors, failure summaries). Already established in `runDryRun` and `runDeploy`.

### Integration Points
- `runDeploy()` in `main.go`: insert compose execution between step 9 (`Upload()`) and step 10 (success print). The SSH `client` is already open at that point.
- `config.Resolve()`: add `composeFile string` parameter (flag value) and populate `Config.ComposeFile` with the resolved filename.
- `cmd/docker-deploy/main.go` flag registration: add `--compose-file` `StringVar` alongside `--host`, `--path`, etc.

</code_context>

<specifics>
## Specific Ideas

- User explicitly wants `docker compose -f <path>/<file>` (not `cd && compose`) for clarity and debuggability.
- PTY mode should feel like running compose locally — colors, progress bars, the same UX the operator gets when SSH'd in manually.
- The verbose flag (Phase 7) should get richer failure detail; design the compose runner with that extension point in mind.

</specifics>

<deferred>
## Deferred Ideas

- `--pull always` / image pull control — deferred to Phase 7 (v2 leftovers)
- `--verbose` flag implementation — Phase 7; but failure UX in Phase 4 must accommodate it structurally
- `--wait` flag (compose 2.1+: wait for healthchecks before returning) — deferred to Phase 5 discussion; health polling is Phase 5's domain

None — discussion stayed within phase scope otherwise.

</deferred>

---

*Phase: 4-Core Deploy Loop*
*Context gathered: 2026-05-15*
