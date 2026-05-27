---
quick_id: 260527-brw
slug: brew-symlink-post-install
description: "brew install docker-deploy missing symlink in ~/.docker/cli-plugins/docker-deploy"
created: "2026-05-27"
must_haves:
  truths:
    - ".goreleaser.yaml brews section has post_install block creating ~/.docker/cli-plugins/docker-deploy symlink"
    - ".goreleaser.yaml brews section has custom_block with sandbox_allowlist? = true"
    - "live formula has sandbox_allowlist? = true and post_install"
    - "caveats include manual ln -sf fallback instructions"
  artifacts:
    - ".goreleaser.yaml"
    - "/Users/mniedre/git/homebrew-docker-deploy/docker-deploy.rb"
---

# Quick Plan 260527-brw: brew install docker-deploy missing symlink

## Root Cause

`brew install docker-deploy` installs to `HOMEBREW_PREFIX/lib/docker/cli-plugins` but Docker CLI
only checks `~/.docker/cli-plugins` by default on Apple Silicon. Previous attempts to fix via
`post_install` failed because the Homebrew macOS sandbox blocks `~/.docker` writes — but those
attempts never combined `sandbox_allowlist? = true` WITH the `post_install` block. The two
were tried separately; combining them is the correct fix.

## Tasks

### Task 1: Restore post_install + sandbox_allowlist in .goreleaser.yaml

**Files:** `.goreleaser.yaml`

**Action:**
- Add `post_install` block that creates `~/.docker/cli-plugins/docker-deploy` symlink using Pathname Ruby API
- Add `custom_block` with `sandbox_allowlist? = true` and `uninstall` hook (removes symlink on `brew uninstall`)
- Update `caveats` to include both the config.json approach AND a direct `ln -sf` one-liner

**Verify:** `grep -A2 "post_install" .goreleaser.yaml` shows the symlink creation code

### Task 2: Update live tap formula

**Files:** `/Users/mniedre/git/homebrew-docker-deploy/docker-deploy.rb`

**Action:**
- Add `def sandbox_allowlist? = true` before `def caveats`
- Add `def post_install` block matching the goreleaser template
- Add `def uninstall` block (removes `~/.docker/cli-plugins/docker-deploy` symlink)
- Update caveats to include manual symlink command as fallback

**Verify:** `grep sandbox_allowlist /Users/mniedre/git/homebrew-docker-deploy/docker-deploy.rb` returns the method
