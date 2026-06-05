---
phase: 09-documentation
plan: 01
subsystem: infra
tags: [goreleaser, cosign, homebrew, github-actions, darwin, release]

# Dependency graph
requires:
  - phase: 08-init-wizard
    provides: complete plugin binary built by existing GoReleaser linux-only config
provides:
  - GoReleaser config producing 4 platform binaries (linux+darwin x amd64+arm64)
  - cosign keyless signing of checksums.txt (no GPG key required)
  - Homebrew formula auto-pushed to webcane/homebrew-docker-deploy on each tag
  - release.yml with OIDC permission and HOMEBREW_TAP_TOKEN env wired
affects: [09-02, 09-03, 09-04]

# Tech tracking
tech-stack:
  added: [cosign keyless signing (sigstore OIDC), GoReleaser brews block, Homebrew tap distribution]
  patterns: [sign-checksum-not-artifacts pattern, cross-repo PAT via encrypted Actions secret]

key-files:
  created: []
  modified:
    - .goreleaser.yaml
    - .github/workflows/release.yml

key-decisions:
  - "cosign keyless (OIDC) — no GPG key, no key management; COSIGN_EXPERIMENTAL=1 enables Fulcio/Rekor transparency"
  - "Sign checksums.txt only (not individual archives) — standard pattern: one signature covers all artifacts"
  - "Homebrew formula test uses docker-cli-plugin-metadata (hermetic, no Docker daemon needed in Homebrew CI)"
  - "Caveats instruct user to symlink binary into ~/.docker/cli-plugins/ — Homebrew cannot write there directly"
  - "HOMEBREW_TAP_TOKEN stored as encrypted Actions secret — GITHUB_TOKEN has no cross-repo write access"

patterns-established:
  - "GoReleaser sign-blob pattern: signs: artifacts: checksum produces a single .pem + .sig pair for checksums.txt"
  - "Homebrew caveats explain Docker plugin path requirement to prevent silent install failures"

requirements-completed:
  - SC-09-1
  - SC-09-3

# Metrics
duration: 2min
completed: 2026-05-22
---

# Phase 9 Plan 01: GoReleaser Distribution Pipeline Summary

**GoReleaser extended to produce 4 platform binaries (linux+darwin x amd64+arm64) with cosign keyless signing and auto-push of Homebrew formula to webcane/homebrew-docker-deploy tap — awaiting human setup of tap repo and HOMEBREW_TAP_TOKEN secret (Task 3 checkpoint)**

## Performance

- **Duration:** ~2 min
- **Started:** 2026-05-22T16:55:37Z
- **Completed:** 2026-05-22T16:57:40Z (Tasks 1-2; Task 3 is a human-action checkpoint)
- **Tasks:** 2 of 3 auto tasks complete; 1 human-action checkpoint pending
- **Files modified:** 2

## Accomplishments

- Added darwin/amd64 and darwin/arm64 to GoReleaser goos matrix — 4 platform binaries per release
- Added cosign keyless signs block (COSIGN_EXPERIMENTAL=1) signing checksums.txt via sigstore OIDC — no GPG key required
- Added brews block pushing Homebrew formula to webcane/homebrew-docker-deploy with hermetic test and symlink caveats
- Added id-token: write permission to release.yml release job for OIDC cosign signing
- Added HOMEBREW_TAP_TOKEN env to goreleaser-action step in release.yml

## Task Commits

Each auto task was committed atomically:

1. **Task 1: Expand GoReleaser config — darwin builds, cosign signing, brews block** - `074d5ec` (feat)
2. **Task 2: Update release.yml — add OIDC permission and tap token env** - `295df07` (feat)
3. **Task 3: Human — create tap repo and HOMEBREW_TAP_TOKEN secret** - PENDING (checkpoint:human-action)

## Files Created/Modified

- `.goreleaser.yaml` — Added darwin to goos, signs block (cosign keyless), brews block (webcane/homebrew-docker-deploy)
- `.github/workflows/release.yml` — Added id-token: write permission, HOMEBREW_TAP_TOKEN env in goreleaser step

## Decisions Made

- cosign keyless signing chosen (OIDC via sigstore Fulcio/Rekor) — no GPG key management, ephemeral keys tied to workflow OIDC token
- Sign checksums.txt only (not individual archives) — standard release pattern, one signature covers all artifacts
- Homebrew formula test uses `docker-cli-plugin-metadata` — hermetic test, no Docker daemon required in Homebrew CI
- Formula caveats explain the ~/.docker/cli-plugins/ symlink requirement — Homebrew installs to prefix/bin, Docker CLI reads cli-plugins
- PAT stored as encrypted Actions secret (HOMEBREW_TAP_TOKEN) — GITHUB_TOKEN scoped to current repo only, cannot push to separate tap repo

## Deviations from Plan

None — plan executed exactly as written.

## Issues Encountered

None.

## User Setup Required

**Task 3 is a blocking human-action checkpoint.** The following manual steps are required before a tag push will succeed:

**Step 1 — Create the tap repository:**
Go to https://github.com/new
- Owner: webcane
- Repository name: homebrew-docker-deploy
- Visibility: Public
- Initialize with a README (required for GoReleaser to push to it)
Click "Create repository"

**Step 2 — Create a Personal Access Token with repo scope:**
Go to GitHub Settings → Developer settings → Personal access tokens → Tokens (classic)
- Note: "GoReleaser Homebrew tap push"
- Expiration: 1 year (or no expiration)
- Scope: repo (full repo access)
Click "Generate token" and copy the token value immediately.

**Step 3 — Add the PAT as HOMEBREW_TAP_TOKEN secret:**
Go to https://github.com/webcane/docker-deploy/settings/secrets/actions
Click "New repository secret"
- Name: HOMEBREW_TAP_TOKEN
- Secret: paste the token from Step 2
Click "Add secret"

**Verify:**
- https://github.com/webcane/homebrew-docker-deploy exists and is public
- https://github.com/webcane/docker-deploy/settings/secrets/actions shows HOMEBREW_TAP_TOKEN listed

## Next Phase Readiness

- .goreleaser.yaml and release.yml are fully configured — no further code changes needed for Homebrew distribution
- Remaining blockers: tap repo must exist and HOMEBREW_TAP_TOKEN secret must be set before first tag push
- After human checkpoint completes, Plans 09-02 through 09-05 can proceed independently (documentation plans do not depend on the tap being live)

## Known Stubs

None — no stub values, placeholder text, or unwired data sources in the modified files.

## Threat Flags

No new security surface beyond what is documented in the plan's threat model. HOMEBREW_TAP_TOKEN is referenced only as `${{ secrets.HOMEBREW_TAP_TOKEN }}` — never inlined. COSIGN_EXPERIMENTAL=1 enables keyless signing; no private key material is stored.

---

## Self-Check

Verifying claims before finalizing:

- [x] `.goreleaser.yaml` modified — file exists with darwin, cosign, HOMEBREW_TAP_TOKEN, docker-cli-plugin-metadata
- [x] `.github/workflows/release.yml` modified — file exists with id-token: write and HOMEBREW_TAP_TOKEN
- [x] Task 1 commit `074d5ec` — goreleaser config expansion
- [x] Task 2 commit `295df07` — release.yml update

## Self-Check: PASSED

---
*Phase: 09-documentation*
*Completed (partial): 2026-05-22 — awaiting Task 3 human-action checkpoint*
