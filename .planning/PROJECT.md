# docker-deploy

## What This Is

A Docker CLI plugin (`docker deploy`) that deploys docker-compose projects to remote VPS instances over SSH. It copies project files (compose.yaml, .env, Makefile, etc.) to a target directory on the remote host, runs `docker compose up -d`, and reports deployment health — all without requiring git on the VPS.

## Core Value

A developer can deploy their local docker-compose project to any SSH-accessible VPS with a single command, no git required on the remote.

## Requirements

### Validated

- [x] Configurable via flags (`--host "ssh://user@host:port"`) with `deploy.yaml` override — Validated in Phase 2: SSH Transport & Config
- [x] Default deploy target `/opt/<proj_name>`, configurable via flag/config — Validated in Phase 2: SSH Transport & Config

### Active

- [ ] Docker CLI plugin interface (`docker deploy ...`)
- [ ] Deploy via SSH/SCP: copy project files to remote host
- [ ] Configurable via flags (`--host "ssh://user@host:port"`) with `deploy.yaml` override
- [ ] Default deploy target `/opt/<proj_name>`, configurable via flag/config
- [ ] Smart file copy defaults (compose.yaml, .env, Makefile, README.md) with user-defined include/exclude in deploy.yaml
- [ ] Run `docker compose up -d` on remote after file copy
- [ ] Pre-deployment checks: Docker installed, SSH user in docker group, root user warning
- [ ] First-deploy wizard: detect unconfigured VPS, optionally create dedicated docker SSH user
- [ ] `--init` flag to configure/reconfigure SSH user via root
- [ ] Post-deploy: wait for healthcheck status, report pass/fail; warn if healthcheck not configured
- [ ] `-d` / `--detach` flag: detached mode (fire-and-forget) vs streaming remote logs
- [ ] Optional: configure Docker remote context after deploy for ongoing container observation

### Out of Scope

- Docker remote context as primary deploy mechanism — files live only locally, remote context doesn't solve that
- Git on VPS — explicit non-requirement
- Multi-host / named targets (staging, prod) — v2 feature, architecture must allow future extension
- Container registry / image push — out of scope for v1

## Context

The problem this solves: docker remote context requires project files on the remote, and git adds operational complexity on a VPS. This tool treats the local machine as the source of truth and the VPS as a dumb execution host. SSH/SCP is the transport.

**Plugin convention:** Docker CLI plugins live in `~/.docker/cli-plugins/` as binaries named `docker-<name>`. They receive `docker-cli-plugin-metadata` as the first arg for discovery. The plugin is invoked as `docker deploy [args]`.

**SSH host format:** `ssh://user@host:port` (Docker-compatible URI format)

**Remote user management:** Root SSH access used only during `--init` to create a dedicated deploy user with docker group membership. Day-to-day deploys use the non-root user.

**Health checking:** Uses `docker inspect` to poll container health status. Containers without HEALTHCHECK defined get a warning but deploy still succeeds.

## Constraints

- **Language**: Go — single binary, no runtime deps, standard for Docker ecosystem tooling
- **No remote git**: VPS only needs Docker + SSH daemon
- **SSH transport**: SCP/SSH only — no Docker daemon socket exposure
- **v1 scope**: Single remote target; config designed for future multi-target extension

## Key Decisions

| Decision | Rationale | Outcome |
|----------|-----------|---------|
| Docker CLI plugin over standalone binary | Integrates naturally into Docker workflow, user runs `docker deploy` | ✓ Validated Phase 1 |
| Flags-first, deploy.yaml for persistence | Flags for quick starts, config for repeat deploys — mirrors docker-compose UX | ✓ Validated Phase 2 |
| Manual `Resolve(flags, file, defaults)` — no Viper | Viper has flag-override ordering bugs; manual precedence is explicit and testable | ✓ Validated Phase 2 |
| golang.org/x/crypto/ssh + knownhosts (no InsecureIgnoreHostKey) | Tool copies .env files; MITM is catastrophic | ✓ Validated Phase 2 |
| --skip-env flag + skip_env config | Preserve remote .env across deploy; backup/restore around atomic swap | ✓ Validated Phase 7 |
| Backup/restore remote .env (not skip entirely) | The atomic swap replaces the whole dir; backup before swap + restore after ensures remote .env is truly preserved | ✓ Validated Phase 7 |
| Warning rollup without --verbose | All non-blocking warnings suppressed; single rollup at end; --verbose prints each inline | ✓ Validated Phase 7 |
| `docker deploy version` subcommand | Build metadata (semver tag, git commit, build timestamp, OS/Arch) via ldflags — no runtime git | ✓ Validated Phase 13 |
| `docker deploy validate` subcommand | Local-only config validation, no SSH — fast feedback before deploy | ✓ Validated Phase 13 |
| SudoExec / SudoCreds exported types | Consolidated sudo machinery: single interactive prompt per deploy, []byte credential with Zero() wipe | ✓ Validated Phase 13 |
| needsSudo probe in Upload() | `test -w` OR probe skips all sudo scaffolding for user-writable paths — no false prompts for ~/project deploys | ✓ Validated Phase 13 |
| ErrDeployCancelled sentinel | Confirm-prompt cancellation stops execution cleanly before RunCompose; staging dir cleaned up on cancel | ✓ Validated Phase 13 |
| Smart file defaults + user override | Opinionated but flexible — avoids copying entire project tree inadvertently | ✓ Validated Phase 3 |
| Go | Single binary distribution, no deps on VPS or dev machine beyond the binary | ✓ Validated Phase 1 |

## Evolution

This document evolves at phase transitions and milestone boundaries.

**After each phase transition** (via `/gsd-transition`):
1. Requirements invalidated? → Move to Out of Scope with reason
2. Requirements validated? → Move to Validated with phase reference
3. New requirements emerged? → Add to Active
4. Decisions to log? → Add to Key Decisions
5. "What This Is" still accurate? → Update if drifted

**After each milestone** (via `/gsd-complete-milestone`):
1. Full review of all sections
2. Core Value check — still the right priority?
3. Audit Out of Scope — reasons still valid?
4. Update Context with current state

---
*Last updated: 2026-05-23 after Phase 7 completion*
