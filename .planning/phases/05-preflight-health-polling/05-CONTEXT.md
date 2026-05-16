# Phase 5: Pre-flight & Health Polling - Context

**Gathered:** 2026-05-16
**Status:** Ready for planning

<domain>
## Phase Boundary

Run a suite of SSH remote-command checks before any file copy to validate the remote host is deploy-ready, then poll `docker inspect` after `docker compose up -d` completes to report container health and exit accordingly.

**In scope:**
- Pre-flight checks: CHECK-01 through CHECK-07 (Docker installed, compose v2, daemon running, docker group, sudo access, target dir writable, root user warning)
- Health polling after compose up: HEALTH-01 through HEALTH-03 (poll via `docker inspect`, report healthy/unhealthy/unknown, exit non-zero on unhealthy or timeout)
- Auto-recovery for CHECK-04 (docker group) and CHECK-06 (target dir) via passwordless sudo
- Config extension: `health_timeout` and `health_interval` keys in `deploy.yaml`

**Out of scope:**
- `--verbose` flag / live checklist output (Phase 7)
- Passwordless sudo setup documentation (Phase 7)
- `--init` wizard (Phase 6)

</domain>

<decisions>
## Implementation Decisions

### Pre-flight Output UX

- **D-01:** Default behavior is silent on pass. No output is printed when all checks pass cleanly.
- **D-02:** Errors and warnings are printed to `os.Stderr`. CHECK-07 (root user warning) always prints even on an otherwise clean run.
- **D-03:** The `--verbose` live checklist (e.g. `[✓] Docker v25.0 installed`) is deferred to Phase 7 when the `--verbose` flag is implemented. Phase 5 does not register the flag or add the verbose path.

### Pre-flight Check Sequencing and Failure Behavior

- **D-04:** Fail-fast on first blocking error — stop and report immediately when a hard-blocking check fails. Do not continue to subsequent checks.
- **D-05:** Check severity classification:
  - **Hard block:** CHECK-01 (Docker binary absent), CHECK-02 (compose v2 missing or only v1 present)
  - **Warning only (never blocks):** CHECK-03 (Docker daemon not running — file copy can proceed; compose will fail at execution time), CHECK-07 (SSH user is root)
  - **Auto-fix via sudo, fail if fix fails:** CHECK-04 (docker group membership), CHECK-06 (target dir writable)
  - **Conditional — only checked when sudo is actually needed:** CHECK-05 (sudo access, checked before any sudo attempt in CHECK-04 or CHECK-06)
- **D-06:** CHECK-03 rationale: daemon stopped is a recoverable state (operator can `systemctl start docker`); blocking the copy would prevent even preparing the files. Print a warning and let compose fail later with the native Docker error if the daemon is still down.

### sudo Mechanics

- **D-07:** Assume passwordless sudo. Attempt sudo commands directly (`sudo usermod -aG docker $USER`, `sudo mkdir -p <path>`, `sudo chown <user> <path>`). Do not prompt for a sudo password — SSH exec sessions have no interactive TTY for password entry.
- **D-08:** If a sudo command fails (no passwordless sudo configured), print a clear error message to `os.Stderr` that includes the exact command the operator needs to run manually. Example: `Error: user not in docker group. Fix: sudo usermod -aG docker <user> (as root or a user with NOPASSWD sudo)`.
- **D-09:** Passwordless sudo setup documentation is deferred to Phase 7 (separate `.md` file, linked from README.md).

### CHECK-06: Directory Creation Strategy

- **D-10:** Try `mkdir -p <path>` without sudo first. If it fails with a permission error (EACCES / exit code indicating permission denied), retry with `sudo mkdir -p <path> && sudo chown $USER <path>`. Let the error drive the sudo decision — do not hard-code path-prefix heuristics.

### Health Polling

- **D-11:** Poll container health via `docker inspect --format '{{.State.Health.Status}}' <container>` (or equivalent `docker inspect` query for all compose-project containers) every **5 seconds** for up to **60 seconds** after `docker compose up -d` returns.
- **D-12:** Both timeout and interval are configurable in `deploy.yaml` as `health_timeout` (seconds, default 60) and `health_interval` (seconds, default 5). These extend the `Config` struct and are resolved via the existing `Resolve()` precedence chain.
- **D-13:** Terminal states:
  - Any container reaches `unhealthy` → print error to `os.Stderr`, exit non-zero immediately (do not wait for remaining poll time).
  - All containers reach `healthy` → print success line, exit 0.
  - Container has no HEALTHCHECK defined (status `""` or `"none"`) → print a warning per container (`Warning: <service> has no HEALTHCHECK defined`), treat as passing, continue.
  - Any container still in `starting` when the 60s timeout expires → print error, exit non-zero.

### Claude's Discretion

- Which `docker inspect` format string produces the health status across Docker versions — research during planning.
- Whether to enumerate containers by compose project label (`com.docker.compose.project=<name>`) or by parsing `docker compose ps --format json` — pick the more robust approach.
- Error message wording and formatting (keep concise, actionable, consistent with Phase 4 patterns).

</decisions>

<canonical_refs>
## Canonical References

**Downstream agents MUST read these before planning or implementing.**

### Phase Requirements
- `.planning/REQUIREMENTS.md` — CHECK-01 through CHECK-07 and HEALTH-01 through HEALTH-03 are the 10 requirements this phase satisfies
- `.planning/ROADMAP.md` §Phase 5 — Success criteria (6 items) and dependency on Phase 4

### Existing Implementation (read before extending)
- `cmd/docker-deploy/main.go` — `runDeploy()` is the entry point; Phase 5 inserts pre-flight checks between step 6 (SSH Dial) and step 7 (existence check), and adds health polling after step 9 (RunCompose)
- `internal/ssh/client.go` — `Dial()` returns `*gossh.Client`; session model: `client.NewSession()` per command, sessions are NOT reusable
- `internal/config/config.go` — `Config` struct and `Resolve()` to extend with `HealthTimeout` and `HealthInterval` fields
- `internal/compose/run.go` — `RunCompose()` — health polling is called after this returns successfully
- `internal/filetransfer/upload.go` — `ShellQuote()` already exported; reuse for any remote shell arguments in check commands

### Critical Constraints (from CLAUDE.md)
- `CLAUDE.md` — Rule 1 (no InsecureIgnoreHostKey), Rule 3 (separate NewSession() per SSH command), Rule 5 (docker compose v2 only — CHECK-02 enforces this at runtime)

### Prior Phase Context (relevant decisions)
- `.planning/phases/04-core-deploy-loop/04-CONTEXT.md` — D-14: all plugin-level error messages go to `os.Stderr`; D-11: ShellQuote usage pattern for remote commands

</canonical_refs>

<code_context>
## Existing Code Insights

### Reusable Assets
- `filetransfer.ShellQuote(s string) string` — exported in Phase 3; use for quoting any path or argument passed to SSH exec commands in pre-flight checks
- `sshpkg.Dial(ctx, cfg)` — returns `*gossh.Client`; pre-flight sessions and health poll sessions use `client.NewSession()` on the same client opened in `runDeploy()`
- `config.Resolve()` — extend with `HealthTimeout int` and `HealthInterval int` fields following the same flag > file > default pattern already established

### Established Patterns
- **One NewSession() per SSH command:** CHECK-01 through CHECK-07 each get their own session. Health poll each fires a new session per poll tick. Never reuse a session.
- **Session lifecycle:** create → configure → run/output → close. `session.Output(cmd)` for commands that return a result; `session.Run(cmd)` for side-effect commands.
- **Stderr for plugin messages:** `fmt.Fprintf(os.Stderr, ...)` for all warnings and errors. Compose output stays on `os.Stdout` (unchanged from Phase 4).
- **Error wrapping:** `fmt.Errorf("preflight: docker not installed: %w", err)` — keep context prefix short and descriptive.
- **Context propagation:** pass `ctx context.Context` through all check functions to support future timeout/cancellation wiring.

### Integration Points
- `runDeploy()` in `cmd/docker-deploy/main.go`: insert `RunPreflightChecks(ctx, client, resolved)` call after step 6 (`sshpkg.Dial`) and before step 7 (existence check). The SSH `client` is already open.
- `RunCompose()` return path: insert `PollHealth(ctx, client, resolved)` call after `compose.RunCompose()` returns `nil` (success only — skip health poll on compose failure).
- `config.Resolve()` in `internal/config/config.go`: add `HealthTimeout` and `HealthInterval` int fields to `Config`, read `health_timeout` / `health_interval` from `deploy.yaml`, apply defaults (60 / 5).

</code_context>

<specifics>
## Specific Ideas

- User explicitly wants CHECK-03 (daemon not running) to be a warning rather than a block — files can be copied even if Docker daemon is down. Compose will fail naturally with a clear Docker error if daemon is still down at execution time.
- The sudo decision for CHECK-06 (dir creation) is error-driven: try without sudo first, escalate to sudo only on EACCES. No path-prefix heuristics.
- Pre-flight output is designed for Phase 7 extension: the check runner should return structured results (check name, status, message) so Phase 7 can render the live checklist without rewriting the check logic.

</specifics>

<deferred>
## Deferred Ideas

- `--verbose` live checklist output for pre-flight checks (`[✓] Docker v25.0 installed`) — Phase 7 when `--verbose` flag is implemented
- Passwordless sudo setup documentation (separate `.md` file, linked from README.md) — Phase 7
- `--pull always` / image pull control — Phase 7 v2 leftovers

</deferred>

---

*Phase: 5-Pre-flight & Health Polling*
*Context gathered: 2026-05-16*
