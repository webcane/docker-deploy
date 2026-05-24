---
phase: 11
slug: ci-tooling-polish
status: verified
threats_open: 0
asvs_level: 1
created: 2026-05-24
---

# Phase 11 — Security

> Per-phase security contract: threat register, accepted risks, and audit trail.

---

## Trust Boundaries

| Boundary | Description | Data Crossing |
|----------|-------------|---------------|
| CI runner → codecov.io | Coverage report uploaded from GitHub Actions to third-party service | coverage.out (non-sensitive, public repo) |
| Dependabot → workflow files | Automated PRs that bump action versions | Workflow YAML changes |
| brew install → ~/.docker/cli-plugins/ | post_install writes symlink to user's home directory | Symlink to Homebrew-managed binary |
| brew uninstall → ~/.docker/cli-plugins/ | def uninstall removes symlink from user's home directory | Symlink removal |
| CI runner → go install golangci-lint | Fetching golangci-lint binary at CI time | Linter binary (version-pinned) |

---

## Threat Register

| Threat ID | Category | Component | Disposition | Mitigation | Status |
|-----------|----------|-----------|-------------|------------|--------|
| T-11-01-01 | Information Disclosure | coverage.out upload to Codecov | accept | Public repo — coverage data is not sensitive; tokenless upload intentional (D-05) | closed |
| T-11-01-02 | Denial of Service | Codecov upload step | mitigate | `fail_ci_if_error: false` in codecov-action config — CI not blocked by Codecov outages | closed |
| T-11-02-01 | Tampering | Dependabot auto-PRs bumping action versions | accept | PRs require human review before merge; major-version pinning (@vN) limits blast radius | closed |
| T-11-02-02 | Elevation of Privilege | Removing FORCE_JAVASCRIPT_ACTIONS_TO_NODE24 | accept | Env var was a workaround; removal reduces noise; actions already on current versions | closed |
| T-11-03-01 | Tampering | post_install writes symlink to ~/.docker/cli-plugins/ | accept | Homebrew install is an intentional privileged operation; user consents by running brew install | closed |
| T-11-03-02 | Tampering | def uninstall removes symlink from ~/.docker/cli-plugins/ | accept | brew uninstall is intentional; File.exist? guard prevents error on missing file; only removes specific symlink | closed |
| T-11-03-03 | Elevation of Privilege | post_install runs as the user, not root | accept | Homebrew runs as current user; ~/.docker/cli-plugins/ is user-owned — no privilege escalation | closed |
| T-11-04-01 | Tampering | go install golangci-lint in CI | mitigate | Pinned to specific version (@v2.12.2 via golangci-lint/v2) — prevents new release from breaking CI for unchanged code | closed |
| T-11-04-02 | Information Disclosure | errcheck catching unhandled errors | mitigate | errcheck enabled as CI gate in .golangci.yml — prevents silent failures in SSH/SFTP operations | closed |

*Status: open · closed*
*Disposition: mitigate (implementation required) · accept (documented risk) · transfer (third-party)*

---

## Accepted Risks Log

| Risk ID | Threat Ref | Rationale | Accepted By | Date |
|---------|------------|-----------|-------------|------|
| AR-11-01 | T-11-01-01 | Public repo — coverage data not sensitive; tokenless Codecov upload is intentional design decision D-05 | plan-time | 2026-05-24 |
| AR-11-02 | T-11-02-01 | Dependabot PRs require human review; major-version pinning limits blast radius | plan-time | 2026-05-24 |
| AR-11-03 | T-11-02-02 | FORCE_JAVASCRIPT_ACTIONS_TO_NODE24 removal is intentional cleanup; actions already on current versions | plan-time | 2026-05-24 |
| AR-11-04 | T-11-03-01 | brew install is a user-initiated, consented action; symlink into user-owned dir is expected | plan-time | 2026-05-24 |
| AR-11-05 | T-11-03-02 | brew uninstall is a user-initiated, consented action; File.exist? guard prevents accidental deletion | plan-time | 2026-05-24 |
| AR-11-06 | T-11-03-03 | post_install runs as the invoking user (not root); ~/.docker/cli-plugins/ is user-owned — no escalation | plan-time | 2026-05-24 |

---

## Security Audit Trail

| Audit Date | Threats Total | Closed | Open | Run By |
|------------|---------------|--------|------|--------|
| 2026-05-24 | 9 | 9 | 0 | gsd-secure-phase (automated) |

---

## Sign-Off

- [x] All threats have a disposition (mitigate / accept / transfer)
- [x] Accepted risks documented in Accepted Risks Log
- [x] `threats_open: 0` confirmed
- [x] `status: verified` set in frontmatter

**Approval:** verified 2026-05-24
