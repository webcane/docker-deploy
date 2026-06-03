---
quick_id: 260603-abc
slug: close-codecov-todo
description: Promote codecov replacement todo to implementation — close out pending TODO now that 379562e landed the fix
date: 2026-06-03
must_haves:
  truths:
    - pending todo file moved to done/
    - done file annotated with commit reference 379562e
  artifacts:
    - .planning/todos/done/2026-06-03-replace-codecov-with-tj-actions-badge.md
---

# Quick Task 260603-abc: Promote codecov replacement todo to implementation

## Context

Implementation already landed in commit `379562e` which replaced the codecov badge with
`tj-actions/coverage-badge-go` in `.github/workflows/ci-jobs.yml` and updated `README.md`.
The pending TODO file was never closed out.

## Task 1: Close out pending TODO

**files:** `.planning/todos/pending/2026-06-03-replace-codecov-with-tj-actions-badge.md`
**action:** Move to `.planning/todos/done/`, annotate with `completed` date and `commit: 379562e`
**verify:** File exists in `done/`, absent from `pending/`
**done:** ✓
