# Phase 16: Release Tooling Enhancement — Discussion Log

**Date:** 2026-05-27
**Participants:** User + Claude

---

## Area 1: Pre-release check behavior

**Context:** The release-tag skill currently has no pre-release checks. Wave 0 adds go test, golangci-lint gate (already configured in .golangci.yml), and test-ci (integration tests, needs Docker).

| Question | Options | Decision |
|---|---|---|
| Should test-ci be required to pass before releasing? | Required / Skipped when Docker absent / Always skip | **Skipped when Docker absent** — auto-detect; unit tests + lint still gate the release |
| What should happen when go test fails? | Hard abort / Warn and ask to continue / You decide | **Hard abort** — no commit, no tag, clear error |
| Where in the release-tag flow should checks run? | Before version bump question / After version selection / After confirmation | **Before version bump question** — fail fast |

**Outcome:** Pre-release checks gate the entire flow before user interaction. Hard abort on test failure. test-ci auto-skipped when Docker socket unreachable.

---

## Area 2: Commit message body format

**Context:** Current release commit is just "chore: bump version to $NEXT_TAG". Wave 1 wants a meaningful body.

| Question | Options | Decision |
|---|---|---|
| How should body be generated? | REQ IDs / Fixed template from SUMMARY.md / Configurable per-release | **User redirected** — look at /gsd-ship for ideas; only diff is local pre-check and tag, no PR |
| What source drives the body? | Git log since last tag / ROADMAP.md goals / research/SUMMARY.md executive summary | **Git log since last tag** — shows exactly what changed |
| How to present commits? | Filtered onelines (exclude chore) / Full oneline list / You decide | **Filtered onelines** — exclude `chore:` / `chore(` prefixes; keep feat/fix/refactor/docs/test/ci |

**Outcome:** Body = filtered `git log $PREV_TAG..HEAD --oneline`. Omit body entirely if all commits are chores.

---

## Area 3: STATE.md update scope

**Context:** STATE.md has YAML frontmatter + narrative body. Which fields update on release?

| Question | Options | Decision |
|---|---|---|
| Which fields to update? | milestone / last_updated / last_activity / stopped_at | **last_updated** + **last_activity** only |
| When relative to commit+tag? | Include in same release commit / Separate commit before tag / After tag | **Same release commit** — one atomic change with README.md + INSTALL.md |

**Outcome:** `last_updated` set to ISO 8601 timestamp; `last_activity` set to `{date} -- Released {NEXT_TAG}`. Staged with README.md + INSTALL.md in the bump commit. `milestone:` field NOT touched.

---

## Area 4: Terminal demo tool & placement

**Context:** Wave 2 calls for a reproducible terminal demo (vhs or asciinema) embedded in README.md.

| Question | Options | Decision |
|---|---|---|
| Which tool? | vhs (GIF) / asciinema (.cast) / Skip Wave 2 | **Skip Wave 2** — terminal demo deferred |

**Outcome:** Wave 2 deferred. No demo recording in this phase. Noted as deferred idea for future phase.

---

## Claude's Discretion

- Exact shell snippet for Docker socket detection in the release skill: reuse the condition from `Makefile`'s `test-ci` target verbatim for consistency
- Formatting of pre-release check output: `▶ go test ./...` prefix style (matches existing CI output conventions)
- Omit commit body when all commits since last tag are chores — cleaner than a body with zero meaningful entries

## Deferred Ideas

- **Terminal demo (Wave 2)**: vhs tape file → GIF in README.md. Deferred by explicit user choice. Revisit if visitor engagement data warrants it.
