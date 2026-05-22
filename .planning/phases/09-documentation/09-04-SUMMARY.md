---
phase: 09-documentation
plan: "04"
subsystem: documentation
tags: [docs, prerequisites, troubleshooting, comparison, config-reference]
dependency_graph:
  requires: []
  provides:
    - PREREQUISITES.md
    - TROUBLESHOOTING.md
    - COMPARISON.md
    - DEPLOY_CONFIG.md
  affects: []
tech_stack:
  added: []
  patterns: [markdown-documentation, field-reference-from-source-code]
key_files:
  created:
    - PREREQUISITES.md
    - TROUBLESHOOTING.md
    - COMPARISON.md
    - DEPLOY_CONFIG.md
  modified: []
decisions:
  - "DEPLOY_CONFIG.md field defaults (health_timeout=60, health_interval=5) derived directly from internal/config/config.go Resolve() to guarantee accuracy"
  - "COMPARISON.md uses 9 rows (docker-deploy + 8 comparators) matching D-27; Docker remote context included as a distinct tool from manual SSH"
  - "All examples use placeholder values (sshuser, vps.example.com, myapp) — no real credentials per threat model T-09-04-01"
metrics:
  duration: "2m 54s"
  completed: "2026-05-22T16:59:59Z"
  tasks_completed: 2
  tasks_total: 2
---

# Phase 9 Plan 04: Supporting Documentation Summary

## One-liner

Four supporting docs covering SSH prerequisites, 5-scenario troubleshooting, 8-tool comparison table, and complete deploy.yaml field reference cross-checked against config.go.

## What Was Built

### Task 1: PREREQUISITES.md and TROUBLESHOOTING.md (commit 7db5d6d)

**PREREQUISITES.md** — three sections:
1. SSH Key Setup: check for existing key, `ssh-keygen -t ed25519`, `ssh-copy-id`, connection test
2. Passwordless sudo for sshuser: useradd, docker group membership, `visudo -f /etc/sudoers.d/sshuser` with exact sudoers line, verification command
3. Windows users: WSL2 and Git Bash guidance, plus `go install` alternative for native Windows

**TROUBLESHOOTING.md** — exactly 5 H2 sections:
1. SSH authentication failure — key not on VPS, wrong user, SSH agent not running, passphrase not loaded
2. Unknown host / knownhosts prompt — strict knownhosts policy explanation, `yes` to accept, `ssh` pre-seeding workaround
3. Target directory not writable — three remediation options (passwordless sudo, pre-create as root, use user-owned path)
4. Docker not found on remote — official install script, docker group membership, verification command
5. docker compose v1 detected — apt-get install docker-compose-plugin, official docs link, verification command

### Task 2: COMPARISON.md and DEPLOY_CONFIG.md (commit 6d939ef)

**COMPARISON.md** — 9-row × 10-column table (1 tool name column + 9 dimension columns):
- 9 rows: docker-deploy + 8 comparators (Terraform, Ansible, Docker remote context, Manual SSH scripts, docker-compose + Watchtower, Portainer, Kamal, Full CI/CD)
- 9 dimensions: Docker Compose native, .env/secrets handling, time to first deploy, compose-centric design, SSH best practices, complexity/learning curve, remote dependencies, requires git on VPS, best fit
- "When to use docker-deploy" section (5 bullets: solo dev, single VPS, compose-native, single command, no git on VPS)
- "When NOT to use docker-deploy" section (5 bullets: multi-server, rolling deploys, UI management, IaC requirements, automated pipeline)

**DEPLOY_CONFIG.md** — complete field reference:
- Full deploy.yaml schema example with all 8 TargetConfig fields + version
- Field reference table (Field | Type | Default | Description) for all 8 fields plus version
- Built-in default excludes: all 16 patterns from `defaultExcludes` in config.go including `.terraform/`
- Config precedence: three-tier (CLI flags > deploy.yaml > defaults) with additive exclude merge explanation
- Flags without deploy.yaml equivalent: `--dry-run` and `--verbose` documented as CLI-only

## Deviations from Plan

None — plan executed exactly as written. All content derived from source documents (09-CONTEXT.md decisions D-25 through D-30, internal/config/config.go for DEPLOY_CONFIG.md defaults).

## Threat Surface Scan

No new network endpoints, auth paths, file access patterns, or schema changes introduced. All files are documentation only.

Threat model compliance verified:
- T-09-04-01: All examples use `sshuser`, `vps.example.com`, `myapp` — no real credentials
- T-09-04-02: No real host addresses in troubleshooting steps
- T-09-04-03: `health_timeout` default documented as 60, `health_interval` as 5 — matches `config.go` Resolve() exactly

## Known Stubs

None.

## Self-Check: PASSED

Files created:
- PREREQUISITES.md: FOUND
- TROUBLESHOOTING.md: FOUND
- COMPARISON.md: FOUND
- DEPLOY_CONFIG.md: FOUND

Commits verified:
- 7db5d6d: FOUND (docs(09-04): add PREREQUISITES.md and TROUBLESHOOTING.md)
- 6d939ef: FOUND (docs(09-04): add COMPARISON.md and DEPLOY_CONFIG.md)
