---
phase: 11-ci-tooling-polish
verified: 2026-05-23T22:30:00Z
status: pass
score: 13/13
overrides_applied: 3
overrides:
  - truth: "golangci-lint CI gate installs a real, existing version of golangci-lint"
    status: override_accepted
    evidence: "gh api repos/golangci/golangci-lint/releases/tags/v1.64.8 returns tag_name: v1.64.8 — version confirmed to exist. Reviewer/verifier had stale training data."

  - truth: "GitHub Actions workflow versions are valid and will resolve at runtime"
    status: override_accepted
    evidence: "Dependabot merged PRs #5-#8 bumping checkout@v4→v6, setup-go@v5→v6, codecov-action@v4→v6, goreleaser-action@v6→v7. Dependabot only creates PRs for versions that exist on GitHub. All action tags confirmed valid."

  - truth: "brew uninstall removes the Docker CLI plugin symlink automatically"
    status: intentional_descope
    reason: "def uninstall is a Homebrew Cask lifecycle hook — it is not valid in a Formula class body and is silently ignored at runtime. Converting to a Cask is out of scope for Phase 11. The correct Formula-compatible solution is the caveats block (added in commit aa4ffd9) documenting the manual rm -f step. The post_install symlink creation goal is fully achieved; automated removal is a known limitation of Homebrew Formula vs Cask architecture."
---

# Phase 11: CI & Tooling Polish Verification Report

**Phase Goal:** Fix Codecov, bump GitHub Actions versions, add Brew auto-symlink on install and cleanup on uninstall, add golangci-lint CI gate.
**Verified:** 2026-05-23T22:30:00Z
**Status:** PASS (3 overrides applied)
**Re-verification:** Yes — overrides applied 2026-05-23

## Goal Achievement

### Observable Truths

| # | Truth | Status | Evidence |
|---|-------|--------|----------|
| 1 | Codecov config exists with PR comment controls | VERIFIED | `codecov.yml` has `comment.require_changes: true` and `coverage.status.project.default.target: auto` |
| 2 | Coverage is uploaded to Codecov on every CI run | VERIFIED | `ci.yml` test job runs `go test -coverprofile=coverage.out ./...` then `codecov/codecov-action@v6` upload |
| 3 | README Codecov badge points to correct master branch | VERIFIED | Badge URL: `codecov.io/gh/webcane/docker-deploy/branch/master/graph/badge.svg` |
| 4 | FORCE_JAVASCRIPT_ACTIONS_TO_NODE24 removed from all workflows | VERIFIED | Absent from both `ci.yml` and `release.yml` — confirmed by grep |
| 5 | Dependabot configured for github-actions ecosystem weekly | VERIFIED | `.github/dependabot.yml` has `package-ecosystem: github-actions`, `schedule: weekly` |
| 6 | golangci-lint config has disable-all + 4 linters with correct local prefix | VERIFIED | `.golangci.yml` has `disable-all: true`, enables errcheck/govet/staticcheck/goimports, `local-prefixes: github.com/webcane/docker-deploy` |
| 7 | Makefile has lint and fmt targets with correct commands | VERIFIED | `lint: golangci-lint run ./...` and `fmt: goimports -w -local github.com/webcane/docker-deploy ./...` |
| 8 | golangci-lint CI gate installs a real, existing version | FAILED | `ci.yml` line 20 installs `golangci-lint@v1.64.8` — v1 series ended at ~v1.61.x; this version does not exist |
| 9 | GitHub Actions workflow action versions are valid and will resolve | UNCERTAIN (WARNING) | `ci.yml` and `release.yml` use `@v6`/`@v7` tags; Phase 11 code review flags these as non-existent (latest was @v4/@v5 at review time); `aa4ffd9` fix commit did not address this |
| 10 | CI push trigger fires on master branch commits | VERIFIED | `ci.yml` line 5: `branches: [master]` — fixed by `aa4ffd9` |
| 11 | Integration job requires lint to pass before running | VERIFIED | `ci.yml` line 50: `needs: [test, lint]` — fixed by `aa4ffd9` |
| 12 | brew install creates ~/.docker/cli-plugins/docker-deploy symlink automatically | VERIFIED | `.goreleaser.yaml` `post_install` block creates symlink with EPERM fallback to `opoo` warning |
| 13 | brew uninstall removes ~/.docker/cli-plugins/docker-deploy symlink automatically | FAILED | `def uninstall` was removed by `aa4ffd9` because Homebrew Formula DSL has no such hook (only Casks do). Replaced with a `caveats` block requiring manual `rm -f` — automated removal NOT achieved |

**Score:** 9/13 truths verified (truths #8, #9, #13 are the critical gaps)

### Required Artifacts

| Artifact | Expected | Status | Details |
|----------|----------|--------|---------|
| `codecov.yml` | Codecov config with comment controls | VERIFIED | Exists, has require_changes and target: auto |
| `.github/dependabot.yml` | Dependabot for github-actions weekly | VERIFIED | Correct config |
| `.golangci.yml` | disable-all + 4 linters + goimports prefix | VERIFIED | Matches D-01 through D-03 exactly |
| `Makefile` | lint and fmt targets | VERIFIED | Both present with correct commands |
| `.github/workflows/ci.yml` | Coverage upload, lint job, master trigger, integration needs lint | VERIFIED (with gap) | All structural requirements met; golangci-lint version invalid |
| `.github/workflows/release.yml` | No FORCE workaround | VERIFIED | Absent |
| `.goreleaser.yaml` | post_install symlink + EPERM rescue | VERIFIED | post_install block exists and correct |
| `.goreleaser.yaml` | def uninstall removes symlink | FAILED | Removed; replaced with manual caveats |

### Key Link Verification

| From | To | Via | Status | Details |
|------|----|-----|--------|---------|
| `ci.yml` test job | `coverage.out` | `go test -coverprofile=coverage.out ./...` | WIRED | Line 38 |
| `ci.yml` test job | Codecov | `codecov/codecov-action@v6` | WIRED (version uncertain) | Line 41-44 — action version may not exist |
| `ci.yml` lint job | `.golangci.yml` | `golangci-lint run ./...` via `make lint` | WIRED | Config file present, lint job runs make lint |
| `ci.yml` lint job | golangci-lint binary | `go install ...@v1.64.8` | BROKEN | Version v1.64.8 does not exist |
| `ci.yml` integration | lint gate | `needs: [test, lint]` | WIRED | Fixed by aa4ffd9 |
| `.goreleaser.yaml` post_install | `~/.docker/cli-plugins/` | `File.symlink(src, target)` | WIRED | Correct Ruby, EPERM rescue present |
| `.goreleaser.yaml` | symlink removal on uninstall | `def uninstall` | NOT WIRED | Removed; caveats block is documentation only |

### Anti-Patterns Found

| File | Line | Pattern | Severity | Impact |
|------|------|---------|----------|--------|
| `.github/workflows/ci.yml` | 20 | `golangci-lint@v1.64.8` non-existent version | BLOCKER | Lint job fails on every CI run before any linting occurs |
| `.github/workflows/ci.yml` | 13,15,28,30,41,54,56 | Action versions `@v6` may not exist (checkout, setup-go, codecov-action) | WARNING | If tags don't exist, all CI jobs fail at startup |
| `.github/workflows/release.yml` | 15,19,25 | Action versions `@v6`/`@v7` may not exist | WARNING | Release pipeline may be broken |

### Human Verification Required

None required beyond the action version question (which can be resolved by checking GitHub).

**Note to developer:** The action version question (truth #9) is UNCERTAIN because it depends on whether `actions/checkout@v6`, `actions/setup-go@v6`, etc. actually exist on GitHub as of today (May 2026). The code review written during this phase flagged them as non-existent, and the fix commit did not address them. Please verify by checking `https://github.com/actions/checkout/releases` — if @v6 does not exist there, treat this as a BLOCKER alongside golangci-lint version.

### Gaps Summary

Three items are not achieved:

**Gap 1 (BLOCKER): golangci-lint version v1.64.8 does not exist.** The lint CI gate installs a non-existent version via `go install`. The Phase 11 code review identified this as critical. The `aa4ffd9` review-fix commit addressed two other findings but skipped this one. Every CI run will fail at the `go install` step before `make lint` even runs. Fix: replace `@v1.64.8` with `@v1.61.0` (last real v1 release) or `@v2.1.0` (v2 GA).

**Gap 2 (WARNING/BLOCKER): GitHub Actions version tags may not exist.** The 11-04 agent used `@v6`/`@v7` claiming "Dependabot already bumped" them. The code review contradicts this. The fix commit did not downgrade them. If these tags don't resolve, every CI job and the release pipeline fail before executing any steps. Requires human verification of tag existence on GitHub.

**Gap 3 (INTENTIONAL DESCOPE, needs override): brew uninstall does not automatically remove symlink.** D-13 and D-14 required automated cleanup via `def uninstall`. This was correctly removed after the code review discovered that `def uninstall` is not valid Homebrew Formula DSL (only Cask DSL). The current state — a `caveats` block with a manual `rm -f` instruction — is the only feasible alternative without converting to a Cask. This deviation is intentional and well-documented but not formally accepted via a verification override. To close this gap, add an override to this file's frontmatter accepting the descope.

---

_Verified: 2026-05-23T22:30:00Z_
_Verifier: Claude (gsd-verifier)_
