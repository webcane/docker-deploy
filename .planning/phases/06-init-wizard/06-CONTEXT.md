# Phase 6: Init Wizard - Context

**Gathered:** 2026-05-19
**Status:** Ready for planning

<domain>
## Phase Boundary

`docker deploy --init` runs an interactive wizard (powered by `huh`) that:
1. Collects SSH connection details, project config, and target directory via guided prompts
2. Connects to the VPS, probes its state (root check, docker group, sudo access), and optionally sets up a deploy user when connected as root
3. Writes `deploy.yaml` to the project root with the full config set

The wizard is explicitly invoked with `--init`. It is NOT auto-triggered from deploy failures. The deploy loop continues to fail fast with user-friendly errors when directory creation fails.

</domain>

<decisions>
## Implementation Decisions

### SSH Connection & Auth

- **D-01:** Wizard connects as whatever SSH user the user provides — root or deploy user. A single connection is used (no separate root connection). INIT-01's "root SSH credentials" wording is superseded by D-01 + D-02: the wizard accepts any user and detects root post-connect.
- **D-02:** On connect, run `id -u` (or equivalent) to detect if the connected user is root. Root triggers D-03; non-root continues normally.
- **D-03:** If connected as root: show visible warning ("You are connected as root. Consider creating a dedicated deploy user.") and offer partial-assist setup (see D-19). Do NOT block the wizard — root + warning is acceptable.
- **D-04:** Check the connected user's group membership: if not in `docker` group → show a warning (not a block). If not in `sudo` group → note it, relevant for D-07.
- **D-05:** SSH key resolution: if the provided hostname matches an entry in `~/.ssh/config` → use its `IdentityFile`. Otherwise → present a list of keys found in `~/.ssh/` for the user to select.
- **D-06:** If the SSH key is not loaded in the agent → prompt for key password (wizard is interactive, has TTY — this is fine).
- **D-07:** Sudo password handling: try sudo commands without password first. If passwordless sudo is not configured and sudo is required → prompt for sudo password interactively (wizard runs in a TTY, unlike the deploy loop). Consistent with Phase 3's `term.ReadPassword` pattern.

### Wizard Question Sequence

- **D-08:** **Host prompt** — accepts `user@host`, `user@ip`, or bare hostname/IP. If bare (no `user@` prefix) → ask for username as a follow-up prompt.
- **D-09:** **SSH key selection** — after host is known: check `~/.ssh/config` for a matching `Host` entry and use its `IdentityFile`. If no config match → present list of key files from `~/.ssh/` for user to pick.
- **D-10:** **Compose file** — only asked if neither `compose.yaml` nor `docker-compose.yml` is found in the current working directory.
- **D-11:** **Project name** — default: `filepath.Base(cwd)`. Override with compose label (`name:` key) or DOCKER label if present.
- **D-12:** **Target directory** — default: `/opt/<project>`. User can set any path (e.g., `~/<project>`). The chosen path drives whether sudo is needed (D-13).
- **D-13:** **Deploy mode** — ask: "Copy files only" vs "Copy files + run `docker compose up`". This sets the default behavior in deploy.yaml.

### Sudo Usage (conditional, error-driven)

- **D-14:** Use sudo only when the target path requires it. Try operations without sudo first. If the attempt fails with a permission error (EACCES / non-zero exit) → retry with sudo. Do NOT hard-code path-prefix heuristics. Consistent with Phase 5 D-10.
- **D-15:** If `~/<project>` is chosen as target → no sudo needed at all.
- **D-16:** If sudo is required but passwordless sudo is not configured → prompt for sudo password interactively (D-07 path). Max 3 attempts (consistent with Phase 3 pattern).

### Auto-trigger Mechanics (INIT-02 override)

- **D-17:** INIT-02 ("wizard offers to run init on first deploy if dir not ready") is **overridden**. The `--init` wizard is explicit-only. When `docker deploy` fails to create the target directory: show a user-friendly error message. Full OS-level error detail available via `--verbose`. Do NOT suggest running `--init` in the error message — `--init` does not help with mid-deploy directory permission failures.
- **D-18:** `--init` has two distinct responsibilities: (1) write `deploy.yaml` (config setup); (2) when connected as root, assist with deploy user creation (infrastructure setup). These happen in one wizard run — the root-detection path (D-02/D-03) determines which infrastructure steps are offered.

### deploy.yaml Write Behavior

- **D-19:** Write full field set: `host`, `user`, `path`, `ssh_key`, `compose_file`, `project_name`. This supersedes INIT-04's minimal list ("host, user, and path") — the full set makes `deploy.yaml` self-contained and removes `~/.ssh` resolution at deploy time.
- **D-20:** If `deploy.yaml` already exists in the project root → ask before overwriting. Show a diff-style preview of what will change. Default: No (do not overwrite).

### Root-User Deploy User Setup ("partial assist")

- **D-21:** When connected as root: after warning (D-03), offer to execute specific limited steps: create a dedicated deploy user + add to docker group. Skip sensitive steps (sudoers configuration). Each step is confirmed by the user before execution.
- **D-22:** Do NOT automate sudoers configuration — this is a sensitive file and a security risk. Print the exact command for the operator to run manually instead.
- **D-23:** After successful deploy user creation: wizard continues normally and writes `deploy.yaml` using the new deploy user's credentials (not root).

### Completion

- **D-24:** When `--init` finishes and `deploy.yaml` is written successfully → ask: "deploy.yaml written. Run `docker deploy` now? [y/N]". If yes, invoke the deploy flow inline.

### Claude's Discretion

- Exact `huh` form/group structure (single-page vs multi-step pager) — planner should use whatever `huh` layout is most readable for the 6-step question sequence.
- Error message copy for the user-friendly target-dir failure (D-17) — keep it short and actionable.
- Whether to print a summary of the written `deploy.yaml` at the end of the wizard before the "run now?" prompt.

</decisions>

<canonical_refs>
## Canonical References

**Downstream agents MUST read these before planning or implementing.**

### Requirements
- `.planning/REQUIREMENTS.md` §INIT-01 through INIT-04, INIT-EXT-01, INIT-EXT-02 — the 6 requirements this phase satisfies. Note D-01, D-17, D-19 override the literal wording of INIT-01, INIT-02, and INIT-04 respectively.

### Roadmap
- `.planning/ROADMAP.md` §Phase 6 — goal, success criteria, and dependency chain.

### Prior phase context (sudo and SSH patterns)
- `.planning/phases/05-preflight-health-polling/05-CONTEXT.md` — D-07 through D-10 (sudo mechanics: passwordless assumption, error-driven escalation, error message format, path-agnostic mkdir with EACCES retry). Phase 6 wizard extends this with interactive sudo password prompt (D-07 in this file).
- `.planning/phases/03-file-copy/03-CONTEXT.md` — interactive sudo password prompt pattern (`term.ReadPassword`, 3 retries, graceful fallback) used in Phase 3. Wizard reuses this UX pattern.

### Existing code
- `cmd/docker-deploy/main.go` — `runDeploy()` entry point. `runInit()` slots in as a parallel entry point triggered by the `--init` flag.
- `internal/config/config.go` — `Config` and `TargetConfig` structs. Wizard output must write a `deploy.yaml` that Resolve() can parse without changes (field names must match yaml tags).
- `internal/ssh/` — existing SSH dial + knownhosts pattern. Wizard reuses the same dial path (no InsecureIgnoreHostKey).

### Rules
- `CLAUDE.md` — Rule 1 (no InsecureIgnoreHostKey, ever), Rule 2 (SSH dial timeout pattern), Rule 3 (separate NewSession per command, sessions not reusable).

### Library
- `github.com/charmbracelet/huh` — already in `go.mod`. Wizard uses `huh` for all interactive prompts. Verify it's present before planning (`go list -m github.com/charmbracelet/huh`).

</canonical_refs>

<code_context>
## Existing Code Insights

### Reusable Assets
- `internal/ssh/` dial + knownhosts: wizard reuses the same SSH connection path — same TOFU flow, same timeout pattern.
- Phase 3 `term.ReadPassword` sudo prompt (3 retries, graceful fallback message): wizard's interactive sudo password prompt should follow the same UX.
- `internal/config/config.go` `Resolve()` and yaml tags: wizard must write deploy.yaml fields using the exact yaml tags from `TargetConfig` to remain Resolve()-compatible.
- `filetransfer.ShellQuote()`: available for quoting any path or username args in wizard SSH exec commands.

### Established Patterns
- **One NewSession per command**: every SSH exec in the wizard (id -u, group check, mkdir, chown, useradd) gets its own `client.NewSession()`.
- **Error-driven sudo**: try without sudo first, escalate on EACCES (Phase 5 D-10). Don't assume `/opt` requires sudo.
- **Deduplicated warnings**: "passwordless sudo not configured" warning prints once (Phase 5 quick task pattern).

### Integration Points
- `runInit()` in `main.go` is the new entry point, parallel to `runDeploy()`. The `--init` flag routes there.
- `deploy.yaml` write goes to `filepath.Join(cwd, "deploy.yaml")`.
- After wizard completes + user confirms "run now" → call `runDeploy()` directly (same process, no subprocess).

</code_context>

<specifics>
## Specific Ideas

- **Host prompt format**: wizard should accept all of: `user@host`, `user@192.168.1.99`, `user@minipc`, `192.168.1.99`, `minipc`. If no `user@` prefix → follow-up prompt for username.
- **Key list from `~/.ssh/`**: filter to common key filename patterns (`id_rsa`, `id_ed25519`, `id_ecdsa`, and any `*.pub` counterparts — list the private key, not the `.pub`).
- **Project name from compose**: if `compose.yaml` has a top-level `name:` key → use that as the project name default.
- **Root warning copy**: "You are connected as root. It is recommended to create a dedicated non-root deploy user. This wizard can help set one up."
- **partial-assist deploy user setup**: wizard asks: "Create a deploy user for this VPS? [y/N]" → if yes, prompts for username, runs `useradd -m <user>` and `usermod -aG docker <user>` via SSH, then prints the sudoers command to run manually.

</specifics>

<deferred>
## Deferred Ideas

- **sudoers auto-configuration**: intentionally excluded from `--init` (D-22). Print the exact command for the operator to run manually. Could be revisited in a later phase.
- **INIT-02 auto-trigger from deploy failures**: overridden by D-17. Could be re-evaluated if users report confusion, but the current judgment is that `--init` doesn't help with runtime permission failures.
- **SSH config write**: wizard could offer to write an `~/.ssh/config` entry for the host. Deferred — out of scope for v1.
- **Verbose deploy user setup feedback**: detailed output of user creation steps behind `--verbose`. Deferred to Phase 7 (verbose flag).

</deferred>

---

*Phase: 6-init-wizard*
*Context gathered: 2026-05-19*
