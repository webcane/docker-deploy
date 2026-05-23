---
plan: 11-01
phase: 11-ci-tooling-polish
status: complete
completed: 2026-05-23
commit: 0d3c3b5
---

# Plan 11-01: Codecov Coverage Reporting

## What Was Built

Wired Codecov coverage reporting into the CI pipeline and fixed the README badge.

## Key Files Created/Modified

- `codecov.yml` — minimal Codecov config; `comment.require_changes: true` suppresses noisy per-PR comments; `coverage.status.project.default.target: auto` allows natural variation
- `.github/workflows/ci.yml` — test job now runs `go test -coverprofile=coverage.out ./...`; new "Upload coverage to Codecov" step uses `codecov/codecov-action@v4` tokenless (no `token:` key; `fail_ci_if_error: false` so Codecov outages don't break CI)
- `README.md` — Codecov badge branch corrected from `/branch/main/` to `/branch/master/` (repo has no main branch)

## Deviations

None.

## Self-Check: PASSED

- `codecov.yml` exists with `require_changes: true` ✓
- `ci.yml` test job runs `go test -coverprofile=coverage.out ./...` ✓
- `ci.yml` test job has `codecov/codecov-action@v4` upload step with no token and `fail_ci_if_error: false` ✓
- README Codecov badge URL references `/branch/master/` not `/branch/main/` ✓
