---
phase: 01-plugin-scaffolding
plan: 02
subsystem: ci-pipeline
tags: [goreleaser, github-actions, ci, release, cross-platform]
dependency_graph:
  requires:
    - go.mod with module github.com/mniedre/docker-deploy (from 01-01)
    - cmd/docker-deploy/main.go with var version = "dev" (from 01-01)
  provides:
    - .goreleaser.yaml targeting linux/amd64 and linux/arm64 with ldflags version injection
    - .github/workflows/ci.yml running go test + go vet on every push and PR
    - .github/workflows/release.yml publishing GoReleaser archives on v* tag
  affects: []
tech_stack:
  added:
    - GoReleaser v2 schema (.goreleaser.yaml)
    - goreleaser/goreleaser-action@v6 (GitHub Actions)
    - actions/checkout@v4
    - actions/setup-go@v5
  patterns:
    - go-version-file: go.mod for single-source Go version (D-14)
    - permissions: contents: write scoped to release job only (T-02-03)
    - fetch-depth: 0 for GoReleaser changelog generation
key_files:
  created:
    - .goreleaser.yaml
    - .github/workflows/ci.yml
    - .github/workflows/release.yml
  modified: []
decisions:
  - No darwin/windows targets in GoReleaser per D-09 and D-10 (VPS-only delivery)
  - go-version-file instead of hardcoded version string satisfies D-14 single version
  - permissions scoped to contents:write only per T-02-03 (GITHUB_TOKEN minimal scope)
  - goreleaser-action@v6 major-version tag per T-02-04 acceptance (SHA pinning deferred)
metrics:
  duration: ~5 minutes
  completed_date: "2026-05-13"
  tasks_completed: 2
  tasks_total: 2
  files_created: 3
  files_modified: 0
---

# Phase 1 Plan 2: CI/CD Pipeline Summary

**One-liner:** GoReleaser v2 config producing linux/amd64+arm64 tar.gz archives with SHA256 checksums, wired to GitHub Actions CI (every push/PR) and release (v* tag) workflows with minimal GITHUB_TOKEN scope.

## Objective

Add GoReleaser configuration and GitHub Actions workflows so the project has a tested CI pipeline on every push and produces signed cross-platform release archives on a v* tag push.

## Tasks Completed

| Task | Name | Commit | Files |
|------|------|--------|-------|
| 1 | Create .goreleaser.yaml for linux cross-platform builds | 1636872 | .goreleaser.yaml |
| 2 | Create GitHub Actions CI and release workflows | 7b461ab | .github/workflows/ci.yml, .github/workflows/release.yml |

## Verification Results

All 12 verification criteria passed:

1. `.goreleaser.yaml` exists with `goos: [linux]`
2. `goarch: [amd64, arm64]` — no darwin or windows targets
3. `binary: docker-deploy` inside archives
4. ldflags inject `-X main.version={{.Version}}`
5. `format: tar.gz` archives
6. `checksums.txt` with SHA256 algorithm
7. `.github/workflows/ci.yml` triggers on push and pull_request
8. CI runs `go test ./...`
9. CI runs `go vet ./...`
10. `.github/workflows/release.yml` triggers on `v*` tags only
11. Release workflow uses `goreleaser/goreleaser-action@v6` with `release --clean`
12. `GITHUB_TOKEN` passed as env var; `permissions: contents: write` scoped to release job

## Success Criteria Met

- CI workflow fires on every push and PR, runs `go test ./...` (D-12) ✓
- Release workflow fires on `v*` tag push, runs GoReleaser (D-13) ✓
- GoReleaser builds linux/amd64 and linux/arm64 only (D-09); macOS excluded (D-10) ✓
- Release archives are `.tar.gz` with `checksums.txt` (D-11) ✓
- Binary inside archives is named `docker-deploy` (PLUG-01) ✓
- ldflags inject `main.version` with the git tag (D-08) ✓

## Deviations from Plan

None — plan executed exactly as written.

## Known Stubs

None — configuration files only; no data flows or UI rendering.

## Threat Flags

None — all STRIDE mitigations from the plan's threat model are implemented:

- T-02-01: `checksums.txt` SHA256 configured in `.goreleaser.yaml`
- T-02-03: `permissions: contents: write` scoped to release job only in `release.yml`
- T-02-02 and T-02-04: accepted as per threat register

## Self-Check: PASSED

- .goreleaser.yaml exists and contains correct goos/goarch/binary/ldflags/checksum config
- .github/workflows/ci.yml exists with push, pull_request triggers and go test + go vet steps
- .github/workflows/release.yml exists with v* tag trigger, goreleaser action, and GITHUB_TOKEN
- Commits 1636872 and 7b461ab both exist in git log
