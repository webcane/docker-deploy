# Phase 9: Distribution & Documentation - Context

**Gathered:** 2026-05-22
**Status:** Ready for planning

<domain>
## Phase Boundary

Ship docker-deploy as a fully installable, well-documented OSS tool. This phase delivers:
- **Distribution:** 4 install paths (Homebrew tap, curl|sh script, GitHub Releases binary, go install) backed by GoReleaser producing signed darwin+linux × amd64+arm64 binaries
- **Documentation:** README.md as the concise user hub + 4 supporting files (PREREQUISITES.md, DEPLOY_CONFIG.md, TROUBLESHOOTING.md, COMPARISON.md)

This phase does NOT add new plugin features — only distribution infrastructure and user-facing documentation.

</domain>

<decisions>
## Implementation Decisions

### GoReleaser & Binary Signing

- **D-01:** Add `darwin/amd64` and `darwin/arm64` to GoReleaser `goos`/`goarch` matrix (currently only `linux`)
- **D-02:** Use **cosign keyless** signing (OIDC via sigstore) — no GPG key, no key management
- **D-03:** Sign **checksums file only** (not individual archives) — standard pattern: one signature covers all artifacts
- **D-04:** Add `id-token: write` permission to `.github/workflows/release.yml` (required for OIDC cosign in GitHub Actions)
- **D-05:** No SBOM for v1 — skip SBOM generation, can be added later
- **D-06:** GoReleaser `signs:` block signs `checksums.txt` using cosign keyless

### Homebrew Tap

- **D-07:** Create new tap repo: **`webcane/homebrew-docker-deploy`** (not homebrew-core PR)
- **D-08:** GoReleaser `brews:` block auto-pushes formula to tap repo on every release — zero manual work per release
- **D-09:** Formula `test` block uses: `docker-deploy docker-cli-plugin-metadata` (no Docker daemon needed, hermetic, always passes in Homebrew CI)
- **D-10:** Use a dedicated PAT stored as **`HOMEBREW_TAP_TOKEN`** Actions secret (GITHUB_TOKEN has no cross-repo write access)

### install.sh

- **D-11:** Hosted at main repo root; invoked via `curl https://raw.githubusercontent.com/webcane/docker-deploy/main/install.sh | sh`
- **D-12:** Installs **latest** by default; supports version pinning via `INSTALL_VERSION=vX.Y.Z` env var
- **D-13:** **Silent overwrite** on upgrade — replace existing binary and print "Updated docker-deploy vOLD → vNEW"
- **D-14:** Verification: attempt cosign signature verification; **fall back to SHA256-only with a warning** if cosign is not installed on the user's machine (do NOT fail hard)
- **D-15:** Always verify SHA256 checksum against `checksums.txt` even when cosign is unavailable

### Install Methods (4 total)

- **D-16:** Four documented install methods:
  1. **Homebrew:** `brew tap webcane/docker-deploy && brew install docker-deploy`
  2. **curl|sh:** `curl https://raw.githubusercontent.com/webcane/docker-deploy/main/install.sh | sh`
  3. **GitHub Releases:** manual binary download from releases page, chmod +x, move to `~/.docker/cli-plugins/`
  4. **go install:** `GOBIN=~/.docker/cli-plugins go install github.com/webcane/docker-deploy/cmd/docker-deploy@latest` (note: must set GOBIN explicitly due to plugin placement requirement)

### README Structure

- **D-17:** README is the **concise hub** — explains what it is, how to install, and how to use it. No deep content inline.
- **D-18:** All deep content lives in separate linked files: PREREQUISITES.md, DEPLOY_CONFIG.md, TROUBLESHOOTING.md, COMPARISON.md
- **D-19:** README section order (top → bottom):
  1. Title + Badges
  2. Tagline (one sentence)
  3. What is docker-deploy? (3 sentences — value prop, no git on VPS, compose-native)
  4. Installation (all 4 methods with copy-paste commands)
  5. Usage (Quick Start + 3 scenarios)
  6. Learn More (links to the 4 supporting files)
  7. Feedback (GitHub Issues link + welcome message)
  8. TON donation badge (very bottom)

### README Badges

- **D-20:** Badges in header: CI status, Latest Release version, Go Report Card, CodeCov/Coveralls, License, Open Issues
- **D-21:** TON donation badge at the very bottom of README (not in header badges row):
  ```
  [![TON](https://img.shields.io/badge/Donate-TON-blue)](https://tonviewer.com/UQCB7Y1q3cMl2wxfE1DDHr-VzJ-EeaJIUykx_CUkUdMrbtLG)
  ```

### README Usage Section

- **D-22:** Three use-case scenarios, each with a full **command + annotated deploy.yaml** side by side (copy-paste ready):
  1. Recommended: sshuser (non-root) setup
  2. Flags-only (no deploy.yaml, minimal configuration)
  3. deploy.yaml config-driven (persistent config for repeat deploys)
- **D-23:** Brief `--verbose` mention inline in usage (one line: "Use --verbose to see each file transferred and each SSH command")
- **D-24:** Link to DEPLOY_CONFIG.md for full configuration reference (not duplicated in README)

### PREREQUISITES.md

- **D-25:** Covers: SSH key setup, adding passwordless sudo to sshuser on the remote VPS
- **D-26:** Brief note for Windows users: "Use WSL2 or Git Bash — install.sh requires bash"

### COMPARISON.md

- **D-27:** Compare docker-deploy against 8 tools: Terraform, Ansible, remote Docker context, manual SSH scripts, docker-compose + Watchtower, Portainer, Kamal, full CI/CD tools (GitHub Actions / GitLab CI)
- **D-28:** Comparison dimensions (9 total): docker compose native on remote, secrets/.env handling, time to first deploy (quick path), compose-centric design, SSH best practices (no root), complexity/learning curve, remote dependencies, requires git on VPS, use-case fit

### TROUBLESHOOTING.md

- **D-29:** Cover the 5 most common failure scenarios with actionable fixes: SSH auth failure, unknown host (knownhosts prompt), target dir not writable, Docker not found on remote, compose v1 detected (EOL)

### DEPLOY_CONFIG.md

- **D-30:** Full deploy.yaml config reference: all supported fields with types, defaults, and description. Linked from README.

</decisions>

<canonical_refs>
## Canonical References

**Downstream agents MUST read these before planning or implementing.**

### Project Foundation
- `.planning/ROADMAP.md` §Phase 9 — Success criteria (12 items) for distribution and documentation
- `.planning/REQUIREMENTS.md` — v1 requirements for context on what the tool does (for README accuracy)
- `.planning/PROJECT.md` — Core value proposition, key decisions, constraints

### Existing Distribution Infrastructure
- `.goreleaser.yaml` — Current GoReleaser config (linux-only; needs darwin + cosign + brews: block added)
- `.github/workflows/release.yml` — Current release workflow (needs `id-token: write` added)
- `.github/workflows/ci.yml` — CI workflow (for CI badge URL)

### Codebase (for accurate documentation)
- `cmd/docker-deploy/` — Plugin entry point; for understanding `--help` output and flag names
- `internal/config/` — deploy.yaml schema; all fields must be documented in DEPLOY_CONFIG.md
- `README.md` — Current placeholder (2 lines); will be replaced

No external specs — all decisions captured above.

</canonical_refs>

<code_context>
## Existing Code Insights

### Reusable Assets
- `cmd/docker-deploy/main.go` — Cobra command with all flags defined; planner should read this to document all flags accurately in README and DEPLOY_CONFIG.md
- `internal/config/config.go` — Config struct with all deploy.yaml fields and defaults; DEPLOY_CONFIG.md must be derived from this

### Established Patterns
- GoReleaser already wired to tag-triggered release workflow — adding darwin builds and cosign signing extends existing config, not a rewrite
- CI already configured — CI badge URL can be derived from existing `.github/workflows/ci.yml` name

### Integration Points
- `install.sh` places binary at `~/.docker/cli-plugins/docker-deploy` — matches the plugin convention in CLAUDE.md and Phase 1
- `GOBIN=~/.docker/cli-plugins` workaround for `go install` — required because Docker CLI plugin path differs from standard GOBIN

</code_context>

<specifics>
## Specific Ideas

- **"Chinese tutorial video" style:** Use-case section should be step-by-step clear with copy-paste examples — the user explicitly wants this style for the usage documentation
- **TON donation badge exact URL:** `https://tonviewer.com/UQCB7Y1q3cMl2wxfE1DDHr-VzJ-EeaJIUykx_CUkUdMrbtLG` — badge at bottom of README
- **cosign fallback behavior:** `install.sh` must print: "cosign not found — skipping signature verification, checking SHA256 only" and continue (not fail)
- **Tap install command:** `brew tap webcane/docker-deploy && brew install docker-deploy`
- **go install note:** Must explain GOBIN requirement — unlike standard Go tools, docker-deploy must land in `~/.docker/cli-plugins/` not `$GOPATH/bin`

</specifics>

<deferred>
## Deferred Ideas

None — discussion stayed within phase scope.

</deferred>

---

*Phase: 9-Distribution & Documentation*
*Context gathered: 2026-05-22*
