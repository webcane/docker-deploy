---
phase: quick
plan: 260524-brw
subsystem: homebrew-formula
tags: [brew, formula, sandbox, symlink, goreleaser]
dependency_graph:
  requires: []
  provides: [brew-symlink-fix]
  affects: [homebrew-tap, goreleaser-template]
tech_stack:
  added: []
  patterns: [homebrew-sandbox-allowlist]
key_files:
  created: []
  modified:
    - /Users/mniedre/git/homebrew-docker-deploy/docker-deploy.rb
    - /Users/mniedre/git/docker-deploy/.goreleaser.yaml
decisions:
  - "Used single-line `def sandbox_allowlist? = true` form (not block form) to match what should appear in the generated formula"
  - "Updated opoo message to reference `brew info` rather than duplicate the symlink command"
  - "Updated caveats to include manual symlink command as fallback for older Homebrew or Linux"
metrics:
  duration: ~5 min
  completed: "2026-05-24"
---

# Quick Task 260524-brw: Fix brew install symlink warning — add sandbox_allowlist to formula and goreleaser template

**One-liner:** Added `def sandbox_allowlist? = true` to live formula and goreleaser custom_block so Homebrew's macOS sandbox permits writing the CLI plugin symlink to `~/.docker/cli-plugins/` during `post_install`.

## What Was Done

### Root Cause

`brew install docker-deploy` printed "Warning: Could not create symlink automatically. Run manually: ln -sf ..." because Homebrew's macOS sandbox blocks filesystem writes outside the Homebrew prefix by default. The `post_install` block uses `File.symlink` to write to `~/.docker/cli-plugins/`, which raises `Errno::EPERM` and hits the rescue path.

The fix is declaring `def sandbox_allowlist? = true` in the formula class — this tells Homebrew to lift the sandbox restriction for the home directory during `post_install`.

The live formula was missing this declaration entirely. The `.goreleaser.yaml` had `custom_block` with the method in block form (`def sandbox_allowlist?` / `true` / `end`), but the generated formula did not include it — the single-line form `def sandbox_allowlist? = true` is the reliable form to use in `custom_block`.

### Changes

**`/Users/mniedre/git/homebrew-docker-deploy/docker-deploy.rb`**
- Added `def sandbox_allowlist? = true` immediately before `def post_install`
- Updated `opoo` rescue message to reference `brew info docker-deploy` instead of duplicating the symlink command
- Expanded `caveats` block to include the manual `ln -sf` command as a fallback (for older Homebrew or Linux where `sandbox_allowlist?` may not apply)

**`/Users/mniedre/git/docker-deploy/.goreleaser.yaml`**
- Changed `custom_block` from block form to single-line: `def sandbox_allowlist? = true`
- Updated `post_install` rescue `opoo` message to match the formula

## Commits

| Repo | Commit | Message |
|------|--------|---------|
| homebrew-docker-deploy | 47217cf | fix: add sandbox_allowlist and improve caveats for cli-plugins symlink |
| docker-deploy | d2c702c | fix(brew): goreleaser template — sandbox_allowlist and updated opoo message |

## Deviations from Plan

None — plan executed exactly as written.

## Verification

- `grep sandbox_allowlist /Users/mniedre/git/homebrew-docker-deploy/docker-deploy.rb` returns line 47: `def sandbox_allowlist? = true`
- Both repos have clean commits
- Future `goreleaser release` runs will regenerate the formula with `sandbox_allowlist?` present in `custom_block`

## Self-Check: PASSED

- `/Users/mniedre/git/homebrew-docker-deploy/docker-deploy.rb` — FOUND and contains `sandbox_allowlist?`
- `/Users/mniedre/git/docker-deploy/.goreleaser.yaml` — FOUND and contains `def sandbox_allowlist? = true`
- Tap repo commit 47217cf — FOUND
- Main repo commit d2c702c — FOUND
