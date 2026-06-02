---
phase: 1
slug: plugin-scaffolding
status: verified
threats_open: 0
asvs_level: 1
created: 2026-06-02
---

# Phase 1 — Security

> Per-phase security contract: threat register, accepted risks, and audit trail.

---

## Trust Boundaries

| Boundary | Description | Data Crossing |
|----------|-------------|---------------|
| Developer machine → Docker CLI | Docker CLI discovers and invokes the plugin binary by name/location in `~/.docker/cli-plugins/` | Binary executable only; no secrets or user data |
| Build pipeline → binary | GoReleaser cross-compiles binaries on GitHub-managed runners and uploads to GitHub Releases | Compiled binary + `checksums.txt` (SHA256) |
| Downloader → release archive | End users download archives from GitHub Releases | Binary archive; integrity verified via `checksums.txt` |

---

## Threat Register

| Threat ID | Category | Component | Disposition | Mitigation | Status |
|-----------|----------|-----------|-------------|------------|--------|
| T-01-01 | Spoofing | `~/.docker/cli-plugins/docker-deploy` install path | accept | Phase 1 skeleton has no secrets/network access; install is developer-controlled via explicit `make install` — no auto-install mechanism | closed |
| T-01-02 | Tampering | GoReleaser release artifacts | mitigate | `.goreleaser.yaml` generates `checksums.txt` (SHA256, `algorithm: sha256`) attached to every release; users verify before installing | closed |
| T-01-03 | Information Disclosure | `docker-cli-plugin-metadata` JSON response | accept | Response contains only plugin name, version string, and description — no secrets, env data, or user information | closed |
| T-01-04 | Elevation of Privilege | `make install` writes to `~/.docker/cli-plugins/` | accept | Install target writes to user home directory only; no sudo, no system paths; scope limited to current user | closed |
| T-02-01 | Tampering | Release binary download | mitigate | GoReleaser generates `checksums.txt` (SHA256) attached to every GitHub Release (same artifact as T-01-02 control) | closed |
| T-02-02 | Tampering | GitHub Actions workflow files | accept | Workflow files live in the repo; any modification is visible in git history and requires push access to the repository | closed |
| T-02-03 | Elevation of Privilege | `GITHUB_TOKEN` in release workflow | mitigate | `permissions: contents: write, id-token: write` scoped to the `release` job only — `ci` job inherits no elevated permissions. `id-token: write` required for cosign signing (sigstore) | closed |
| T-02-04 | Spoofing | GoReleaser action version pinning | accept | Using `goreleaser/goreleaser-action@v7` major-version tag (plan noted @v6; updated to @v7 — same major-version pinning strategy). Acceptable for Phase 1 skeleton; SHA-pin if required in future hardening phase | closed |

*Status: open · closed*
*Disposition: mitigate (implementation required) · accept (documented risk) · transfer (third-party)*

---

## Accepted Risks Log

| Risk ID | Threat Ref | Rationale | Accepted By | Date |
|---------|------------|-----------|-------------|------|
| AR-01-01 | T-01-01 | Plugin install path is developer-controlled; Phase 1 skeleton has no network access or secrets. Explicit `make install` required — no auto-install attack surface. | plan-time | 2026-05-13 |
| AR-01-02 | T-01-03 | Metadata endpoint returns only name/version/description. No sensitive data exposed. | plan-time | 2026-05-13 |
| AR-01-03 | T-01-04 | `make install` writes to `~/.docker/cli-plugins/` (user home) only. No privilege escalation path. | plan-time | 2026-05-13 |
| AR-01-04 | T-02-02 | Workflow file modifications are visible in git history and require repository push access. | plan-time | 2026-05-13 |
| AR-01-05 | T-02-04 | `goreleaser/goreleaser-action@v7` major-version tag is acceptable for Phase 1; SHA-pin deferred to a future security-hardening phase if warranted. | plan-time | 2026-05-13 |

*Accepted risks do not resurface in future audit runs.*

---

## Security Audit Trail

| Audit Date | Threats Total | Closed | Open | Run By |
|------------|---------------|--------|------|--------|
| 2026-06-02 | 8 | 8 | 0 | gsd-secure-phase (automated) |

**Verification notes:**
- T-01-02 / T-02-01: `checksums.txt` (SHA256) confirmed in `.goreleaser.yaml` lines 28–29
- T-02-03: `permissions` block confirmed scoped to `release` job in `.github/workflows/release.yml`; `id-token: write` is expected (cosign/sigstore signing)
- T-02-04: Action updated from @v6 → @v7 since plan was authored; same major-version pinning strategy applies

---

## Sign-Off

- [x] All threats have a disposition (mitigate / accept / transfer)
- [x] Accepted risks documented in Accepted Risks Log
- [x] `threats_open: 0` confirmed
- [x] `status: verified` set in frontmatter

**Approval:** verified 2026-06-02
