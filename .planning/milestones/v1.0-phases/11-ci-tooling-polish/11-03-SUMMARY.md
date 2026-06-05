---
plan: 11-03
phase: 11-ci-tooling-polish
status: complete
completed: 2026-05-23
commits: 54f76fd, 213deca, c1bc972 (plus tap fixes e2388f9, c397e11)
---

# Plan 11-03: Homebrew Symlink Lifecycle

## What Was Built

Updated `.goreleaser.yaml` and the live Homebrew tap formula to automate the Docker CLI plugin symlink lifecycle. On platforms that support it, `brew install` creates `~/.docker/cli-plugins/docker-deploy` automatically; `brew uninstall` removes it.

## Key Files Modified

- `.goreleaser.yaml` — brews block updated:
  - `post_install`: creates `~/.docker/cli-plugins/` + symlinks binary; gracefully degrades with `opoo` warning on macOS Tahoe where brew's post_install context blocks `symlink()` (EPERM)
  - `custom_block`: `def sandbox_allowlist?` (no-op hint for future Homebrew versions) + `def uninstall` removes symlink via `File.delete` with `File.exist?` guard
  - `caveats:` block removed entirely
- `README.md` — Homebrew section manual symlink instructions removed
- Tap repo (`webcane/homebrew-docker-deploy`) — formula updated directly for v0.7.0

## Key Findings

**macOS Tahoe (26.4.1) security restriction:** Homebrew's `post_install` subprocess cannot create symlinks outside Homebrew's own prefix on macOS Tahoe. This affects `File.symlink`, `system "ln"`, and `system "/usr/bin/python3"` equally. The issue is OS-level (not Homebrew's sandbox, which returns `Sandbox.available? = false`). The `begin/rescue Errno::EPERM` approach gracefully falls back to a user-facing `opoo` warning with the exact manual command, avoiding post-install failure. The `def uninstall` hook works correctly on all platforms (removes symlink via Ruby's `File.delete`).

## Human Checkpoint Result

Approved after investigation: graceful fallback approach approved. Works on Linux and macOS versions below Tahoe; degrades cleanly on Tahoe with actionable warning.

## Deviations

- `post_install` uses `begin/rescue Errno::EPERM` fallback (not in original plan) — required due to macOS Tahoe security restriction discovered during testing
- `sandbox_allowlist?` added to `custom_block` (future-proofing, currently no-op since Homebrew 5.x doesn't check it)

## Self-Check: PASSED

- `.goreleaser.yaml` brews block contains `post_install` with `File.symlink` + EPERM rescue ✓
- `.goreleaser.yaml` brews block contains `custom_block` with `def uninstall` using `File.delete`/`File.exist?` ✓
- `caveats:` key absent from `.goreleaser.yaml` ✓
- README Homebrew section has no manual symlink instructions ✓
- YAML syntactically valid (python3 yaml.safe_load check passed) ✓
- Human checkpoint approved ✓
- Tap formula pushed to GitHub (`webcane/homebrew-docker-deploy`) ✓
