# Phase 2: SSH Transport & Config - Context

**Gathered:** 2026-05-14
**Status:** Ready for planning

<domain>
## Phase Boundary

The plugin can open a verified SSH connection to a remote host and resolve
configuration from flags, `deploy.yaml`, and built-in defaults in the correct
precedence order. The deliverable is a working SSH dial + `--dry-run` flag
that proves connectivity — no file copy, no compose execution.

</domain>

<decisions>
## Implementation Decisions

### SSH Authentication

- **D-01:** Auth chain: SSH agent → system `~/.ssh/config` key resolution. No
  password fallback. The plugin never prompts for a password.
- **D-02:** No `--identity-file` CLI flag. Users configure key paths in
  `~/.ssh/config`. The plugin documents what SSH config is required (Host
  entry with IdentityFile) and surfaces a clear error when auth fails.
- **D-03:** On auth failure: emit a human-readable message pointing to
  `~/.ssh/config` setup, e.g. `SSH auth failed: ensure your key is loaded in
  ssh-agent or configured in ~/.ssh/config for host <host>`.

### Known Hosts / Host Verification

- **D-04:** Unknown host: TOFU prompt — show the host fingerprint and ask the
  user to confirm before connecting. On confirmation, write the entry to
  `~/.ssh/known_hosts` (system default, not a plugin-specific file).
- **D-05:** Changed fingerprint: hard fail with a loud warning (mirrors
  OpenSSH's "WARNING: REMOTE HOST IDENTIFICATION HAS CHANGED!"). Provide an
  interactive confirmation that makes the consequences clear. If the user
  confirms they know what they're doing, output the `ssh-keygen -R <host>`
  command to let them remove the stale entry — never auto-remove or
  auto-override.
- **D-06:** `InsecureIgnoreHostKey` is never used under any code path
  (already locked in CLAUDE.md).

### deploy.yaml — Phase 2 Scope

- **D-07:** Phase 2 reads `host` and `path` fields from `deploy.yaml` only.
  The full schema (include/exclude, identity_file, multi-target) is designed
  in later phases when those features are built.
- **D-08:** `deploy.yaml` is loaded from the current working directory.
- **D-09:** Config precedence: `--flag` > `deploy.yaml` > built-in defaults
  (already locked, confirmed here).
- **D-10:** The v1 structure should be extensible — do not use flat top-level
  keys in a way that would conflict with future multi-target nesting. The
  planner should propose a minimal schema that leaves room for future keys.

### Connectivity Verification

- **D-11:** `--dry-run` flag on the `deploy` command triggers connectivity
  verification. It dials SSH, authenticates, then exits without copying files
  or running compose.
- **D-12:** On success, `--dry-run` prints a full config dump + connection
  info: resolved host, resolved path, auth method used, and the SSH server
  version string (e.g., `SSH-2.0-OpenSSH_8.9`). Exit 0.
- **D-13:** On failure, `--dry-run` prints the connection error and exits
  non-zero.

### Claude's Discretion

- Exact output formatting (colors, prefixes, column alignment) for `--dry-run`
  summary — open to standard CLI conventions used elsewhere in the project.
- SSH dial timeout value (default) — use a sensible default (e.g., 10s)
  consistent with the goroutine + `context.WithTimeout` pattern locked in
  CLAUDE.md.

</decisions>

<canonical_refs>
## Canonical References

**Downstream agents MUST read these before planning or implementing.**

### Project Decisions & Requirements
- `.planning/REQUIREMENTS.md` — CFG-01 through CFG-05 are the requirements
  for this phase; read before writing any plans.
- `.planning/PROJECT.md` §Key Decisions — locked decisions (SSH library
  choice, config resolution pattern, knownhosts requirement, dial timeout
  pattern).
- `CLAUDE.md` — Critical Implementation Rules (§1 through §5) override any
  default patterns. Especially Rule 1 (no InsecureIgnoreHostKey), Rule 2
  (SSH dial timeout pattern), Rule 4 (lock docker/cli version first).

### SSH Implementation
- `go.mod` — current dependency versions; SSH and SFTP libs must be added
  here. `golang.org/x/crypto/ssh` and `github.com/pkg/sftp` are the chosen
  libraries.

### Existing Code
- `cmd/docker-deploy/main.go` — the Phase 1 scaffold; all new commands and
  flags are added to or alongside this root cobra command.

No external specs beyond the above — requirements fully captured in decisions
above and REQUIREMENTS.md.

</canonical_refs>

<code_context>
## Existing Code Insights

### Reusable Assets
- `cmd/docker-deploy/main.go`: Root cobra command with `plugin.Run()` — new
  subcommands or flags (e.g., `--dry-run`, `--host`, `--path`) attach here.
  The plugin.Run() pattern is established and must not change.

### Established Patterns
- Cobra command structure: one root `deploy` command, add persistent flags
  for `--host`, `--path`, `--dry-run`. Phase 2 likely introduces a `connect`
  or `transport` internal package.
- Module path: `github.com/mniedre/docker-deploy` — all new packages live
  under this module.

### Integration Points
- `go.mod` must be updated to add `golang.org/x/crypto/ssh` and
  `github.com/pkg/sftp` before any SSH code is written.
- `cmd/docker-deploy/main.go` is the entry point — all new behavior wires in
  through the cobra root command returned from `plugin.Run()`.

</code_context>

<specifics>
## Specific Ideas

- User explicitly wants the plugin to inform operators what SSH config they
  need to set up before first use — error messages should be actionable and
  point to `~/.ssh/config` specifically.
- `--dry-run` output should show the full resolved config so users can
  debug config precedence issues (flag vs deploy.yaml vs default).

</specifics>

<deferred>
## Deferred Ideas

- **Full deploy.yaml schema** (include/exclude, identity_file, multi-target
  structure) — deferred to the phase that builds those features (Phase 3+).
  The v1 schema design for init wizard output is a Phase 6 concern.
- **`--identity-file` flag** — deferred; rely on `~/.ssh/config` for v1.
  Can be added as a convenience in a later phase if users request it.

</deferred>

---

*Phase: 2-SSH Transport & Config*
*Context gathered: 2026-05-14*
