# docker-deploy

## What This Is

A Docker CLI plugin (`docker deploy`) that deploys docker-compose projects to remote VPS instances over SSH/SFTP with a single command. It copies local project files to a staging directory on the remote, atomically swaps them into place, runs `docker compose up -d`, and reports per-container health status — no git required on the remote.

**v1.0 shipped 2026-06-05** with 60 plans across 15 phases, 690 commits, ~12,000 LOC Go.

## Core Value

A developer can deploy their local docker-compose project to any SSH-accessible VPS with a single command, no git required on the remote.

## Requirements

### Validated

- ✓ Binary named `docker-deploy` in `~/.docker/cli-plugins/` — PLUG-01/02/03 — v1.0
- ✓ SSH dial with knownhosts TOFU, full auth chain, goroutine dial timeout — CFG-01..05 — v1.0
- ✓ deploy.yaml config (host, path, exclude, skip_env, verbose, healthcheck) with flag precedence — CFG-03/04/05 — v1.0
- ✓ Global config layer (`~/.docker/cli-plugins/deploy.yaml`) — CFG-07 — v1.0
- ✓ SFTP upload with atomic staging + four-step swap with rollback — DEPLOY-02/03 — v1.0
- ✓ Default include/exclude lists; user-extendable exclude via deploy.yaml or --exclude — FILES-01/02/03 — v1.0
- ✓ Full deploy loop: copy → compose up -d → streaming output + exit codes — DEPLOY-01/04/05/06 — v1.0
- ✓ Structured sudo auth fallback (direct → passwordless → interactive prompt) — DEPLOY-07 — v1.0
- ✓ Pre-flight checks: Docker installed, compose v2, daemon running, docker group, dir writable, root warning — CHECK-01..07 — v1.0
- ✓ Post-deploy health polling per container; unhealthy exits non-zero — HEALTH-01/02/03 — v1.0
- ✓ Per-service healthcheck config format (interval, timeout, retries) in deploy.yaml — v1.0
- ✓ `version` and `validate` subcommands; deploy.yaml resolved relative to cwd — v1.0
- ✓ SSH config host alias resolution (HostName/User/Port from ~/.ssh/config) — v1.0
- ✓ Shell completions (bash/zsh) shipped in goreleaser tarballs + Homebrew — v1.0
- ✓ Integration test suite (testcontainers DinD+SSH, all requirements covered) — v1.0
- ✓ Homebrew tap, install.sh, GoReleaser + cosign distribution — v1.0

### Active (Next Milestone)

- [ ] `--init` flag triggers setup wizard for first-time VPS users (INIT-01..04) — Phase 999.1
- [ ] File permissions preserved during SFTP upload (WR-01 — executable scripts lose +x)
- [ ] SSH `Include` directive support in sshconfig parser
- [ ] `--config <path>` flag for non-default config file (CFG-06)
- [ ] Sudo credential caching via system keyring (Phase 14-03)
- [ ] Include field in file filter (strict allowlist mode, FILES-03 complement)

### Out of Scope

- Docker remote context as primary deploy mechanism — files live only locally
- Git on VPS — explicit non-requirement
- Multi-host / named targets (staging, prod) — v2 feature (MULTI-01/02), architecture allows future extension
- Container registry / image push — out of scope; image building is user's responsibility
- Blue-green / zero-downtime — requires reverse proxy in compose.yaml, user-controlled
- Rollback — atomic staging-dir pattern limits blast radius; explicit rollback not in v1
- Web UI / server daemon — CLI-only; different product (Coolify, Dokploy)
- Secrets vault integration — .env copied as-is, user responsible
- docker-compose v1 compatibility — EOL since June 2023; hard-fail pre-flight condition
- Terminal demo recording (deferred from Phase 16 Wave 2)

## Context

**Current state:** v1.0 shipped. Plugin is functional and distributed via Homebrew tap, install.sh, and manual binary. Integration tests cover all v1 requirements automatically against a real SSH daemon.

**Tech stack:** Go 1.26.3 · `github.com/docker/cli` · `golang.org/x/crypto/ssh` · `github.com/pkg/sftp` · `github.com/spf13/cobra` · `github.com/testcontainers/testcontainers-go` · `github.com/charmbracelet/huh` (charmbracelet reserved for init wizard)

**Known technical debt:**
- WR-01: SFTP does not preserve Unix file permissions — entrypoint scripts may lose +x
- WR-02: WalkFiles called twice (count + upload) — minor TOCTOU in file count message
- CR-01: Sudo password may appear in error chain on post-auth sudo command failures
- Phase 09 UAT partial — Homebrew E2E smoke test requires live tagged release
- Phase 02/07/16 VERIFICATION.md: `human_needed` — live SSH host required for full behavioral verification

## Constraints

- **Language**: Go — single binary, no runtime deps, standard for Docker ecosystem tooling
- **No remote git**: VPS only needs Docker + SSH daemon
- **SSH transport**: SCP/SSH only — no Docker daemon socket exposure
- **v1 scope**: Single remote target; config designed for future multi-target extension (v2)
- **InsecureIgnoreHostKey**: Never used — tool copies .env files, MITM is catastrophic

## Key Decisions

| Decision | Rationale | Outcome |
|----------|-----------|---------|
| Docker CLI plugin over standalone binary | Integrates naturally into Docker workflow | ✓ Good — Phase 1 |
| Manual `Resolve(flags, file, defaults)` — no Viper | Viper has flag-override ordering bugs; manual precedence is explicit and testable | ✓ Good — Phase 2 |
| golang.org/x/crypto/ssh + knownhosts (no InsecureIgnoreHostKey) | Tool copies .env files; MITM is catastrophic | ✓ Good — Phase 2 |
| Flags-first, deploy.yaml for persistence | Mirrors docker-compose UX — quick starts + repeat deploy config | ✓ Good — Phase 2 |
| `/tmp/docker-deploy-<ts>` staging (not /opt directly) | /opt may not be writable; /tmp always is; target dir creation is separate step with sudo fallback | ✓ Good — Phase 3 |
| Four-step atomic swap with rollback | new→old→swap→backup-rm; rollback at step 2/3; non-fatal backup removal | ✓ Good — Phase 3 |
| ShouldExclude three-level matching | Exact, prefix, and component scan for deep paths | ✓ Good — Phase 3 |
| --skip-env flag + backup/restore around swap | The atomic swap replaces entire dir; backup before + restore after truly preserves remote .env | ✓ Good — Phase 7 |
| Warning rollup without --verbose | Terse default output; --verbose shows per-file and per-command details inline | ✓ Good — Phase 7 |
| testcontainers DinD+SSH for integration tests | Real SSH daemon instead of mocks; all v1 requirements testable automatically | ✓ Good — Phase 8 |
| Static completions in goreleaser tarballs | No RegisterFlagCompletionFunc; hidden `completion` subcommand for make target; deterministic output | ✓ Good — Phase 10 |
| SudoCreds stores pw as []byte with Zero() wipe | Safe memory handling; single interactive prompt per deploy session | ✓ Good — Phase 13 |
| needsSudo probe in Upload() | `test -w` probe skips sudo scaffolding for user-writable paths — no false prompts | ✓ Good — Phase 13 |
| ErrDeployCancelled sentinel | Confirm-prompt cancellation stops execution cleanly before RunCompose | ✓ Good — Phase 13 |
| `docker deploy version` subcommand | Build metadata via ldflags — no runtime git invocation | ✓ Good — Phase 13 |
| `docker deploy validate` subcommand | Local-only config validation; no SSH connection; fast feedback | ✓ Good — Phase 13 |
| Four-tier Resolve (opts/file/globalFile/projectName/localDir) | Supports per-project and global config layers without breaking existing single-target configs | ✓ Good — Phase 15 |
| retries==0 preserves immediate-fail | Backward compatibility — zero healthcheck config skips polling entirely | ✓ Good — Phase 15 |
| CHECK-05 unconditional + sudo password sanitized from errors | D-11 sudo escalation always attempted; password never leaks in error chains | ✓ Good — Phase 5+13 |
| Phase 6 init wizard → Phase 999.1 backlog | Feature scope too large for v1.0 timeline; deploy works without wizard via manual VPS setup | ⚠️ Revisit for v1.1 |
| Terminal demo recording deferred | Low-priority during active development; deferred to avoid blocking release | — Pending |

## Evolution

This document evolves at phase transitions and milestone boundaries.

**After each phase transition** (via `/gsd-transition`):
1. Requirements invalidated? → Move to Out of Scope with reason
2. Requirements validated? → Move to Validated with phase reference
3. New requirements emerged? → Add to Active
4. Decisions to log? → Add to Key Decisions

**After each milestone** (via `/gsd-complete-milestone`):
1. Full review of all sections
2. Core Value check — still the right priority?
3. Audit Out of Scope — reasons still valid?
4. Update Context with current state

---
*Last updated: 2026-06-05 after v1.0 milestone*
