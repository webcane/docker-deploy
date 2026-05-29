---
created: "2026-05-29 00:00"
title: "Update --host flag and usage docs for SSH config alias support"
area: docs
files:
  - cmd/docker-deploy/main.go:73
  - README.md
  - USAGE.md
---

## Problem

Phase 14 added SSH config alias resolution to `--host`, but the `--host` flag help text still only described the `ssh://user@host:port` URL form. README usage scenarios also made no mention of SSH config aliases, and all scenarios were inlined in README with no dedicated usage reference file.

## Solution

1. Updated `--host` flag description in `cmd/docker-deploy/main.go:73` to read: `"Remote host: ssh://user@host:port URL or SSH config alias (overrides deploy.yaml)"`
2. Simplified README Usage section to a Quick Start showing both the URL and alias forms, with a link to USAGE.md for full scenarios.
3. Created `USAGE.md` covering: URL format, SSH config alias, deploy.yaml config, custom path, excludes, non-root recommended setup, flags-only deploy, the `validate` subcommand, and precedence rules.

Committed as:
- `86cbc7e` — `docs: update --host flag description to mention SSH config aliases`
- `0925366` — `docs: add SSH config alias usage to README and USAGE.md`
