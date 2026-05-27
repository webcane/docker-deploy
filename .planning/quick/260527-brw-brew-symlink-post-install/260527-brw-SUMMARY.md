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

**One-liner:** `post_install` cannot write to `~/.docker/cli-plugins` on macOS — the sandbox blocks home
dir writes and `sandbox_allowlist?` does not bypass it for `~/` paths. Settled on keeping the
`lib/docker/cli-plugins` install symlink (works for Docker Desktop) and improving caveats with an
explicit `mkdir + ln -sf` two-liner for colima/Docker Engine users.

## Root Cause Analysis

The macOS sandbox applied to Homebrew's `post_install` blocks writes to any path under `~/`.
`sandbox_allowlist?` is not a sandbox escape for home directory writes — it is an internal Homebrew
concept that does not override the macOS sandbox profile for `~/.docker/`.

All previous post_install attempts failed for the same reason:
1. Ruby `File.symlink` → EPERM (sandbox blocked)
2. `system "ln"` → EPERM (sandbox blocked)
3. `sandbox_allowlist? = true` combined with Pathname API → also blocked (this session's attempt)

**What works:**
- `(lib/"docker/cli-plugins").install_symlink` in `install` → writes to `HOMEBREW_PREFIX`, always allowed
- Docker Desktop automatically adds `HOMEBREW_PREFIX/lib/docker/cli-plugins` to its plugin search path
- Intel Mac Docker Engine checks `/usr/local/lib/docker/cli-plugins` (same as Homebrew prefix)
- Apple Silicon + colima/Engine users must run the `ln -sf` manually (captured in caveats)

## Changes

**`/Users/mniedre/git/homebrew-docker-deploy/docker-deploy.rb`**
- Removed `sandbox_allowlist?`, `post_install`, and `uninstall` hooks (do not work)
- Updated `caveats` to clearly distinguish Docker Desktop (automatic) from colima/Engine (manual `ln -sf`)

**`/Users/mniedre/git/docker-deploy/.goreleaser.yaml`**
- Removed `post_install` and `custom_block`
- Updated `caveats` to match

## Commits

| Repo | Commit | Message |
|------|--------|---------|
| homebrew-docker-deploy | fc259a8 | fix: remove post_install/sandbox_allowlist, improve caveats |
| docker-deploy | cee1cea | fix(brew): remove post_install/sandbox_allowlist, improve caveats |

## Deviations

Initial attempt added `sandbox_allowlist? = true` + `post_install` — reverted after confirming
the macOS sandbox blocks home dir writes regardless of `sandbox_allowlist?`.
