# Phase 2: SSH Transport & Config - Discussion Log

> **Audit trail only.** Do not use as input to planning, research, or execution agents.
> Decisions are captured in CONTEXT.md — this log preserves the alternatives considered.

**Date:** 2026-05-14
**Phase:** 2-SSH Transport & Config
**Areas discussed:** SSH auth chain, Unknown host handling, deploy.yaml schema, Connectivity test surface

---

## SSH Auth Chain

### Q1: Which SSH auth methods should the plugin try?

| Option | Description | Selected |
|--------|-------------|----------|
| Agent → identity file → password | Mirrors ssh CLI; most users have keys loaded | |
| Agent → identity file (no password) | Safer for automation; never block on prompt | ✓ |
| Identity file only | Explicit --identity-file required always | |

**User's choice:** Agent → identity file (no password)

---

### Q2: Should --identity-file be a flag, deploy.yaml key, or both?

| Option | Description | Selected |
|--------|-------------|----------|
| Both flag and deploy.yaml key | Flag for overrides, config for repeat deploys | |
| deploy.yaml key only | Set once in config | |
| CLI flag only | Always explicit | |

**User's choice (free text):** No `--identity-file` CLI flag and no deploy.yaml param for it. Rely on pre-configured `~/.ssh/config`. Inform user what they need to configure SSH first.

---

### Q3: Auth failure error message

| Option | Description | Selected |
|--------|-------------|----------|
| Point to ssh_config setup | Actionable message citing ~/.ssh/config | ✓ |
| Generic SSH error passthrough | Raw crypto/ssh error | |
| You decide | Leave to implementer | |

**User's choice:** Point to ssh_config setup

---

## Unknown Host Handling

### Q1: Unknown host — TOFU or hard-fail?

| Option | Description | Selected |
|--------|-------------|----------|
| TOFU prompt — show fingerprint, ask to confirm | Mirrors OpenSSH; add to known_hosts on confirm | ✓ |
| Hard fail with instructions | Refuse, tell user to add manually | |
| Auto-accept first connection | Trust silently on first use | |

**User's choice:** TOFU prompt (recommended)

---

### Q2: Where to write confirmed fingerprints?

| Option | Description | Selected |
|--------|-------------|----------|
| ~/.ssh/known_hosts (system default) | Integrates with existing SSH tooling | ✓ |
| ~/.docker/deploy_known_hosts (plugin-specific) | Isolated from system SSH | |
| You decide | Leave to implementer | |

**User's choice:** ~/.ssh/known_hosts (system default)

---

### Q3: Changed fingerprint (possible MITM)?

| Option | Description | Selected |
|--------|-------------|----------|
| Hard fail with clear MITM warning | Loud warning, refuse, tell user to run ssh-keygen -R | |
| Prompt to update | Ask whether to update entry | |
| You decide | Leave to implementer | |

**User's choice (free text):** Show loud warning, use interactive request with obvious approve. Help user remove old SSH entry only if they really know what they are doing. Never auto-override.

**Notes:** Provide the `ssh-keygen -R <host>` command as guidance but let the user run it. Never execute it automatically.

---

## deploy.yaml Schema

### Q1: v1 schema structure

| Option | Description | Selected |
|--------|-------------|----------|
| Named default target from day one | targets: default: host/path — v2 just adds more targets | |
| Flat top-level keys for v1, migrate in v2 | host/path at top level; simpler for v1 | |
| Nested under a 'deploy' key | Namespaced, middle ground | |

**User's choice (free text):** deploy.yaml not for Phase 2. Later.

**Clarification follow-up:**

### Q2: Phase 2 scope for deploy.yaml

| Option | Description | Selected |
|--------|-------------|----------|
| Minimal: just host + path fields for Phase 2 | Full schema designed later | ✓ |
| Full schema locked now | Design complete structure today | |
| You decide | Leave to planner | |

**User's choice:** Minimal — just host + path for Phase 2.

---

## Connectivity Test Surface

### Q1: How does an operator verify SSH connectivity?

| Option | Description | Selected |
|--------|-------------|----------|
| docker deploy ping subcommand | Dedicated command: dial, print server version, exit 0 | |
| --dry-run flag on deploy command | Reuses main command surface | ✓ |
| Implicit — SSH error on failed deploy is enough | No dedicated verification step | |

**User's choice:** --dry-run flag

---

### Q2: --dry-run success output

| Option | Description | Selected |
|--------|-------------|----------|
| Brief connection confirmation | Connected: user@host:22 (SSH-2.0-...) | |
| Full config dump + connection info | Resolved config + SSH server version | ✓ |
| Silent success, exit 0 | No output on success | |

**User's choice:** Full config dump + connection info

---

## Claude's Discretion

- Exact output formatting (colors, prefixes, column alignment) for `--dry-run` summary
- SSH dial timeout default value (e.g., 10s)

## Deferred Ideas

- **Full deploy.yaml schema** (include/exclude, multi-target structure) — deferred to later phases
- **`--identity-file` flag** — deferred; rely on `~/.ssh/config` for v1
