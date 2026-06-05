---
phase: 9
phase_name: "documentation"
project: "docker-deploy"
generated: "2026-06-05"
counts:
  decisions: 7
  lessons: 5
  patterns: 4
  surprises: 3
missing_artifacts:
  - "09-HUMAN-UAT.md — present but tests deferred (SC-09-3 and SC-09-4 require a live tagged release)"
---

# Phase 9 Learnings: Documentation

## Decisions

### cosign keyless signing via OIDC, no GPG key
Used cosign's OIDC-based keyless signing (sigstore Fulcio/Rekor) rather than GPG. `COSIGN_EXPERIMENTAL=1` was originally added but removed after discovering it is deprecated in cosign v2.

**Rationale:** Keyless signing eliminates GPG key management entirely. Ephemeral signing keys are tied to the GitHub Actions OIDC token — no long-lived secrets to rotate. The `id-token: write` permission in the release job workflow is sufficient.
**Source:** 09-01-SUMMARY.md

---

### Sign checksums.txt only, not individual archives
The `signs` block in `.goreleaser.yaml` targets `artifacts: checksum`, producing a single `.pem` + `.sig` pair for `checksums.txt`.

**Rationale:** Signing the checksum file rather than each archive is the standard GoReleaser pattern. One signature covers all artifacts via SHA256 integrity, reducing the number of signature files attached to each release.
**Source:** 09-01-SUMMARY.md

---

### Homebrew formula test uses docker-cli-plugin-metadata
The `brews[0].test` block runs `system "#{bin}/docker-deploy docker-cli-plugin-metadata"` rather than a connectivity test.

**Rationale:** Homebrew CI does not have Docker installed. The `docker-cli-plugin-metadata` subcommand only prints JSON metadata — it requires no Docker daemon and no VPS. Hermetic and always passes in Homebrew's sandboxed test environment.
**Source:** 09-01-SUMMARY.md

---

### Homebrew caveats explain the cli-plugins symlink requirement
The formula includes a `caveats` block instructing users to run `mkdir -p ~/.docker/cli-plugins && ln -sf $(brew --prefix)/bin/docker-deploy ~/.docker/cli-plugins/docker-deploy`.

**Rationale:** Homebrew installs binaries to its prefix (`/opt/homebrew/bin` or `/usr/local/bin`). Docker CLI plugin discovery only looks in `~/.docker/cli-plugins/`. Without the symlink, `docker deploy` would not be recognized as a plugin even though the binary is on PATH.
**Source:** 09-01-SUMMARY.md

---

### HOMEBREW_TAP_TOKEN as encrypted Actions secret, not inlined
The GoReleaser `brews.repository.token` field uses `{{ .Env.HOMEBREW_TAP_TOKEN }}`. The PAT is stored as a GitHub Actions secret, never hardcoded.

**Rationale:** `GITHUB_TOKEN` is scoped to the current repository and cannot push to a separate `webcane/homebrew-docker-deploy` tap repo. A classic PAT with `repo` scope is required for cross-repo write access.
**Source:** 09-01-SUMMARY.md

---

### README is a concise hub — deep content lives in linked support files
README.md covers the value proposition, all install methods, and three usage scenarios but does not inline deep content. All detailed references (SSH setup, config fields, troubleshooting, comparisons) link out to four separate files.

**Rationale:** A README that tries to cover everything becomes unmaintainable and overwhelming. Four dedicated files (PREREQUISITES.md, DEPLOY_CONFIG.md, TROUBLESHOOTING.md, COMPARISON.md) each serve a specific user need without bloating the entry point.
**Source:** 09-03-SUMMARY.md

---

### DEPLOY_CONFIG.md field defaults derived directly from config.go source
All field defaults in DEPLOY_CONFIG.md (health_timeout=60, health_interval=5, etc.) were verified against `internal/config/config.go Resolve()` rather than written from memory.

**Rationale:** Documentation drift between defaults in docs and defaults in code is a common failure mode. Cross-referencing the source directly at write time prevents this for the initial publication.
**Source:** 09-04-SUMMARY.md

---

## Lessons

### SHA256 verification in install.sh had a silent bypass on Linux
The initial implementation piped directly to `sha256sum -c` without isolating the matching checksum line first. On Linux, `sha256sum -c` attempts to verify all entries in the file and fails if any entry's filename is not present — but the exit code behavior could silently succeed in some shells.

**Context:** CR-01 code review finding. Fixed by extracting the matching checksum line to a temporary `archive.sha256` file and verifying only that one entry.
**Source:** 09-VERIFICATION.md

---

### cosign verify-blob requires --certificate-identity and --certificate-oidc-issuer flags in v2
The initial `cosign verify-blob` call omitted the required identity flags. cosign v2 enforces identity verification by default and rejects bare `verify-blob` without these flags.

**Context:** CR-02 code review finding. Fixed by adding `--certificate-identity-regexp` and `--certificate-oidc-issuer` to the cosign invocation in install.sh.
**Source:** 09-VERIFICATION.md

---

### INSTALL_VERSION version pinning requires the env var before `sh`, not before `curl`
The initial install.sh usage example showed `INSTALL_VERSION=v1.0.0 curl ... | sh`. In a curl-pipe-sh invocation, env vars set before `curl` are not visible to the `sh` subprocess that runs the script.

**Context:** CR-03 code review finding. Fixed by placing the variable before `sh`: `curl ... | INSTALL_VERSION=v1.0.0 sh`. Both install.sh comment and README.md usage example were corrected.
**Source:** 09-VERIFICATION.md

---

### COSIGN_EXPERIMENTAL is deprecated in cosign v2
The initial `.goreleaser.yaml` signs block included `env: [COSIGN_EXPERIMENTAL=1]`. This env var was required for cosign v1 keyless signing but is deprecated (and in some versions, causes warnings) in cosign v2.

**Context:** WR-02 code review finding. The env var was removed from `.goreleaser.yaml`. Keyless signing works without it in cosign v2.
**Source:** 09-VERIFICATION.md

---

### Codecov badge shows "unknown" state until a CI run uploads coverage data
The README includes a Codecov badge but on initial publication the badge shows "unknown" because no coverage report has been uploaded to Codecov yet.

**Context:** Reported in 09-HUMAN-UAT.md as a minor issue. The badge will display correctly after the first CI run that uploads coverage. Not a blocker — 5 of 6 badges were correct immediately.
**Source:** 09-UAT.md

---

## Patterns

### Concise hub README pattern: badges, tagline, why, install, usage, links, feedback
README.md follows an 8-section structure: (1) title + badges, (2) tagline, (3) what is it, (4) installation (4 methods), (5) usage (3 scenarios), (6) learn more links, (7) feedback, (8) donation badge.

**When to use:** Any CLI tool with multiple install paths and usage scenarios. The pattern separates the "what and why" (README) from the "how in depth" (linked support files). Keeps README under ~200 lines while remaining comprehensive.
**Source:** 09-03-SUMMARY.md

---

### POSIX sh installer with mktemp -d trap for cleanup
Install scripts use `TMPDIR=$(mktemp -d)` and `trap 'rm -rf "$TMPDIR"' EXIT` to guarantee temp directory cleanup on any exit path.

**When to use:** Any POSIX sh script that downloads files to a temporary location. The trap fires on normal exit, error exit, and signal exit, preventing leftover files regardless of how the script terminates.
**Source:** 09-02-SUMMARY.md

---

### go install option requires GOBIN=~/.docker/cli-plugins
When documenting `go install` as an install method for a Docker CLI plugin, GOBIN must be set to `~/.docker/cli-plugins`. Standard `$GOPATH/bin` is not scanned by Docker CLI plugin discovery.

**When to use:** Any Docker CLI plugin that offers `go install` as an installation method. Include an explicit note explaining why GOBIN must differ from the default.
**Source:** 09-03-SUMMARY.md

---

### Field reference documentation cross-checked against source code defaults
Documentation of configuration defaults (health_timeout=60, health_interval=5) is verified against the `Resolve()` function in `internal/config/config.go` rather than written from assumption.

**When to use:** Any phase that produces user-facing documentation of configuration fields. Verify each default against the actual code at write time; note the source file in the document or a code comment.
**Source:** 09-04-SUMMARY.md

---

## Surprises

### macOS Gatekeeper quarantine requires xattr step after manual binary install
Downloaded binaries on macOS are quarantined by Gatekeeper. The `docker deploy --help` command silently fails until `xattr -d com.apple.quarantine ~/.docker/cli-plugins/docker-deploy` is run.

**Impact:** TROUBLESHOOTING.md gained a macOS Gatekeeper section during UAT. README manual install option 3 was updated with the xattr step. Discovered during human UAT (09-UAT.md).
**Source:** 09-UAT.md

---

### Tap repository requires a human setup step that blocks automated testing
GoReleaser cannot push the Homebrew formula until `webcane/homebrew-docker-deploy` exists as a public repo and `HOMEBREW_TAP_TOKEN` is added as an Actions secret. These steps require a human and cannot be automated or verified programmatically.

**Impact:** SC-09-3 and SC-09-4 were explicitly deferred to "pending" in HUMAN-UAT.md, requiring a tagged release to complete verification. The VERIFICATION report classified the phase as passing with 10/12 truths verified programmatically.
**Source:** 09-VERIFICATION.md

---

### goreleaser archive name template omits version by default
The `.goreleaser.yaml` archive name template `{{ .ProjectName }}_{{ .Os }}_{{ .Arch }}` does not include the version. Install script functionality is unaffected (version appears in the download URL path), but downloaded archives lack the version in their filename.

**Impact:** Identified as INFO/optional in the code review. Not fixed in this phase. Users downloading manually see `docker-deploy_linux_amd64.tar.gz` rather than `docker-deploy_v1.0.0_linux_amd64.tar.gz`.
**Source:** 09-VERIFICATION.md
