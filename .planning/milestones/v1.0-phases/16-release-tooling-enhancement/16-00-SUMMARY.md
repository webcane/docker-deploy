---
phase: "16-release-tooling-enhancement"
plan: "00"
subsystem: "release-tooling"
tags: ["release", "skill", "pre-release-checks", "state-management"]
dependency_graph:
  requires: []
  provides: ["wave-0-pre-release-checks", "state-md-release-update", "commit-body-generation"]
  affects: [".claude/commands/gsd/release-tag.md"]
tech_stack:
  added: []
  patterns: ["wave-0-gates", "auto-fix-retry-lint", "git-log-commit-body"]
key_files:
  created: []
  modified:
    - ".claude/commands/gsd/release-tag.md"
decisions:
  - "D-01: Wave 0 checks run BEFORE version bump question — fail fast, no interactive steps wasted"
  - "D-02: make test → hard abort on any unit test failure"
  - "D-03: make lint → auto-fix with make lint-fix → re-lint → abort only on persistent failures"
  - "D-04: test-ci skips with warning if no Docker socket; hard-aborts if Docker present and tests fail"
  - "D-05: Each check prefixed with ▶ before running; result is PASS/FAIL inline"
  - "D-06: Commit body from git log $PREV_TAG..HEAD --oneline"
  - "D-07: Filter chore:/chore( prefixed lines; keep feat/fix/refactor/docs/test/ci/perf"
  - "D-08: Empty filtered body → subject-line-only commit"
  - "D-09: Body format: Changes since $CURRENT_TAG: with bullet lines"
  - "D-10/D-11: STATE.md last_updated (ISO 8601) and last_activity updated in same release commit"
  - "D-12: milestone: field NOT changed — tracks planning milestones not semver"
  - "D-13: .planning/STATE.md staged in same git add as README.md and INSTALL.md"
metrics:
  duration: "~2 minutes"
  completed: "2026-05-27"
  tasks_completed: 2
  files_modified: 1
---

# Phase 16 Plan 00: Release Tooling Enhancement Summary

**One-liner:** Extended release-tag skill with Wave 0 pre-release gates (unit tests, lint+auto-fix-retry, integration tests with Docker auto-detect) and Wave 1 STATE.md release tracking with git-log-derived commit body.

## Tasks Completed

| Task | Name | Commit | Files |
|------|------|--------|-------|
| 1 | Add Wave 0 pre-release checks | 77cfc02 | .claude/commands/gsd/release-tag.md |
| 2 | Add STATE.md update and commit body generation | 6d97850 | .claude/commands/gsd/release-tag.md |

## What Was Built

### Task 1 — Wave 0 Pre-release Checks

A new "Wave 0 — Pre-release checks" section was inserted before Step 1 in the `release-tag.md` skill. It contains three ordered sub-steps:

**Step A — Unit tests:** Runs `make test`. Hard-aborts with `ABORT: unit tests failed` on non-zero exit. No files are changed.

**Step B — Linter with auto-fix retry:** Runs `make lint`. On failure, auto-runs `make lint-fix` then re-runs `make lint`. Only hard-aborts (`ABORT: lint issues remain after auto-fix`) if the second lint run still fails. Prints `PASS (auto-fixed)` on recovery.

**Step C — Integration tests with Docker auto-detect:** Detects Docker socket via `[ -S /var/run/docker.sock ] || [ -S $HOME/.colima/default/docker.sock ]`. Skips with a non-blocking `WARNING: Docker not detected` if absent. Hard-aborts with `ABORT: integration tests failed` if Docker is present and `make test-ci` fails.

Each check prints a `▶ <command>` prefix before running. After all three pass: `All checks passed — proceeding with release`.

### Task 2 — STATE.md Update and Commit Body Generation

**Step 5 extension:** After updating README.md/INSTALL.md, a new "Update STATE.md" sub-section instructs reading `.planning/STATE.md` and using Edit to set:
- `last_updated:` to the ISO 8601 timestamp via `date -u +"%Y-%m-%dT%H:%M:%SZ"`
- `last_activity:` to `{YYYY-MM-DD} -- Released {NEXT_TAG}`
- `milestone:` is explicitly NOT changed

**Step 6 replacement:** Replaces the bare `git commit` with commit body generation:
- Runs `git log $PREV_TAG..HEAD --oneline`
- Filters out `chore:`/`chore(` lines
- If body is empty (all chores): subject-line-only commit
- If body is non-empty: commit with "Changes since $CURRENT_TAG:" bullet list

**Step 6 git add:** Updated to `git add README.md INSTALL.md .planning/STATE.md`

**Guardrails:** Updated to `Only README.md, INSTALL.md, and .planning/STATE.md are modified; no other files change`

**Step 9 Report:** Added `.planning/STATE.md updated (last_updated, last_activity)` line.

## Deviations from Plan

None — plan executed exactly as written.

## Threat Surface Scan

No new network endpoints, auth paths, file access patterns, or schema changes. The `git log` output injected into the commit message body is developer-authored commit text (T-16-00-01, accepted in plan threat model).

## Self-Check: PASSED

- `.claude/commands/gsd/release-tag.md` modified: FOUND
- Commit 77cfc02 (Task 1): FOUND
- Commit 6d97850 (Task 2): FOUND
- `Wave 0` section before `Step 1`: VERIFIED (line 25 vs line 81)
- All three ABORT conditions present: VERIFIED
- `make test`, `make lint`, `make lint-fix`, `make test-ci`: VERIFIED
- Docker socket detection string: VERIFIED
- `last_updated`, `last_activity`, `milestone:` NOT clause: VERIFIED
- `git log $PREV_TAG..HEAD --oneline`: VERIFIED
- `git add README.md INSTALL.md .planning/STATE.md`: VERIFIED
- Guardrails updated with STATE.md: VERIFIED
- Steps 7, 8, 9 intact: VERIFIED
