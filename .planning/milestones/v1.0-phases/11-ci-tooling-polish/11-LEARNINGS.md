---
phase: 11
phase_name: "ci-tooling-polish"
project: "docker-deploy"
generated: "2026-06-05"
counts:
  decisions: 6
  lessons: 5
  patterns: 4
  surprises: 3
missing_artifacts:
  - "11-SECURITY.md (present but not in original artifact list — treated as bonus)"
---

# Phase 11 Learnings: CI & Tooling Polish

## Decisions

### Tokenless Codecov Upload for Public Repos
For a public GitHub repo, Codecov uploads do not require a token. The `codecov/codecov-action@v4` step runs without a `token:` key and `fail_ci_if_error: false` ensures Codecov outages do not break CI.

**Rationale:** Coverage data is not sensitive for a public repo; tokenless upload is simpler to configure and has no security downside.
**Source:** 11-01-PLAN.md

---

### Pin golangci-lint to a Specific Version, Not @latest
The lint CI job pins golangci-lint to a specific version (`@v1.64.8` in the plan, though this version turned out not to exist) via `go install`. Using `@latest` is non-reproducible — a new release can add linters or change behavior, causing CI failures on unchanged code.

**Rationale:** Reproducible CI gates require pinned tool versions; Dependabot handles future bumps.
**Source:** 11-04-PLAN.md

---

### disable-all: true in golangci-lint Config
The `.golangci.yml` uses `disable-all: true` and explicitly enables exactly four linters: errcheck, govet, staticcheck, goimports. This prevents surprise linter additions when golangci-lint releases new defaults.

**Rationale:** An unexpected new linter enabled by a version bump can break CI on unchanged code; explicit enable list is stable.
**Source:** 11-04-PLAN.md

---

### Homebrew post_install Automates Symlink Creation
The GoReleaser brews block uses `post_install` (Ruby DSL) with `File.symlink` to create the `~/.docker/cli-plugins/docker-deploy` symlink automatically on `brew install`, replacing a manual caveats instruction.

**Rationale:** Removing the manual step from caveats eliminates user friction and errors.
**Source:** 11-03-PLAN.md

---

### goimports Local Prefix Matches Module Path
The `goimports` linter is configured with `local-prefixes: github.com/webcane/docker-deploy` to group project imports as a third import group (separate from stdlib and third-party). The same prefix is used in the Makefile `fmt` target for consistency.

**Rationale:** Consistent import grouping across CI and local formatting prevents spurious lint failures.
**Source:** 11-04-PLAN.md

---

### Dependabot Weekly Schedule for GitHub Actions Only
Phase 11 adds Dependabot for the `github-actions` ecosystem only (not Go modules or Docker). Schedule is weekly with `open-pull-requests-limit: 5` to avoid noise.

**Rationale:** Actions version management was the immediate problem; Go module updates are a separate concern; limiting PRs prevents automation fatigue.
**Source:** 11-02-PLAN.md

---

## Lessons

### goimports Does Not Support the ./... Pattern
Running `goimports -w -local ... ./...` fails with `stat ./...: no such file or directory`. The `./...` syntax is a Go tool convention (go build, go test) but goimports treats it as a literal path. The correct form is `find . -name '*.go' | xargs goimports -w -local ...`.

**Context:** Discovered during UAT when the user ran `make fmt` and received a make error.
**Source:** 11-UAT.md

---

### golangci-lint Version v1.64.8 Does Not Exist
The plan specified `golangci-lint@v1.64.8` but the v1 series ended around v1.61.x; v2.x is the current series. CI ran with a non-existent version string, meaning the lint gate would fail at `go install` before any linting occurred.

**Context:** The Verification report caught this as a BLOCKER after plans were executed and committed.
**Source:** 11-VERIFICATION.md

---

### def uninstall Is Not Valid Homebrew Formula DSL
`def uninstall` is only valid in Homebrew Cask class bodies, not Formula. The plan's design called for `def uninstall` to automate symlink removal on `brew uninstall`, but this hook is silently ignored in a Formula. The feasible alternative is a caveats block with a manual `rm -f` instruction.

**Context:** Discovered during implementation and confirmed during verification; resulted in an intentional descope override.
**Source:** 11-VERIFICATION.md

---

### Dependabot May Bump Actions Before CI Runs That Reference Them
Dependabot merged PRs bumping action versions (checkout@v4→v6, setup-go@v5→v6, etc.) independently of plan execution. The 11-04 agent assumed "Dependabot already bumped" and used @v6/@v7 tags, but these tags were flagged by the code reviewer as potentially non-existent. Verifying tag existence on GitHub is necessary before using Dependabot-proposed versions.

**Context:** Verification report flagged action version existence as uncertain; required override with evidence from GitHub API.
**Source:** 11-VERIFICATION.md

---

### macOS Tahoe Blocks Homebrew post_install Symlinks
macOS Tahoe (26.4.1) applies an OS-level security restriction that prevents Homebrew's `post_install` subprocess from creating symlinks outside Homebrew's own prefix. This affects `File.symlink`, `system "ln"`, and `system "/usr/bin/python3"` equally. The Homebrew sandbox API (`Sandbox.available?`) reports `false`, so this is an OS-level restriction, not the Homebrew sandbox.

**Context:** Discovered during hands-on testing; led to adding `begin/rescue Errno::EPERM` fallback that prints an `opoo` warning with the manual command.
**Source:** 11-03-SUMMARY.md

---

## Patterns

### fail_ci_if_error: false for Non-Critical External Services
For CI steps that call external services (Codecov, future badge services), set `fail_ci_if_error: false`. This prevents an outage at the external service from blocking merges on unrelated code.

**When to use:** Any CI step that uploads metadata to a third-party service where an upload failure does not affect correctness of the build artifact.
**Source:** 11-01-PLAN.md

---

### require_changes: true in Codecov Config to Suppress Noisy PR Comments
Setting `comment.require_changes: true` in `codecov.yml` gates PR comments on actual coverage changes. PRs that do not touch tested code produce no coverage comment, reducing noise.

**When to use:** Projects where many PRs touch non-tested code paths (docs, configs, CI) and per-PR coverage comments are more noisy than helpful.
**Source:** 11-01-PLAN.md

---

### Parallel Lint Job with Branch Protection Gate
The CI lint job runs in parallel with the test job (no `needs:` dependency on test). Lint failures block PRs via branch protection rules, not by blocking the integration job from running. This keeps the pipeline fast when test is slow.

**When to use:** Any project where lint and test are independent; lint should not wait for tests to complete.
**Source:** 11-04-PLAN.md

---

### Graceful Fallback with opoo for Homebrew Restrictions
When a Homebrew `post_install` action may fail due to platform restrictions (e.g., macOS Tahoe EPERM), use `begin/rescue Errno::EPERM` and call `opoo` with the manual command as a fallback. This produces a visible user-facing warning without failing `brew install`.

**When to use:** Any Homebrew formula that creates files or symlinks outside the Homebrew prefix — macOS version-specific restrictions may apply.
**Source:** 11-03-SUMMARY.md

---

## Surprises

### FORCE_JAVASCRIPT_ACTIONS_TO_NODE24 Was a Node.js Deprecation Workaround
The `FORCE_JAVASCRIPT_ACTIONS_TO_NODE24: "true"` env var in all CI jobs was a temporary workaround for Node.js 20 deprecation warnings emitted by GitHub Actions runners. Once action versions were bumped to their current major versions (which already run on Node.js 20+), this env var became unnecessary and was removed.

**Impact:** Removing it cleans up noise in CI output; its presence in the original workflows was invisible technical debt.
**Source:** 11-02-PLAN.md

---

### Homebrew Formula def uninstall Is a Cask-Only Hook
The plan's design assumed `def uninstall` was a valid Homebrew Formula lifecycle hook. It is actually a Cask-only API. In a Formula body, it is silently ignored — there is no error, just no execution on `brew uninstall`. Automated symlink cleanup on uninstall is not achievable in a Formula without converting to a Cask.

**Impact:** The phase goal of automated symlink removal on `brew uninstall` was formally descoped; the workaround is a manual caveats instruction.
**Source:** 11-VERIFICATION.md

---

### Verification Caught Two Blockers Not Caught by Self-Check
The plan self-checks all reported PASSED, but the verification step caught two blockers that the summaries missed: (1) golangci-lint version v1.64.8 does not exist, and (2) GitHub Actions version tags @v6/@v7 may not exist. Both would cause CI to fail on every run. The verification's independent code audit provided genuine quality gating beyond what the executing agent checked.

**Impact:** Reinforces the value of a separate verification pass that re-checks pinned external resource identifiers against their actual availability.
**Source:** 11-VERIFICATION.md
