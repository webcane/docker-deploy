# Phase 14: SSH Config Host Alias Resolution - Discussion Log

> **Audit trail only.** Do not use as input to planning, research, or execution agents.
> Decisions are captured in CONTEXT.md — this log preserves the alternatives considered.

**Date:** 2026-05-29
**Phase:** 14-ssh-config-host-alias-resolution
**Areas discussed:** Alias detection logic, SSH config parser approach, Keyring scope & library, known_hosts key for aliases

---

## Alias Detection Logic

| Option | Description | Selected |
|--------|-------------|----------|
| No prefix → alias | If value has no 'ssh://' prefix, treat as ssh config alias automatically | ✓ |
| Try URL parse; fall back to alias | Attempt url.Parse() first; if no 'ssh' scheme, try alias lookup | |
| New --alias flag | Keep --host for full URLs; add --alias for short names | |

**User's choice:** No prefix → alias

---

| Option | Description | Selected |
|--------|-------------|----------|
| Inside Resolve() — resolve alias before ParseHost() | Resolve() checks for no 'ssh://', looks up ssh config, builds synthetic URL | ✓ |
| In runDeploy() in main.go — after Resolve() returns | Resolve() leaves cfg.Host zero; main.go does lookup before Dial() | |
| Inside Dial() in the ssh package | Dial() detects alias and resolves at connect time | |

**User's choice:** Inside Resolve()

---

| Option | Description | Selected |
|--------|-------------|----------|
| Yes — same rule everywhere | If target.host in deploy.yaml has no ssh:// prefix, treat as alias too | ✓ |
| No — --host flag only | deploy.yaml always requires full ssh:// URL | |

**User's choice:** Yes — same rule everywhere

---

| Option | Description | Selected |
|--------|-------------|----------|
| Clear error at resolve time — 'alias not found in ssh config' | Resolve() returns error before any SSH connection | ✓ |
| Fall through to raw dial attempt using alias as hostname | Pass alias as-is to Dial() and let connection fail | |
| Warn but attempt dial anyway | Print warning, then try connecting using alias as hostname | |

**User's choice:** Clear error at resolve time

**Notes:** User clarified the final intent: alias detection should cover both forms — `--host ssh://user@hostname:port` (full URL) and `--host minipc` (alias). Both are first-class supported input forms.

---

## SSH Config Parser Approach

| Option | Description | Selected |
|--------|-------------|----------|
| Extend existing sshconfig.go | Add HostName/User/Port extraction to existing minimal parser | ✓ |
| Add kevinburke/ssh_config library | External library handling Include, Match, full grammar | |
| Shell out to 'ssh -G minipc' | Use ssh CLI to get resolved config; avoids parsing | |

**User's choice:** Extend existing sshconfig.go

---

| Option | Description | Selected |
|--------|-------------|----------|
| HostEntry{HostName, User, Port, IdentityFiles []string} | Single struct lookup returning all Host block fields at once | ✓ |
| Separate lookup functions per field | GetHostName(), GetUser(), GetPort() — multiple parses | |

**User's choice:** HostEntry struct

---

| Option | Description | Selected |
|--------|-------------|----------|
| Use the alias as the hostname | If no HostName directive, Host label IS the hostname | ✓ |
| Return an error — require explicit HostName | Force explicit HostName in every Host block | |

**User's choice:** Use the alias as the hostname (matches OpenSSH behaviour)

**Notes:** User revealed their `~/.ssh/config` uses `Include /Users/mniedre/.colima/ssh_config` (Colima injects its VM hosts via Include). User decision: defer Include support to a TODO rather than implement now. The parser will silently skip Include lines in Phase 14.

---

## Keyring Scope & Library

| Option | Description | Selected |
|--------|-------------|----------|
| Include in Phase 14 (as planned) | Implement per-host sudo password caching via OS keyring now | |
| Defer to its own phase | Phase 14 ships alias resolution + error messages only | ✓ |

**User's choice:** Defer to its own phase

---

## known_hosts Key for Aliases

| Option | Description | Selected |
|--------|-------------|----------|
| Resolved HostName (e.g. prod.example.com) | Consistent with OpenSSH; prevents duplicate known_hosts entries | ✓ |
| The alias name (e.g. minipc) | Simpler but conflicts with native ssh CLI known_hosts entries | |

**User's choice:** Resolved HostName. User clarified: the hostname used is the resolved `HostName` value from ssh config — e.g. `github.com` or IP `192.168.1.99`.

---

## Claude's Discretion

- Implementation detail of how `LoadFile()` exposes file-existence for error messages (second bool return vs. `os.Stat` at call site in `Resolve()`) — deferred to planner.

## Deferred Ideas

- **Wave 3 — macOS Keychain / system keyring** — Deferred to a future phase. `SudoExec`/`SudoCreds` from Phase 13 is the hook-in point.
- **Include directive support in sshconfig.go** — Needed for Colima hosts. Out of scope for Phase 14; leave TODO comment in `sshconfig.go`.
