# docker-deploy

Docker CLI plugin that deploys docker-compose projects to remote VPS via SSH/SFTP. Single command, no git on remote required.

## Project State

Planning docs: `.planning/`
- `PROJECT.md` — project context and decisions
- `ROADMAP.md` — 6-phase execution plan
- `REQUIREMENTS.md` — 27 v1 requirements with traceability
- `STATE.md` — current phase and progress

## GSD Workflow

This project uses the GSD planning workflow.

**Current state:** Roadmap created, ready to plan Phase 1.

**Next step:** `/gsd-discuss-phase 1` or `/gsd-plan-phase 1`

**Phase order:** Plugin Scaffolding → SSH Transport & Config → File Copy → Core Deploy Loop → Pre-flight & Health Polling → Init Wizard

## Key Technical Decisions

- **Language:** Go — single binary, no runtime deps on VPS
- **Plugin protocol:** `github.com/docker/cli` `plugin.Run()` — binary named `docker-deploy` in `~/.docker/cli-plugins/`
- **SSH:** `golang.org/x/crypto/ssh` — knownhosts verification required (no InsecureIgnoreHostKey)
- **File transfer:** `github.com/pkg/sftp` over existing SSH connection
- **Config:** Manual `Resolve(flags, file, defaults)` — no Viper (flag-override bugs)
- **CLI:** `github.com/spf13/cobra` (required by docker/cli plugin framework)
- **Init wizard prompts:** `github.com/charmbracelet/huh`
- **Integration tests:** `github.com/testcontainers/testcontainers-go` against real SSH daemon

## Critical Implementation Rules

1. **Never use `InsecureIgnoreHostKey()`** — this tool copies `.env` files; MITM is catastrophic
2. **SSH dial timeout** — wrap in goroutine + `context.WithTimeout`; `ClientConfig.Timeout` only covers TCP
3. **Atomic file copy** — stage to `/opt/<project>/.deploy-tmp-<timestamp>`, move atomically; never leave partial state
4. **Lock `docker/cli` version first** — transitive dep conflicts are painful to fix later
5. **docker compose v2 only** — `docker compose version` check; v1 EOL is a hard fail

## SSH session model

Dial once → `NewSession()` per command → `client.Close()` once. Sessions are NOT reusable.
SFTP wraps the existing `*ssh.Client` — no second TCP connection.

## Config precedence

`--flag` > `deploy.yaml` > built-in defaults
