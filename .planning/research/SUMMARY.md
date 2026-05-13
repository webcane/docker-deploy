# Research Summary: docker-deploy

**Synthesized from:** STACK.md, FEATURES.md, ARCHITECTURE.md, PITFALLS.md

## Executive Summary

docker-deploy is a Go binary that plugs into the Docker CLI as `docker deploy`, copying local docker-compose project files to a remote VPS over SSH/SFTP and running `docker compose up -d` — no registry, no git on the VPS, no remote daemon. A clear competitive gap exists: no existing CLI-first tool delivers all of docker-compose support, zero remote agents, no registry requirement, and SSH file copy together.

---

## Recommended Stack

| Technology | Version | Role |
|---|---|---|
| Go | 1.24 min, 1.25 CI | Static binary — no VPS runtime |
| `github.com/docker/cli` | match installed Docker | Plugin protocol — `plugin.Run()` |
| `github.com/spf13/cobra` | v1.10.2 (transitive) | Command tree — required by plugin framework |
| `golang.org/x/crypto` | v0.51.0+ (keep current) | SSH client, agent, knownhosts |
| `github.com/pkg/sftp` | v1.13.10 stable | SFTP file copy over existing SSH connection |
| `gopkg.in/yaml.v3` | v3 | deploy.yaml parsing |
| `github.com/charmbracelet/huh` | latest | --init wizard prompts |
| `github.com/testcontainers/testcontainers-go` | latest | Integration tests against real SSH daemon |
| GoReleaser | latest | Cross-platform release builds |

**Note:** Viper is NOT recommended — documented flag-override ordering bugs. Use a manual `Resolve(flags, fileConfig, defaults)` function (~30 lines, fully testable).

---

## Table Stakes (Must Have)

1. Single command deploy via `--host ssh://user@host` — SSH file copy + `docker compose up -d`
2. Smart file inclusion defaults (compose.yaml, .env, Makefile — never node_modules, .git, vendor)
3. Pre-flight checks: Docker v2 compose installed, SSH user in docker group, Docker daemon running
4. Post-deploy health check polling with pass/fail; warning when no HEALTHCHECK defined
5. `deploy.yaml` config file for persistent host, path, include/exclude
6. Streaming remote log output (default) + `--detach` flag for CI/CD
7. Meaningful exit codes

## Differentiators

- `--init` first-deploy wizard: creates deploy user via root SSH, chowns target directory, writes deploy.yaml
- Docker CLI plugin integration (`docker deploy` native UX)
- Root-user warning on deploy (security guardrail)
- Health-check absence warning (educates users)

## Explicit v2+ Defers

Named deploy targets, rollback, zero-downtime/blue-green, Docker context post-deploy, registry integration.

---

## Architecture Overview

Build order (bottom-up by dependency):

```
config resolution → ssh/auth → ssh/client → sftp/upload → preflight → deploy orchestration → CLI plugin layer → wizard
```

**Package layout:**
```
cmd/docker-deploy/main.go       — plugin.Run() entry point
internal/
  cli/                          — cobra commands, flag binding
  config/                       — Resolve(flags, file, defaults), deploy.yaml schema
  ssh/                          — client.go, auth.go, exec.go
  sftp/                         — upload with staging-directory pattern
  preflight/                    — check pipeline
  deploy/                       — orchestration (copy + exec + poll)
  wizard/                       — --init flow, isolated from deploy path
```

**SSH lifecycle:** Dial once → NewSession() per command → client.Close() once. Sessions are NOT reusable.

**SFTP:** Wraps existing `*ssh.Client` via SFTP subsystem — no second TCP connection.

---

## Top 5 Critical Pitfalls

1. **`InsecureIgnoreHostKey()` MITM vulnerability** — use `knownhosts.New()` from day one. Distinguish new-host (interactive add) from changed-host (hard fail). This is non-negotiable for a tool that copies `.env` files.

2. **SSH dial hangs indefinitely** — `ClientConfig.Timeout` only covers TCP dial, not SSH handshake. Wrap entire dial in goroutine + `context.WithTimeout` + select. Confirmed by Go issues #15113, #51926.

3. **Partial deploy leaves remote inconsistent** — copy to temp dir (`/opt/<project>/.deploy-tmp-<timestamp>`), run compose, move atomically. Implement from the start.

4. **`/opt` not writable by deploy user** — `--init` must `mkdir -p /opt/<project>` AND `chown deployuser:deployuser /opt/<project>`. Pre-flight must check `test -w <remote_path>`. Consider defaulting to `~/deploy/<project>` to sidestep.

5. **docker-compose v1 on remote** — pre-flight must run `docker compose version` (v2 plugin check) and fail clearly if only the legacy EOL v1 binary is found.

---

## Recommended Phase Structure (6 Phases)

| Phase | Name | Key Deliverable |
|---|---|---|
| 1 | Plugin Scaffolding | `docker deploy --help` works in Docker CLI; GoReleaser; CI |
| 2 | SSH Transport Layer | Context-based dial, knownhosts, auth chain, exec |
| 3 | Config & File Copy | `deploy.yaml`, include/exclude filter, SFTP staging-dir upload |
| 4 | Pre-flight & Deploy Orchestration | Check pipeline, compose up, health polling, log streaming |
| 5 | CLI & Integration Tests | Full cobra tree, testcontainers E2E, `make install` |
| 6 | First-Deploy Wizard (`--init`) | Root SSH user creation, chown, deploy.yaml generation |

**Phase 1** must lock `docker/cli` version before any business logic to avoid module conflicts.
**Phase 6** is last — architecturally isolated, adds interactive complexity, builds on solid core.
