---
title: Extend /gsd:release-tag with REQ summary, STATE.md update, and commit message generation
date: 2026-05-24
priority: medium
---

Extend the `/gsd:release-tag` skill to do the full release flow in one command:

1. **Collect REQ summary** — read `.planning/research/SUMMARY.md` to extract requirements; use this to generate a meaningful commit message body describing what shipped
2. **Update STATE.md** — mark the release in state (version bumped, date, tag)
3. **Bump version in README.md and INSTALL.md** — both files pin the version in `curl` install snippets; both must be updated together
4. **Commit with generated summary** — use the REQ-derived summary as the commit message body, not a generic "bump version" line
5. **Tag + push** — semver tag, push commit and tag to trigger CI release

**Why:** Without this, release requires manually updating README, STATE.md, and writing a commit message — three steps that should be one.
