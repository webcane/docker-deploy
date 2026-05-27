---
quick_id: 260527-bsi
status: complete
---

# Quick Task 260527-bsi: Add Apple Silicon Homebrew note to INSTALL.md

## What was done

Added an "Apple Silicon note" subsection under the Homebrew install section in INSTALL.md.

The note explains that Docker does not search `/opt/homebrew/lib/docker/cli-plugins` by default on Apple Silicon, and provides two remedies:

- Option 1: symlink the binary into `~/.docker/cli-plugins/`
- Option 2: add `/opt/homebrew/lib/docker/cli-plugins` to `cliPluginsExtraDirs` in `~/.docker/config.json`

## Files changed

- `INSTALL.md` — added Apple Silicon note under Homebrew section
