# Feature Landscape

**Domain:** SSH-based docker-compose deployment CLI plugin
**Researched:** 2026-05-13
**Confidence:** MEDIUM-HIGH (competitive tools well-documented; gap analysis from direct observation)

---

## Competitive Landscape Summary

Tools researched: Kamal, Dokku, Coolify, Dokploy, Haloy, Ansible docker_compose module, GitHub Action SSH deploy patterns, docker remote context.

| Tool | Model | Docker Compose | No Registry Needed | No Server Agent | SSH File Copy | CLI-first |
|------|-------|---------------|--------------------|-----------------|---------------|-----------|
| **Kamal** | CLI | No (image-based) | No (registry required) | Yes | No | Yes |
| **Dokku** | CLI (git push) | No | No | No (server-side) | No | Yes |
| **Coolify** | Web UI | Yes | Yes | No (server daemon) | No | No |
| **Dokploy** | Web UI | Yes | Yes | No (server daemon) | No | No |
| **Haloy** | CLI | No | No (pushes images) | No (haloyd daemon) | Yes (images) | Yes |
| **Ansible** | Playbooks | Yes | Yes | Yes | Yes | No (YAML overhead) |
| **docker context** | Built-in | Yes | Yes | Yes | No (streams daemon) | Yes |
| **docker-deploy** (this) | CLI plugin | Yes | Yes | Yes | Yes | Yes |

**Gap this project fills:** The only CLI-first tool that deploys docker-compose projects via SSH file copy with zero remote agents, zero registry, zero git-on-VPS, and zero server daemons. Ansible covers the technical need but imposes heavy YAML/playbook overhead and is not docker-native. docker context has critical path resolution bugs with volumes and is not a deploy tool.

---

## Table Stakes

Features users expect from an SSH deploy tool. Missing any of these and users reach for a different tool or write their own bash script.

| Feature | Why Expected | Complexity | Notes |
|---------|--------------|------------|-------|
| Single command deploy | The core value prop — `docker deploy --host ssh://user@host` and it works | Low | Must work from zero config with flags only |
| SSH file copy to remote | How local-as-truth is enforced without git | Medium | SCP/SFTP; handle permissions, existing files, partial transfers |
| `docker compose up -d` execution on remote | The actual deployment step | Low | SSH exec; capture exit codes, surface errors clearly |
| Configurable target directory | `/opt/<proj>` default; users deviate for multi-project hosts | Low | Flag + config file; default derived from project dir name |
| Smart file inclusion defaults | Copy compose.yaml, .env, Makefile — not the whole tree | Low | Built-in defaults; not copying node_modules/vendor matters |
| Post-deploy health status | Did it actually work? Show pass/fail | Medium | Poll `docker inspect` health; timeout configurable |
| Error surfacing | SSH errors, compose errors, health failures must be readable | Low | Distinguish infra errors from app errors |
| Idempotent deploys | Re-running deploy must not corrupt running state | Low | `docker compose up -d` is idempotent by nature; file copy must handle existing files |
| `--host` flag (SSH URI format) | Ad-hoc deploys without config file | Low | `ssh://user@host:port` — Docker-compatible format |
| Config file persistence | `deploy.yaml` for repeat deploys without flags | Low | Saves host, path, include/exclude lists |
| Meaningful exit codes | CI/CD pipelines check exit codes | Low | Non-zero on any failure; distinct codes for infra vs app failure |
| Remote pre-flight checks | Verify Docker installed, user in docker group before copy | Medium | Fail fast with actionable error vs obscure compose failure |

---

## Differentiators

Features that set this tool apart. Not expected from SSH deploy tools, but genuinely valued. Competitive advantage.

| Feature | Value Proposition | Complexity | Notes |
|---------|-------------------|------------|-------|
| First-deploy wizard / `--init` | Detects unconfigured VPS, optionally creates deploy user via root SSH — eliminates the most tedious one-time setup step | High | Root SSH only for init; switches to non-root for day-to-day; guided prompts |
| Docker CLI plugin integration (`docker deploy`) | Lives inside docker's UX; no separate tool to install or remember | Medium | Binary in `~/.docker/cli-plugins/`; responds to `docker-cli-plugin-metadata`; feels native |
| Root-user warning on deploy | Security guardrail: warns when deploying as root instead of failing silently | Low | Check effective UID on remote; warn don't block (user may know what they're doing) |
| Health-check absence warning | Informs users their containers have no HEALTHCHECK defined — educates while deploying | Low | `docker inspect` reveals HEALTHCHECK config; useful nudge without blocking |
| Streaming remote logs (default mode) | Developer stays informed — see compose output in real time without SSHing manually | Medium | SSH exec with PTY or piped stdout; interruptible |
| `-d/--detach` mode | Fire-and-forget for CI/CD pipelines that don't need to wait | Low | Skip log streaming and health poll; return 0 if copy+exec succeeded |
| User-defined include/exclude in deploy.yaml | Precise control over what gets copied — no accidental secret directory uploads | Low | Glob patterns; complement the smart defaults |
| Optional Docker context configuration post-deploy | After deploy, configure `docker context` for ongoing container observation without re-deploying | Low | Out-of-scope for core deploy flow but addable as a sub-command |
| No remote dependencies beyond Docker + SSH | The VPS is genuinely dumb — no agent, no daemon, no git, no Python, no curl | Low (architectural) | Enforced by design; worth stating explicitly in UX |

---

## Anti-Features

Features to deliberately NOT build in v1. Each listed with reasoning and what to do instead.

| Anti-Feature | Why Avoid | What to Do Instead |
|--------------|-----------|-------------------|
| Container registry integration | Adds authentication complexity, breaks the "no registry" value prop, pushes toward image-based model not compose-file model | Stay file-copy + compose. If users need registry, Kamal serves them. |
| Git on remote VPS | The explicit non-requirement; adds operational surface area, SSH key management, repo access from VPS | SCP is the transport. git is not needed when local is source of truth. |
| Web UI / dashboard | Scope creep; undermines the CLI-plugin model; Coolify/Dokploy already own that space well | CLI only. Keep it composable and scriptable. |
| Server-side daemon / agent | Increases attack surface, adds install/upgrade lifecycle, makes "dumb VPS" impossible | SSH exec only. Zero persistent remote process from this tool. |
| Multi-host orchestration (v1) | Complex config model, unclear failure semantics for beginners, premature | Architecture must allow it; v2 feature. Named targets (staging/prod) is the natural extension. |
| Blue-green / zero-downtime (v1) | Requires reverse proxy awareness, adds infra coupling, high complexity for small-project audience | Document the limitation; let users add a reverse proxy (Caddy, Traefik) manually. V2 candidate. |
| Secrets management / vault integration | Out of scope; .env is user responsibility; adding vault integration pulls in auth complexity | Warn that .env is copied as-is; recommend gitignoring it; point to external tools for v2. |
| Rollback | Requires snapshot of previous file state + compose state tracking; complex without a registry tag | Document: rollback is re-deploy of previous local state. V2 candidate if image tags are introduced. |
| Docker Swarm / Kubernetes | Wrong abstraction layer for single-VPS compose-first users | Stay on compose. Users wanting orchestration can graduate to Kamal or Coolify. |
| Automatic SSL / reverse proxy management | Coolify/Dokploy own this; huge operational scope | Document: add Caddy or Traefik to your compose.yaml. |

---

## Feature Dependencies

```
SSH connection (auth) ──────────────────────────────────────────────> All features
                                                                            │
Pre-flight checks (Docker installed, docker group) ──────────────────── file copy
                                                                            │
File copy (SCP) ─────────────────────────────────────────────> compose up execution
                                                                            │
compose up -d ──────────────────────────────────────────> health check polling
                                                                            │
health check ───────────────────────────────────────────> pass/fail report

Config file (deploy.yaml) ─────────────────────────────> replaces flag input
--init / first-deploy wizard ──────────────────────────> creates SSH user (root only)
--detach ───────────────────────────────────────────────> skips log stream + health poll
```

**Ordering constraints for implementation:**
1. SSH exec must work before file copy (pre-flight depends on it)
2. Config file persistence is independent but unlocks repeat use (DX milestone)
3. Health check polling depends on `docker compose up -d` succeeding
4. `--init` is independent of core deploy flow; can ship after v1 core

---

## MVP Recommendation

**Must ship (table stakes):**
1. Single command deploy via `--host` flag — SSH file copy + `docker compose up -d`
2. Smart file inclusion defaults (compose.yaml, .env, Makefile; not entire tree)
3. Pre-flight checks: Docker installed, SSH user in docker group, root warning
4. Post-deploy health check poll with pass/fail report
5. Config file (`deploy.yaml`) for persistent host + path + include/exclude
6. Streaming remote log output (default) with `--detach` flag

**Ship in v1 but after core:**
7. `--init` first-deploy wizard (creates deploy user via root SSH)
8. Health-check absence warning (educates users, zero blocking)
9. Docker CLI plugin integration (native `docker deploy` UX)

**Defer to v2:**
- Named deploy targets (staging/prod multi-target)
- Rollback via previous state
- Zero-downtime / blue-green
- Optional Docker context configuration post-deploy

---

## Sources

- Kamal features: https://kamal-deploy.org/ (HIGH confidence — official docs)
- Competitive tool comparison: https://dev.to/ameistad/self-hosted-deployment-tools-compared-coolify-dokploy-kamal-dokku-and-haloy-2npd (MEDIUM — community article, Feb 2026)
- Haloy deploy tool: https://haloy.dev/docs/what-is-haloy.html (MEDIUM — official docs)
- docker context pain points: https://github.com/docker/compose/issues/9075 and https://github.com/docker/compose/issues/11677 (HIGH — confirmed bugs)
- SSH deploy patterns: https://docs.servicestack.net/ssh-docker-compose-deploment (MEDIUM — documented pattern)
- Zero-downtime without registry: https://www.maxcountryman.com/articles/zero-downtime-deployments-with-docker-compose (MEDIUM — community post)
- .env secrets on VPS: https://www.dchost.com/blog/en/managing-env-files-and-secrets-on-a-vps-safely/ (MEDIUM)
- Docker CLI plugin architecture: https://deepwiki.com/docker/cli/3-plugin-architecture (MEDIUM — derived from docker/cli source)
