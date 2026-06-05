# Milestone v1.0 — Project Summary

**Generated:** 2026-06-05
**Purpose:** Team onboarding and project review

---

## 1. Project Overview

**docker-deploy** is a Docker CLI plugin (`docker deploy`) that deploys local docker-compose projects to remote VPS instances over SSH. It copies project files (compose.yaml, .env, Makefile, etc.) to a target directory on the remote host, runs `docker compose up -d`, and reports deployment health — all without requiring git on the VPS.

**Core value:** A developer can deploy their local docker-compose project to any SSH-accessible VPS with a single command, no git required on the remote.

**Target users:** Developers who want a simpler alternative to CI/CD pipelines for projects that just need `scp + compose up`. Designed for single-target deploys where the local machine is the source of truth and the VPS is a dumb execution host.

**Status at v1.0:** All 16 phases delivered and verified. The tool is installable via manual binary, install script, or Homebrew tap. It supports SSH config alias resolution, per-service healthcheck polling configuration, shell tab completion, and a suite of pre-flight checks.

---

## 2. Architecture & Technical Decisions

### Language & Distribution
- **Decision:** Go — single binary, no runtime dependencies on VPS or developer machine
  - **Why:** Standard for Docker ecosystem tooling; cross-platform binary via GoReleaser; no dependency management on the remote host
  - **Phase:** 1 (Plugin Scaffolding)

- **Decision:** Docker CLI plugin via `plugin.Run()` from `github.com/docker/cli` — binary named `docker-deploy` in `~/.docker/cli-plugins/`
  - **Why:** Integrates naturally into Docker workflow; users run `docker deploy` without learning a new tool name; plugin discovery handled automatically by the Docker CLI
  - **Phase:** 1 (Plugin Scaffolding)

### SSH & Security
- **Decision:** `golang.org/x/crypto/ssh` + knownhosts TOFU — `InsecureIgnoreHostKey()` is forbidden
  - **Why:** The tool copies `.env` files containing production secrets; a MITM attack is catastrophic. TOFU prompts the user on first connect and writes to `~/.ssh/known_hosts` on confirmation
  - **Phase:** 2 (SSH Transport & Config)

- **Decision:** SSH dial timeout via goroutine + `context.WithTimeout` — not `ClientConfig.Timeout` alone
  - **Why:** `ClientConfig.Timeout` only covers TCP establishment; the goroutine+context pattern covers the full SSH handshake and prevents indefinite hangs
  - **Phase:** 2 (SSH Transport & Config)

- **Decision:** Auth chain: SSH agent → `~/.ssh/config` key resolution. No password fallback, no `--identity-file` flag in v1
  - **Why:** Keeps the auth path simple and predictable; users configure identity files in `~/.ssh/config` per host
  - **Phase:** 2 (SSH Transport & Config)

### File Transfer
- **Decision:** Atomic file copy — stage to `/tmp/docker-deploy-<timestamp>` via SFTP, then `mv` atomically to target directory
  - **Why:** A failed mid-copy must never leave the remote in a partial-deploy state; `/tmp` is always writable (avoiding permission complications); the atomic `mv` means the target is either the old version or the new version, never a mix
  - **Phase:** 3 (File Copy)

- **Decision:** Copy-everything-minus-excludes model (no include whitelist)
  - **Why:** An include whitelist requires the user to maintain it; exclude-only is safer — the default behavior is broad, with built-in excludes for `.git/`, `node_modules/`, `vendor/`, `*.log`, `.DS_Store`, and many more
  - **Phase:** 3 (File Copy)

- **Decision:** `.env` is never in the default excludes — it is the core value proposition
  - **Why:** Copying `.env` to the remote is the primary reason this tool exists; removing it from the defaults would silently break the most important use case
  - **Phase:** 3 (File Copy)

- **Decision:** Four-step atomic swap with rollback (staging→new, remoteBase→old, new→remoteBase, rm backup)
  - **Why:** Ensures the target is never corrupted even if the `mv` sequence is interrupted; rollback restores the original if step 2 or 3 fails
  - **Phase:** 3 (File Copy, gap closure)

### Configuration
- **Decision:** Manual `Resolve(FlagOpts, file, defaults)` — no Viper
  - **Why:** Viper has flag-override ordering bugs; manual precedence (`--flag > deploy.yaml > built-in defaults`) is explicit, testable, and predictable
  - **Phase:** 2 (SSH Transport & Config)

- **Decision:** `FlagOpts` struct for `Resolve()` signature (replacing positional parameters)
  - **Why:** Positional parameters become unmanageable as the number of flags grows (8+ by Phase 7); a struct is self-documenting and makes call sites readable
  - **Phase:** 7 (v2 Leftovers)

- **Decision:** Four-tier healthcheck config precedence: CLI flags > local `deploy.yaml target.healthcheck` > global `~/.docker/cli-plugins/deploy.yaml` > absent (skip polling)
  - **Why:** Allows global defaults without hardcoding values in code; omitting the `healthcheck:` block is the supported way to skip health polling
  - **Phase:** 15 (Deploy Healthcheck Config Format)

- **Decision:** `yaml.NewDecoder` with `KnownFields(true)` — YAML typos are hard errors
  - **Why:** Silent ignoring of unknown fields masks typos like `retrise: 3`; strict mode surfaces the error at parse time rather than producing silent zero-values
  - **Phase:** 15 (gap closure)

### Sudo / Elevated Paths
- **Decision:** `SudoExec` / `SudoCreds` exported types — single interactive prompt per deploy, `[]byte` credential with `Zero()` wipe
  - **Why:** Multiple SSH exec calls per deploy (mkdir, mv, rm, rollback) previously prompted for sudo separately; the credential is cached in a `SudoCreds` struct and zeroed after `Upload()` returns; `[]byte` can be zeroed where Go strings cannot
  - **Phase:** 13 (CLI Subcommands & Deploy UX)

- **Decision:** Path-aware `needsSudo` probe using `test -w path.Dir(remoteBase)` — skip all sudo scaffolding for user-writable paths
  - **Why:** User-writable paths (e.g. `~/myproject`) should never prompt for a sudo password; the probe is parent-only so a user-owned target in a root-owned parent correctly triggers sudo
  - **Phase:** 13 (CLI Subcommands & Deploy UX)

### Deploy UX
- **Decision:** Warning rollup without `--verbose` — single rollup message at end; inline detail only with `--verbose`
  - **Why:** Non-blocking warnings (root user, sudo not configured) clutter normal output; operators who want detail use `--verbose`, others get a clean deploy log
  - **Phase:** 7 (v2 Leftovers)

- **Decision:** `docker deploy version` and `docker deploy validate` subcommands
  - **Why:** `version` provides build metadata (semver, commit, timestamp, OS/arch) via ldflags injection — no runtime git; `validate` gives fast local-only config validation before a deploy attempt (no SSH required)
  - **Phase:** 13 (CLI Subcommands & Deploy UX)

- **Decision:** `--skip-env` flag — backup/restore remote `.env` around the atomic swap rather than skipping the copy entirely
  - **Why:** The atomic swap replaces the whole target directory; a simple skip would destroy the remote `.env`; backup before swap + restore after ensures the remote `.env` is truly preserved
  - **Phase:** 7 (v2 Leftovers)

### SSH Config Alias Resolution
- **Decision:** `--host minipc` resolves via `~/.ssh/config` — no `ssh://` prefix = alias detection signal
  - **Why:** Users already have SSH config for their hosts; requiring `ssh://user@host:port` is friction that makes the tool less convenient than native `ssh`
  - **Phase:** 14 (SSH Config Host Alias Resolution)

### Testing
- **Decision:** Testcontainers-based integration tests against real SSH daemon (`DinD+SSH` custom image, four users)
  - **Why:** Mocked SSH tests cannot verify real auth chain, TOFU, knownhosts, SFTP atomicity, or compose execution; four-user setup (`root`, `nosudouser`, `sudopassuser`, `sshuser`) covers all permission paths
  - **Phase:** 8 (Integration Tests)

---

## 3. Phases Delivered

| Phase | Name | Status | One-Liner |
|-------|------|--------|-----------|
| 1 | Plugin Scaffolding | ✅ Complete | `docker deploy --help` works in Docker CLI; module locked; CI configured |
| 2 | SSH Transport & Config | ✅ Complete | Verified SSH dial (knownhosts, timeout, auth chain) + config resolution |
| 3 | File Copy | ✅ Complete | Atomic SFTP staging-dir upload with smart include/exclude filter |
| 4 | Core Deploy Loop | ✅ Complete | Full `copy → compose up` cycle with output streaming and exit code propagation |
| 5 | Pre-flight & Health Polling | ✅ Complete | 7 pre-flight checks + post-deploy container health polling |
| 7 | v2 — Leftovers | ✅ Complete | `--skip-env`, `--verbose`, expanded default excludes |
| 8 | Integration Tests | ✅ Complete | Testcontainers suite against real SSH daemon; all requirements covered |
| 9 | Distribution & Documentation | ✅ Complete | GoReleaser + cosign, install.sh, Homebrew tap, full README + docs |
| 10 | Shell Tab Completion | ✅ Complete | Static bash/zsh completions via hidden `completion` subcommand; shipped in tarballs |
| 11 | CI & Tooling Polish | ✅ Complete | Codecov, Actions hygiene, golangci-lint enforcement, Homebrew symlink lifecycle |
| 12 | Docs Polish | ✅ Complete | Help text fix, README value prop, INSTALL.md restructure |
| 13 | CLI Subcommands & Deploy UX | ✅ Complete | `version`/`validate` subcommands, SudoExec refactor, verbose diff, path-aware sudo |
| 14 | SSH Config Host Alias Resolution | ✅ Complete | Short host aliases via `~/.ssh/config`; better deploy.yaml error messages |
| 15 | Deploy Healthcheck Config Format | ✅ Complete | `target.healthcheck:` block with Docker duration strings, retries, strict YAML parsing |
| 16 | Release Tooling Enhancement | ✅ Complete | Pre-release gates (test/lint/integration), STATE.md update, extended linter coverage |
| 6 | Init Wizard | ⏸ Backlog | `--init` wizard for first-deploy VPS setup (BACKLOG — not part of v1.0) |

**Phase 6 (Init Wizard) was originally planned but moved to backlog.** The atomic file copy + sudo fallback in Phase 3/13 covers the practical need (creating `/opt/<project>` with sudo) without requiring a full wizard.

---

## 4. Requirements Coverage

### Plugin (PLUG)
- ✅ **PLUG-01**: `docker-deploy` binary in `~/.docker/cli-plugins/` responds to `docker deploy` — Phase 1
- ✅ **PLUG-02**: `docker deploy --help` displays usage in Docker CLI — Phase 1
- ✅ **PLUG-03**: Plugin responds to `docker-cli-plugin-metadata` with valid JSON — Phase 1

### Deploy (DEPLOY)
- ✅ **DEPLOY-01**: `docker deploy --host ssh://user@host:port` works — Phase 4
- ✅ **DEPLOY-02**: Files copied via SFTP over existing SSH connection — Phase 3
- ✅ **DEPLOY-03**: Files staged to temp dir first, then moved atomically — Phase 3
- ✅ **DEPLOY-04**: `docker compose up -d` executed on remote after file copy — Phase 4
- ✅ **DEPLOY-05**: Plugin exits non-zero if any step fails — Phase 4
- ✅ **DEPLOY-06**: Deploy output streamed to local terminal — Phase 4
- ⚠️ **DEPLOY-07**: Structured auth fallback (direct → passwordless sudo → interactive sudo) — Implemented in Phase 13 via `SudoExec`; integration test coverage for interactive-sudo path is a known stretch goal

### File Management (FILES)
- ✅ **FILES-01**: Default file detection (compose.yaml / docker-compose.yml, .env, Makefile, README.md) — Phase 3
- ✅ **FILES-02**: Default exclude list (.git/, node_modules/, vendor/, *.log, .DS_Store, __pycache__/) extended further in Phase 7 — Phase 3/7
- ✅ **FILES-03**: User can extend exclude list via deploy.yaml or `--exclude` — Phase 3

### Configuration (CFG)
- ✅ **CFG-01**: `--host` flag accepts `ssh://user@host:port` format — Phase 2
- ✅ **CFG-02**: `--path` flag overrides default remote target directory — Phase 2
- ✅ **CFG-03**: `deploy.yaml` stores persistent config (host, path, exclude, healthcheck, etc.) — Phase 2
- ✅ **CFG-04**: Flag > deploy.yaml > defaults precedence — Phase 2
- ✅ **CFG-05**: deploy.yaml schema is forward-compatible with future multi-target — Phase 2
- ✅ **CFG-07**: `~/.docker/cli-plugins/deploy.yaml` global defaults layer — Phase 15

### Pre-Flight Checks (CHECK)
- ✅ **CHECK-01**: Docker installed on remote — Phase 5
- ✅ **CHECK-02**: `docker compose` v2 plugin installed (v1 is hard fail) — Phase 5
- ✅ **CHECK-03**: Docker daemon running (warning only, not a block) — Phase 5
- ✅ **CHECK-04**: SSH user in docker group; attempt `sudo usermod -aG docker` — Phase 5
- ✅ **CHECK-05**: Passwordless sudo check — warning (deploy proceeds via fallback auth) — Phase 5/13
- ✅ **CHECK-06**: Target directory writable; attempt `sudo mkdir -p` + `sudo chown` — Phase 5
- ✅ **CHECK-07**: Warn if SSH user is root — Phase 5

### Post-Deploy Health (HEALTH)
- ✅ **HEALTH-01**: Poll container health via `docker inspect` after compose up — Phase 5/15
- ✅ **HEALTH-02**: Report healthy / unhealthy / unknown (no HEALTHCHECK) per container — Phase 5
- ✅ **HEALTH-03**: Exit non-zero if any container reaches unhealthy state — Phase 5

### Out of Scope (v1)
| Feature | Reason |
|---------|--------|
| Init wizard (Phase 6 / INIT-01..04) | Moved to backlog; sudo fallback in Phases 3/13 covers the practical need |
| Docker remote context setup | Observability via SSH exec is sufficient for v1 |
| Container registry / image push | File-copy based; image building is user's responsibility |
| Blue-green / zero-downtime | Requires reverse proxy in compose.yaml — user-controlled |
| Multi-target (staging/prod) | Architecture accommodates it; MULTI-01/02 are v2 requirements |
| docker-compose v1 compatibility | EOL since June 2023 |
| Terminal demo recording (SC-16-7..9) | Deferred explicitly per Phase 16 D-14 |

---

## 5. Key Decisions Log

| ID | Decision | Phase | Rationale |
|----|----------|-------|-----------|
| D-PLUG | `plugin.Run()` from `github.com/docker/cli` | 1 | Native Docker CLI integration without subprocess overhead |
| D-GO | Go for implementation | 1 | Single binary, no VPS runtime deps, Docker ecosystem norm |
| D-LDFLAG | Version/commit/buildTime via ldflags, not runtime git | 1/13 | No git required on VPS or in production builds |
| D-SSH-TOFU | TOFU prompt for unknown hosts, hard fail on changed key | 2 | Copies secrets; MITM is unacceptable |
| D-NO-INSECURE | `InsecureIgnoreHostKey` forbidden everywhere | 2 | Absolute security requirement |
| D-TIMEOUT | Goroutine + `context.WithTimeout` for SSH dial | 2 | `ClientConfig.Timeout` covers TCP only |
| D-ATOMIC | Stage to `/tmp/docker-deploy-<ts>`, then atomic `mv` | 3 | Prevents partial-deploy state regardless of mid-copy failure |
| D-EXCLUDE-ONLY | Copy-everything-minus-excludes (no include whitelist) | 3 | Simpler mental model; `.env` always copied by default |
| D-ENV-NEVER-EXCLUDED | `.env` never in default excludes | 3 | Core value proposition of the tool |
| D-RESOLVE | Manual `Resolve(FlagOpts, file, defaults)` — no Viper | 2/7 | Viper flag-override ordering bugs; manual is testable |
| D-FLAGOPTS | `FlagOpts` struct replaces positional params | 7 | Readability at 8+ flag call sites |
| D-ROLLUP | Warning rollup without `--verbose` | 7 | Clean output by default; detail on demand |
| D-SKIP-ENV | Backup/restore remote `.env` around atomic swap | 7 | Atomic swap replaces whole dir; simple skip would destroy remote `.env` |
| D-CONTAINER-TOPOLOGY | DinD+SSH custom image with 4 users for integration tests | 8 | Real auth chain coverage; no stubbing SSH or Docker |
| D-SUDO-CREDS | `SudoExec` / `SudoCreds` with `[]byte` + `Zero()` | 13 | Single prompt per deploy; safe credential lifecycle |
| D-NEEDS-SUDO | `test -w path.Dir(remoteBase)` probe before sudo | 13 | User-writable paths never prompt for password |
| D-VALIDATE | `validate` subcommand — local-only config check | 13 | Fast feedback before deploy; no SSH required |
| D-SSH-ALIAS | No `ssh://` prefix = alias lookup in `~/.ssh/config` | 14 | Users already have SSH config; avoids friction |
| D-HEALTHCHECK-FORMAT | `target.healthcheck:` with Docker duration strings | 15 | Mirrors `compose.yaml` HEALTHCHECK syntax; familiar to operators |
| D-KNOWN-FIELDS | `KnownFields(true)` on YAML decoder | 15 | Typos in config keys are hard errors, not silent zero-values |

---

## 6. Tech Debt & Deferred Items

### Known Code Issues (from REVIEW.md in Phase 16)
- **CR-01 — Goroutine leak in `internal/ssh/client.go`**: The goroutine started for the SSH dial timeout is not explicitly cancelled on success; it leaks until the background context expires. Low severity in practice (the goroutine blocks briefly) but a correctness concern.
- **CR-02 — Shell injection risk in `internal/health/poll.go`**: Container names sourced from `docker inspect` output are interpolated into shell commands; a container name with shell metacharacters could cause unexpected behavior. Mitigated by Docker naming constraints but not validated.
- **CR-03 — TOFU race condition**: Concurrent deploys to the same host could both write to `~/.ssh/known_hosts` simultaneously. Rare in practice but technically a race.

### Pending Todos (from STATE.md)
- **Add `ssh_dial_timeout` to `TargetConfig`** — allow timeout to be configured in deploy.yaml (currently only the goroutine default applies)
- **Add `--cleanup` flag** — remove the remote deploy directory (useful for decommissioning)
- **Shell completion install instructions** — cover Intel (`/usr/local/share/zsh/site-functions/`), Apple Silicon (`/opt/homebrew/share/zsh/site-functions/`), and user-local fallback; document bash equivalent
- **Add `make test` with pass/fail summary** — current `make test` output is verbose; a summary target would improve CI readability

### Deferred Features (intentional)
- **Init Wizard (Phase 6 / BACKLOG)**: `--init` flag for first-deploy VPS setup (create deploy user, set up sudoers, write deploy.yaml). Infrastructure exists (sudo machinery from Phase 13); UX design and testing of interactive wizard deferred.
- **SSH `Include` directive support**: `sshconfig.go` skips `Include` lines with a TODO comment. Colima-managed hosts (`Include ~/.colima/ssh_config`) are not reachable via alias until this is implemented.
- **macOS Keychain credential caching**: `SudoCreds` is the hook-in point; the `go-keyring`/`99designs/keyring` integration was deferred from Phase 14 Wave 3.
- **Terminal demo recording** (SC-16-7..9): A `vhs` or `asciinema` demo was planned but deferred per Phase 16 D-14.
- **Re-include mechanism**: `--include` flag or `include:` yaml key for monorepos where some default-excluded files are needed (e.g., `.github/` for health check scripts).
- **`--config <path>` flag** (CFG-06): Arbitrary config file path for monorepo/multi-service layouts. Designed but not implemented.
- **Multi-target** (MULTI-01, MULTI-02): Named deploy targets (`staging`, `prod`) in deploy.yaml. Architecture accommodates it; v2 feature.

---

## 7. Getting Started

### Run the project

```bash
# Build and install the plugin
make install
# or
go install github.com/mniedre/docker-deploy/cmd/docker-deploy@latest

# Verify installation
docker deploy --help
```

### Deploy a project

```bash
# One-time deploy with a flag
docker deploy --host ssh://user@myserver.com

# With a deploy.yaml (persistent config)
cat > deploy.yaml <<EOF
version: 1
target:
  host: ssh://user@myserver.com
  path: /opt/myapp
EOF
docker deploy

# Validate config without connecting
docker deploy validate

# Preview what will change (verbose diff)
docker deploy --verbose
```

### Key directories

| Path | Purpose |
|------|---------|
| `cmd/docker-deploy/` | Main package — cobra root, subcommands, deploy orchestration |
| `internal/config/` | Config resolution (`Resolve()`, `LoadFile()`, `FlagOpts`) |
| `internal/ssh/` | SSH dial, auth chain, knownhosts TOFU |
| `internal/filetransfer/` | SFTP upload, atomic swap, `SudoExec`, `ShouldExclude` filter |
| `internal/compose/` | `RunCompose()` — PTY/pipe output routing, exit code |
| `internal/preflight/` | `RunPreflightChecks()` — CHECK-01 through CHECK-07 |
| `internal/health/` | `PollHealth()` — container health polling after compose up |
| `internal/sshconfig/` | `~/.ssh/config` parser — `LookupHost()`, `LoadSigners()` |
| `internal/completion/` | Hidden `completion` subcommand for static bash/zsh completions |
| `integration/` | Testcontainers-based end-to-end tests against real SSH daemon |

### Tests

```bash
make test               # unit tests (no Docker required)
make test-integration   # integration tests (Docker + ~5 min container startup)
make lint               # golangci-lint with full linter suite
make completions        # regenerate contrib/_docker-deploy and contrib/docker-deploy.bash
```

### Where to look first

- **Entry point:** [`cmd/docker-deploy/main.go`](../../../cmd/docker-deploy/main.go) — `runDeploy()` orchestrates the full deploy sequence
- **Deploy sequence:** Config resolve → SSH dial → pre-flight checks → SFTP upload (atomic) → `docker compose up -d` → health polling
- **Config:** [`internal/config/config.go`](../../../internal/config/config.go) — `Resolve()` function and all config types
- **Security invariant:** `InsecureIgnoreHostKey` must never appear in any code path — grep for it in CI

---

## Stats

- **Timeline:** 2026-05-13 → 2026-06-05 (~23 days)
- **Phases:** 16 / 16 complete (Phase 6 Init Wizard in backlog)
- **Plans executed:** 60 (across 58 plan files + gap-closure plans)
- **Commits:** 688
- **Files changed:** 777 (+152,974 / -4) from initial commit
- **Quick tasks:** 18 (bug fixes, CI fixes, brew fixes)
- **Contributors:** Mike Niedre (mniedre)
- **Languages:** Go, bash (install.sh, CI), YAML (GoReleaser, Actions, deploy.yaml schema)
- **Key dependencies:** `github.com/docker/cli`, `golang.org/x/crypto/ssh`, `github.com/pkg/sftp`, `github.com/spf13/cobra`, `github.com/testcontainers/testcontainers-go`
