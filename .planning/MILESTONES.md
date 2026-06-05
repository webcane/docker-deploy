# Milestones: docker-deploy

---

## ✅ v1.0 — docker-deploy MVP

**Shipped:** 2026-06-05
**Phases:** 1–16 (15 active phases)
**Plans:** 60 total
**Timeline:** 2026-05-13 → 2026-06-05 (19 days)
**Git commits:** 690
**Codebase:** ~12,000 LOC Go

### Delivered

A fully working Docker CLI plugin (`docker deploy`) that deploys docker-compose projects to remote VPS instances over SSH/SFTP with a single command — no git required on the remote. Ships with pre-flight checks, atomic file staging, health polling, shell completions, and a Homebrew tap.

### Key Accomplishments

1. **Plugin scaffold + CI**: `docker deploy` discoverable in Docker CLI; GoReleaser cross-platform builds on every push
2. **SSH transport with security**: knownhosts TOFU verification, goroutine-based dial timeout, full auth chain (key, agent, password) — InsecureIgnoreHostKey never used
3. **Atomic file staging**: SFTP upload to `/tmp/docker-deploy-<ts>`, four-step atomic swap with rollback; single sudo prompt per deploy via SudoCreds/Zero()
4. **Full deploy loop**: streaming `docker compose up -d` output; pre-flight checks (Docker installed, compose v2, docker group, dir writable, root warning); post-deploy health polling per container
5. **Integration test suite**: testcontainers-based DinD+SSH against real daemon — all requirements covered automatically
6. **Distribution**: GoReleaser + cosign, install.sh, Homebrew tap, shell completions (bash/zsh) in release tarballs
7. **CLI UX**: `version` and `validate` subcommands; deploy.yaml resolved relative to cwd; verbose pre-confirm file diff; path-aware sudo detection; SSH config host alias resolution
8. **Healthcheck config format**: per-service `target.healthcheck.{interval,timeout,retries}` in deploy.yaml; strict YAML parsing; global config layer
9. **Extended linter coverage**: gosec, ineffassign, errorlint, wrapcheck, gocritic, gocognit, nestif, prealloc — zero findings

### Known Deferred Items at Close

- 28 items acknowledged and deferred (see STATE.md Deferred Items)
- Includes: 15 quick task state files (all completed), 3 verification gaps (human/live-host needed), 1 UAT partial (Homebrew smoke test), 4 todos (future features/tooling)

### Known Gaps

- **FILES-03 partial**: Exclude-only model — no Include field; users cannot configure a strict file allowlist
- **WR-01**: SFTP does not preserve Unix file permissions — executable scripts may lose +x
- **Init wizard**: `--init` flag (Phase 999.1) deferred to backlog — first-time VPS setup not in v1.0
- **Keyring integration**: sudo credential caching (Phase 14-03) deferred
- **Terminal demo**: README video recording deferred (Phase 16 Wave 2)

### Archived

- `.planning/milestones/v1.0-ROADMAP.md` — full phase details
- `.planning/milestones/v1.0-REQUIREMENTS.md` — all requirements with outcomes
- `.planning/milestones/v1.0-MILESTONE-AUDIT.md` — pre-close audit report (29/29 verified, tech_debt status)

---

*See `.planning/milestones/v1.0-ROADMAP.md` for full phase and plan details.*
