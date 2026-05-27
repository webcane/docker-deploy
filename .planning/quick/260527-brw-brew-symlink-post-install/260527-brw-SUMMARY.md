---
phase: quick
plan: 260527-brw
subsystem: homebrew-formula
tags: [brew, formula, sandbox, symlink, goreleaser]
dependency_graph:
  requires: []
  provides: [brew-symlink-post-install]
  affects: [homebrew-tap, goreleaser-template]
tech_stack:
  added: []
  patterns: [homebrew-sandbox-allowlist, homebrew-post-install]
key_files:
  created: []
  modified:
    - /Users/mniedre/git/homebrew-docker-deploy/docker-deploy.rb
    - /Users/mniedre/git/docker-deploy/.goreleaser.yaml
decisions:
  - "Combined sandbox_allowlist? = true WITH post_install — previous attempts tried these separately"
  - "Use Pathname#make_symlink (Ruby API) over system ln for idiomatic Homebrew code"
  - "Added uninstall hook to clean up symlink on brew uninstall"
  - "Kept lib/docker/cli-plugins symlink in install for Docker Desktop / Intel Mac fallback"
  - "Improved caveats to show ln -sf one-liner first, config.json approach as alternative"
metrics:
  duration: ~5 min
  completed: "2026-05-27"
---

# Quick Task 260527-brw: brew install docker-deploy missing symlink in ~/.docker/cli-plugins

**One-liner:** Restored `post_install` + `sandbox_allowlist? = true` combo. Previous fixes tried these
separately — one attempt removed sandbox_allowlist before trying system ln; another tried Ruby symlink
without sandbox_allowlist. Combining both is the correct fix.

## Root Cause Analysis

The Homebrew macOS sandbox blocks writes to `~/.docker/cli-plugins` during `post_install` by default.
`sandbox_allowlist? = true` lifts that restriction for the formula. The history of failed attempts:

1. Ruby `File.symlink` without `sandbox_allowlist?` → EPERM (sandbox blocked)
2. `system "ln"` without `sandbox_allowlist?` → EPERM (sandbox blocked)
3. `sandbox_allowlist?` removed as "dead" → was tested incorrectly; NOT tried with working post_install
4. Fallback to `cliPluginsExtraDirs` caveats → works but requires manual config.json edit

The fix: `sandbox_allowlist? = true` in `custom_block` + `post_install` with Pathname API.

## Changes

**`/Users/mniedre/git/homebrew-docker-deploy/docker-deploy.rb`**
- Added `def sandbox_allowlist? = true` (lifts macOS sandbox restriction for post_install)
- Added `def post_install` block: creates `~/.docker/cli-plugins/`, creates symlink via `make_symlink`, graceful EPERM fallback with clear opoo message
- Added `def uninstall` hook: removes symlink when `brew uninstall docker-deploy`
- Updated `caveats` to show `ln -sf` one-liner first, config.json as alternative

**`/Users/mniedre/git/docker-deploy/.goreleaser.yaml`**
- Added `post_install` field matching the formula behavior
- Added `custom_block` with `sandbox_allowlist? = true` and `uninstall` hook
- Updated `caveats` to match

## Commits

| Repo | Commit | Message |
|------|--------|---------|
| homebrew-docker-deploy | a2ade9c | fix: restore post_install symlink with sandbox_allowlist |
| docker-deploy | 8b4ef05 | fix(brew): restore post_install symlink with sandbox_allowlist |

## Deviations

None.

## Verification

- `grep sandbox_allowlist /Users/mniedre/git/homebrew-docker-deploy/docker-deploy.rb` → line 51
- `grep post_install /Users/mniedre/git/docker-deploy/.goreleaser.yaml` → line 54
- Both repos committed cleanly
