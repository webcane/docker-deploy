---
title: Build /release-tag skill
date: 2026-05-24
priority: medium
---

# Build `/release-tag` skill

A local release skill that bundles version bump, README update, tagging, and push into one command.

## Steps the skill must perform

1. **Detect latest tag** — `git describe --tags --abbrev=0` or `git tag --sort=-v:refname | head -1`
2. **Parse semver** — split `vMAJOR.MINOR.PATCH`
3. **Prompt for bump type** — ask major / minor / patch; suggest a default (e.g. patch)
4. **Compute next version** — increment the selected component, reset lower components
5. **Update README.md** — replace the hardcoded install URL version with the new tag
6. **Commit README change** — `git commit -m "chore: bump version to vX.Y.Z"`
7. **Create tag** — `git tag vX.Y.Z`
8. **Push** — `git push && git push --tags` → triggers GitHub Actions CI/release workflow

## Acceptance criteria

- [ ] Skill reads latest tag automatically (no manual input required)
- [ ] User confirms the suggested next version before any changes are made
- [ ] README install URL is the only file modified
- [ ] Commit, tag, and push happen in sequence with clear output
- [ ] If push fails, local tag is left in place with instructions to retry

## Notes

- GitHub Actions fires on `v*` tag push — this skill is the sole trigger
- No commit-back from CI needed; README is updated locally before the tag
