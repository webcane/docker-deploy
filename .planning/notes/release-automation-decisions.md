---
title: Release automation design decisions
date: 2026-05-24
context: Exploration of auto-updating README version on release
---

# Release Automation Design Decisions

## Approach chosen: local `/release-tag` skill

Rather than having GitHub Actions commit the version bump back to the repo, the release process is driven by a local skill. This keeps CI simple and avoids bot commits or dirty-state races.

## Key decisions

### Local skill, not CI commit-back
- CI fires **after** the tag is pushed; having it commit back to `main` introduces race conditions and requires write tokens
- A local skill gives the developer a clear, interactive confirmation step before anything is published

### Only README.md needs version sync
- The install script URL in `README.md` is the only version reference that drifts
- Other docs (PREREQUISITES.md, DEPLOY_CONFIG.md, etc.) are version-agnostic

### Semver bump with user confirmation
- Skill detects latest tag and suggests next version (default: patch bump)
- Developer selects major / minor / patch — no fully-automatic bump to avoid surprises
- Confirmation before commit/tag/push

### Release trigger
- GitHub Actions is configured to fire on `v*` tags only
- The skill is the sole entry point for creating releases — no manual tagging outside the skill

## Flow summary

```
/release-tag
  → detect latest tag (e.g. v0.7.8)
  → prompt: major / minor / patch → suggest v0.7.9
  → update README.md install URL
  → git commit + git tag + git push
  → GitHub Actions fires → binary build + GitHub Release created
```
