# Phase 9: Distribution & Documentation - Discussion Log

> **Audit trail only.** Do not use as input to planning, research, or execution agents.
> Decisions are captured in CONTEXT.md — this log preserves the alternatives considered.

**Date:** 2026-05-22
**Phase:** 9-Distribution & Documentation
**Areas discussed:** Binary signing, Homebrew tap structure, install.sh scope, README content structure

---

## Binary Signing

| Option | Description | Selected |
|--------|-------------|----------|
| cosign keyless | OIDC-based, no key management, runs in GitHub Actions automatically via sigstore | ✓ |
| GPG key | Traditional approach, requires key management and user import | |
| No signing | Skip signing, SHA256 checksums only | |

**User's choice:** cosign keyless

---

| Option | Description | Selected |
|--------|-------------|----------|
| Add darwin/amd64 + darwin/arm64 | Native macOS builds for install.sh and Homebrew | ✓ |
| Linux only | Keep current config, add darwin later | |

**User's choice:** Add darwin/amd64 + darwin/arm64

---

| Option | Description | Selected |
|--------|-------------|----------|
| Verify SHA256 checksum | Download checksums.txt, verify before installing | |
| Verify cosign signature too | Both SHA256 and cosign signature | ✓ |
| No verification | Trust HTTPS transport only | |

**User's choice:** Verify cosign signature too (with SHA256 fallback if cosign absent)

---

| Option | Description | Selected |
|--------|-------------|----------|
| Add id-token: write to release.yml | Required for OIDC cosign in GitHub Actions | ✓ |
| You decide | Leave to planner | |

**User's choice:** Yes — add id-token: write

---

| Option | Description | Selected |
|--------|-------------|----------|
| Sign checksums file only | One signature covers all artifacts — standard practice | ✓ |
| Sign individual archives too | Each .tar.gz gets its own .sig — more granular | |

**User's choice:** Sign checksums file only

---

| Option | Description | Selected |
|--------|-------------|----------|
| No SBOM for v1 | Skip SBOM generation, add later if needed | ✓ |
| Yes, generate SBOM | GoReleaser sboms: via syft | |

**User's choice:** No SBOM for v1

---

## Homebrew Tap Structure

| Option | Description | Selected |
|--------|-------------|----------|
| New tap repo: mniedre/homebrew-docker-deploy | Full control, no review wait, tap URL for install | ✓ |
| PR to homebrew-core | Official repo, no tap URL needed, but popularity threshold required | |

**User's choice:** New tap repo: mniedre/homebrew-docker-deploy

---

| Option | Description | Selected |
|--------|-------------|----------|
| GoReleaser brews: auto-push formula | Zero manual work per release | ✓ |
| Manual formula update | Full control but requires manual commit per release | |

**User's choice:** GoReleaser auto-push

---

| Option | Description | Selected |
|--------|-------------|----------|
| docker-deploy docker-cli-plugin-metadata | Hermetic, no Docker daemon needed | ✓ |
| docker deploy --help | Requires Docker installed in Homebrew CI | |
| Version output only | Simpler but doesn't verify plugin binary contract | |

**User's choice:** docker-deploy docker-cli-plugin-metadata

---

| Option | Description | Selected |
|--------|-------------|----------|
| Dedicated PAT as HOMEBREW_TAP_TOKEN secret | Cross-repo write access, standard approach | ✓ |
| You decide | Leave to planner | |

**User's choice:** Dedicated PAT as HOMEBREW_TAP_TOKEN secret

---

## install.sh Scope

| Option | Description | Selected |
|--------|-------------|----------|
| Latest by default, INSTALL_VERSION pin | Standard pattern for install scripts | ✓ |
| Latest only | Simpler, no pinning | |
| You decide | Leave to planner | |

**User's choice:** Latest by default, pinnable via INSTALL_VERSION env var

---

| Option | Description | Selected |
|--------|-------------|----------|
| Overwrite silently | Print version update message, predictable | ✓ |
| Prompt before overwrite | Breaks non-interactive use | |
| Skip if same version | Idempotent but more script logic | |

**User's choice:** Overwrite silently (print update message)

---

| Option | Description | Selected |
|--------|-------------|----------|
| Fall back to SHA256-only with warning | Keeps script functional if cosign absent | ✓ |
| Fail with cosign install instructions | Strict but blocks users | |
| Skip verification silently | Not recommended — undermines security intent | |

**User's choice:** Fall back to SHA256-only with a warning

---

| Option | Description | Selected |
|--------|-------------|----------|
| Main repo root, raw.githubusercontent.com | Simple, no extra hosting | ✓ |
| GitHub Pages / docs site | Stable URL but requires Pages setup | |

**User's choice:** Main repo root

---

| Option | Description | Selected |
|--------|-------------|----------|
| go install as 4th method (GOBIN workaround) | Covers Go developers, document GOBIN requirement | ✓ |
| Skip go install | 3 methods are enough | |

**User's choice:** Yes — document go install with GOBIN workaround
**Notes:** User explicitly asked if all 4 methods were covered: go install, curl|sh, GitHub release, brew tap

---

## README Content Structure

| Option | Description | Selected |
|--------|-------------|----------|
| Inline prerequisites with anchors | One file, no navigation hop | |
| Separate PREREQUISITES.md | Linked from README | ✓ |
| You decide | Leave to planner | |

**User's choice:** Separate PREREQUISITES.md

---

| Option | Description | Selected |
|--------|-------------|----------|
| Inline troubleshooting | Stays on page, fewer hops when frustrated | |
| Separate TROUBLESHOOTING.md | Linked from README | ✓ |

**User's choice:** Separate TROUBLESHOOTING.md

---

**Comparison table tools** (freeform response):
- Terraform, Ansible, remote Docker context, manual SSH scripts, docker-compose + Watchtower, Portainer, Kamal, full CI/CD tools (GitHub Actions, GitLab CI)

**Comparison dimensions** (freeform):
- docker compose native on remote, secrets/.env handling, quick first deploy, compose-centric design, SSH best practices (no root), complexity/learning curve, remote dependencies, requires git on VPS, use-case fit

---

| Option | Description | Selected |
|--------|-------------|----------|
| Full config reference table in README | Comprehensive, single file | |
| Quick example + DEPLOY_CONFIG.md | Quick example in README, full reference separate | ✓ |

**User's choice:** Quick config example + full config definition in DEPLOY_CONFIG.md

---

| Option | Description | Selected |
|--------|-------------|----------|
| Brief mention in use-case section | One line about --verbose | ✓ |
| Dedicated Verbose/Debug section | Full section | |
| Skip — --help covers it | No mention in README | |

**User's choice:** Brief mention in use-case section

---

| Option | Description | Selected |
|--------|-------------|----------|
| Command + annotated deploy.yaml side by side | Copy-paste ready, both shown | ✓ |
| Command only | Lean, link to config reference | |

**User's choice:** Command + annotated deploy.yaml side by side

---

**Badges** (freeform response, extending options):
Selected: CI status, Latest Release version, Go Report Card, CodeCov/Coveralls, License, Open Issues + TON donation badge at bottom

**TON badge:** `[![TON](https://img.shields.io/badge/Donate-TON-blue)](https://tonviewer.com/UQCB7Y1q3cMl2wxfE1DDHr-VzJ-EeaJIUykx_CUkUdMrbtLG)`

---

**README overall vision** (freeform): "simple deploy tool for devs — explain how to use it (all use cases clearly, Chinese video tutorial style), how to install (all approaches), short but useful manual — do links to related topics in separate MD files (troubleshooting, prerequisite, comparison table, etc.)"

---

| Option | Description | Selected |
|--------|-------------|----------|
| Brief note in prerequisites | "Use WSL2 or Git Bash — install.sh requires bash" | ✓ |
| No mention | Assume macOS/Linux | |
| Full Windows section | WSL2 setup steps | |

**User's choice:** Brief note in prerequisites

---

## Claude's Discretion

None — all areas had explicit user decisions.

## Deferred Ideas

None — discussion stayed within phase scope.
