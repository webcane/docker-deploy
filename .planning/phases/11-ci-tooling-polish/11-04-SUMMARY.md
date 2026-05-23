---
plan: 11-04
phase: 11-ci-tooling-polish
status: complete
completed: 2026-05-23
commit: 668955c
---

# Plan 11-04: golangci-lint CI Gate

## What Was Built

Added golangci-lint config, Makefile targets, and a parallel CI lint job that blocks merge on lint failures.

## Key Files Created/Modified

- `.golangci.yml` — new file; `disable-all: true` with four enabled linters: `errcheck`, `govet`, `staticcheck`, `goimports`; `goimports.local-prefixes: github.com/webcane/docker-deploy`; `max-issues-per-linter: 0` and `max-same-issues: 0` to show all issues
- `Makefile` — `.PHONY` updated to include `lint fmt`; `lint:` target runs `golangci-lint run ./...`; `fmt:` target runs `goimports -w -local github.com/webcane/docker-deploy ./...`
- `.github/workflows/ci.yml` — new `lint` job: `actions/checkout@v6`, `actions/setup-go@v6`, `go install golangci-lint@v1.64.8`, `make lint`; no `needs:` (runs parallel with `test`)

## Deviations

- Action versions in ci.yml are `@v6` (Dependabot already bumped from @v4/@v5) — consistent with rest of file

## Self-Check: PASSED

- `.golangci.yml` exists with `disable-all: true` and four linters enabled ✓
- `goimports.local-prefixes: github.com/webcane/docker-deploy` configured ✓
- Makefile has `lint` and `fmt` targets with correct commands ✓
- `ci.yml` `lint` job installs `golangci-lint@v1.64.8` and runs `make lint` ✓
- `lint` job has no `needs:` key (runs in parallel with `test`) ✓
