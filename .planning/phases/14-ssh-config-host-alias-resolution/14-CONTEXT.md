# Phase 14: SSH Config Host Alias Resolution - Context

**Gathered:** 2026-05-29
**Status:** Ready for planning

<domain>
## Phase Boundary

Two capabilities delivered as 2 plans (Wave 3 keyring deferred):

1. **SSH config alias resolution** — `--host minipc` (no `ssh://` prefix) resolves via `~/.ssh/config` to `HostName`, `User`, and `Port`; unmatched aliases produce a clear error before any dial attempt
2. **Better deploy.yaml error messages** — distinguish "file not found" from "host field missing/invalid" in `config.Resolve()` and `runDeploy()` error paths

**Wave 3 (keyring) is deferred** — sudo credential caching via OS keychain is out of scope for this phase; promoted to a future phase (Phase 14.x or 15+).

This phase adds no new network protocols and no new external dependencies.

</domain>

<decisions>
## Implementation Decisions

### Alias detection logic

- **D-01:** The rule for detecting an alias is: if the host value has no `ssh://` prefix, treat it as a ssh config alias. No fallback URL-parse attempt — the absence of `ssh://` is the sole signal.
- **D-02:** Both `--host minipc` (CLI flag) and `target.host: minipc` in `deploy.yaml` use the same rule. Alias support is first-class in both input paths. `Resolve()` owns this logic — it normalises the host value before calling `ParseHost()`.
- **D-03:** Alias lookup happens inside `Resolve()`: detect no-`ssh://` prefix → call `sshconfig.LookupHost(alias)` → if found, build a synthetic `ssh://user@hostname:port` string and pass to `ParseHost()`; if not found, return a clear error: `"alias %q not found in ~/.ssh/config"` (not a dial error).
- **D-04:** When `Resolve()` returns an error for an unmatched alias, no SSH connection is attempted. The error distinguishes alias-not-found from a malformed URL.

### SSH config parser (sshconfig.go extension)

- **D-05:** Extend `internal/sshconfig/sshconfig.go` — add a new exported function `LookupHost(configPath, alias string) (HostEntry, bool)` that returns a `HostEntry` struct and a found flag.
- **D-06:** `HostEntry` struct:
  ```go
  type HostEntry struct {
      HostName       string   // resolved hostname (alias if HostName directive absent)
      User           string   // empty string if absent (caller uses OS user or deploy.yaml user)
      Port           int      // 0 if absent (caller defaults to 22)
      IdentityFiles  []string // same as LoadSigners collects today
  }
  ```
- **D-07:** HostName fallback: if a matching Host block has no `HostName` directive, `HostEntry.HostName` is set to the alias label itself. Matches OpenSSH behaviour — the Host label IS the hostname when no HostName is declared.
- **D-08:** Port fallback: if no `Port` directive is present, `HostEntry.Port = 0` (caller applies the default of 22, consistent with how `DialConfig.Port = 0` works today).
- **D-09:** User fallback: if no `User` directive is present, `HostEntry.User = ""` (caller inherits OS username or deploy.yaml user if set).
- **D-10:** The existing `parseIdentityFiles()` helper is refactored into the new `LookupHost()` — both parse the same Host block structure. `LoadSigners()` becomes a thin wrapper that calls `LookupHost()` and loads the keys from `HostEntry.IdentityFiles`.
- **D-11:** `Include` directives are **not** implemented in this phase. A TODO comment is left in `sshconfig.go` noting the limitation and the pattern (recursive file open). Known consequence: Colima-injected hosts (e.g. `Include /Users/mniedre/.colima/ssh_config`) are not reachable via alias until a future phase addresses Include support.

### known_hosts key for aliases

- **D-12:** The resolved `HostName` (e.g. `prod.example.com` or `192.168.1.99`) is used as the `known_hosts` key — not the alias (`minipc`). This matches OpenSSH behaviour and prevents duplicate/conflicting known_hosts entries when the user also uses the native `ssh` CLI.
- **D-13:** The `DialConfig.Hostname` field is set to the resolved `HostName` before calling `Dial()`. No changes to the TOFU or key-mismatch paths are needed.

### Better deploy.yaml error messages (Wave 2)

- **D-14:** `LoadFile()` is extended to distinguish file-not-found from other errors. It returns a second bool `fileExists bool` (or alternatively, `Resolve()` calls `os.Stat` before `LoadFile`). The exact approach (second return value vs. wrapper) is left to the planner; the semantic contract is:
  - No deploy.yaml file + no `--host` flag → error: `"no deploy.yaml found in <dir> and no --host flag provided"`
  - deploy.yaml exists but `target.host` is empty → error: `"deploy.yaml: target.host is not set"`
  - deploy.yaml exists but `target.host` is invalid → existing error message with `"deploy.yaml target.host:"` prefix (already present)
- **D-15:** Both error paths are covered by unit tests in the config package. No SSH connection is attempted in either case.

</decisions>

<canonical_refs>
## Canonical References

**Downstream agents MUST read these before planning or implementing.**

### Existing code to extend

- `internal/sshconfig/sshconfig.go` — current IdentityFile-only parser; `LookupHost()` extends this; `LoadSigners()` becomes a wrapper
- `internal/config/config.go` — `ParseHost()`, `Resolve()`, `LoadFile()` — alias detection and error message improvements land here
- `internal/ssh/client.go` — `Dial()`, `DialConfig` — `Hostname` field must receive the resolved `HostName`, not the raw alias; no changes to TOFU/known_hosts logic needed
- `cmd/docker-deploy/main.go` — `runDeploy()`, `runDryRun()` — error message improvements may surface here if Resolve() error messages are propagated directly

### Memory / feedback

- `.claude/memory/feedback_sudo_rollback.md` — not directly relevant to Phase 14 but keep in mind for any Upload() paths touched

### CLAUDE.md rules relevant to this phase

- Rule 1: No `InsecureIgnoreHostKey()` — unchanged; alias resolution must not weaken host key verification
- Rule 2: SSH dial timeout (goroutine + context.WithTimeout) — unchanged
- Rule 3: Sessions are not reusable — unchanged

</canonical_refs>

<code_context>
## Existing Code Insights

### Reusable Assets

- `internal/sshconfig/sshconfig.go` — `parseIdentityFiles()` already does the Host block + wildcard matching loop. `LookupHost()` reuses this loop and collects `HostName`, `User`, `Port` alongside `IdentityFile` in a single pass. The existing `hostMatches()` and `expandPath()` helpers are reused as-is.
- `config.ParseHost()` — unchanged; it keeps enforcing the `ssh://` scheme. `Resolve()` builds a canonical `ssh://user@hostname:port` string from `HostEntry` fields before calling `ParseHost()`.
- `config.LoadFile()` — currently returns `(FileConfig, error)` where a missing file returns `(FileConfig{}, nil)`. The planner must decide whether to add a third return value or use `os.Stat` at the call site in `Resolve()`.

### Established Patterns

- The "no `ssh://` prefix → alias" detection mirrors the existing "no compose file → auto-detect" pattern in `Resolve()` — detect absence of a signal and fall back to a lookup.
- `DialConfig.Port = 0` → defaults to 22 inside `Dial()` — `HostEntry.Port = 0` follows the same zero-means-default convention.
- All SSH exec commands use a fresh `client.NewSession()` — no changes needed for Phase 14.

### Integration Points

- `config.Resolve()` is where alias lookup is wired in — between host-value detection and `ParseHost()` call.
- `ssh/client.go:buildAuthMethods()` calls `loadSSHConfigKeys(hostname, user)` which calls `sshconfig.LoadSigners()`. After the refactor, `LoadSigners()` is a wrapper over `LookupHost()` — both use the same `HostEntry.IdentityFiles` field. The `loadSSHConfigKeys` call site in `buildAuthMethods` can pass the resolved hostname (from `DialConfig.Hostname`) so key lookup continues to work correctly.

</code_context>

<specifics>
## Specific Ideas

- Real-world test case from user's machine: `cat ~/.ssh/config` contains `Include /Users/mniedre/.colima/ssh_config`. Include support is a TODO — the parser should gracefully skip `Include` lines (no error, just ignore) in Phase 14 and leave a comment marking the TODO.
- `HostEntry.HostName` is the field used for `DialConfig.Hostname` AND the `known_hosts` key — must be the resolved real hostname (e.g. `192.168.1.99`), not the alias (`minipc`).
- Error message examples:
  - Alias not found: `alias "minipc" not found in ~/.ssh/config`
  - No deploy.yaml + no --host: `no deploy.yaml found in /Users/mniedre/myproject and no --host flag provided`
  - deploy.yaml exists, target.host empty: `deploy.yaml: target.host is not set`

</specifics>

<deferred>
## Deferred Ideas

- **Wave 3 — macOS Keychain / system keyring sudo credential caching** — `--clear-credentials`, prompt-and-save flow, `go-keyring` or `99designs/keyring`. Deferred to a future phase (Phase 14.x or later). The `SudoExec`/`SudoCreds` infrastructure from Phase 13 is the hook-in point when this is implemented.
- **`Include` directive support in sshconfig.go** — Needed for Colima-managed hosts (`Include /Users/mniedre/.colima/ssh_config`). Out of scope for Phase 14; leave a TODO comment in `sshconfig.go`. Implement in a follow-on quick task or future phase.

</deferred>

---

*Phase: 14-SSH-Config-Host-Alias-Resolution*
*Context gathered: 2026-05-29*
