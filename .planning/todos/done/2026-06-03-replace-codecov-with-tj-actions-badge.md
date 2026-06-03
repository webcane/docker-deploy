---
created: 2026-06-03T00:00:00Z
completed: 2026-06-03T00:00:00Z
commit: 379562e
title: Replace codecov with tj-actions coverage badge
area: tooling
files:
  - .github/workflows/ci-jobs.yml
  - README.md
---

## Problem

The codecov badge in the README shows "unknown" status (noted during UAT in session 84708ac). Codecov requires an external account/token integration. A self-contained GitHub Actions approach using `tj-actions/coverage-badge-go` generates a `coverage.svg` committed directly to the repo — no external service dependency.

## Solution

Add a new GitHub Actions workflow (`.github/workflows/coverage-badge.yml`) using `tj-actions/coverage-badge-go@v3`:

1. Run `go test -v -covermode=count -coverprofile=coverage.out ./...` then `go tool cover -func=coverage.out -o=coverage.out`
2. Use `tj-actions/coverage-badge-go@v3` with `filename: coverage.out` to generate/update `coverage.svg` in repo root
3. Use `tj-actions/verify-changed-files@v16` to detect if `README.md` changed
4. Commit and push `README.md` (badge reference) when changed

Workflow trigger: `pull_request` targeting `main` (or also `push` to `main`).

Update `README.md` badge to:
```markdown
![Coverage](https://github.com/webcane/docker-deploy/blob/main/coverage.svg)
```

Remove or disable the existing codecov badge/integration once the new badge is live.
