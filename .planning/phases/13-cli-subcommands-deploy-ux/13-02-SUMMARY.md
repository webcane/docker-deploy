---
phase: 13-cli-subcommands-deploy-ux
plan: "02"
subsystem: cmd
tags: [version, ldflags, build-metadata, cobra]
dependency_graph:
  requires: []
  provides: [version-subcommand, build-metadata-vars]
  affects: [cmd/docker-deploy/main.go, Makefile, .goreleaser.yaml]
tech_stack:
  added: [runtime (stdlib)]
  patterns: [ldflag-injection, cobra-subcommand, writer-injection-for-testability]
key_files:
  modified:
    - cmd/docker-deploy/main.go
    - cmd/docker-deploy/main_test.go
    - Makefile
    - .goreleaser.yaml
decisions:
  - runVersionTo(w io.Writer) extracted for testability; runVersion() wraps it writing to os.Stdout
  - buildTime check (!=unknown) distinguishes tagged vs dev builds; no additional sentinel needed
  - All three ldflags vars (version, gitCommit, buildTime) injected in both Makefile and GoReleaser
metrics:
  duration: ~8min
  completed: "2026-05-26"
  tasks_completed: 2
  files_modified: 4
---

# Phase 13 Plan 02: Version Subcommand & Build Metadata Summary

`docker deploy version` subcommand printing version, git commit, OS/arch, and build timestamp (tagged builds only) via ldflags injection in Makefile and GoReleaser.

## Tasks Completed

| Task | Name | Commit | Files |
|------|------|--------|-------|
| 1 (RED) | Failing TestVersionCmd tests | 1cc9565 | cmd/docker-deploy/main_test.go |
| 1 (GREEN) | buildVersionCmd + runVersionTo + ldflags vars | bb9ecea | cmd/docker-deploy/main.go |
| 2 | Extend Makefile + GoReleaser ldflags | 1428d12 | Makefile, .goreleaser.yaml |

## What Was Built

- `var gitCommit = "unknown"` and `var buildTime = "unknown"` added alongside existing `var version = "dev"` in main.go — these are the ldflag injection targets
- `buildVersionCmd()` returns a cobra.Command with Use="version" registered via `cmd.AddCommand()` in `buildDeployCmd()`
- `runVersionTo(w io.Writer)` writes 3-line (dev) or 4-line (tagged) version output; `runVersion()` calls it with os.Stdout
- Tagged output includes "Built:" line when buildTime != "unknown"; dev output omits it
- OS/Arch derived at runtime via `runtime.GOOS + "/" + runtime.GOARCH`
- Makefile build target extended with `-X main.gitCommit=$(shell git rev-parse --short HEAD)` and `-X main.buildTime=$(shell date -u +%FT%TZ)`
- GoReleaser ldflags extended with `-X main.gitCommit={{.ShortCommit}}` and `-X main.buildTime={{.Date}}`

## Verification Results

- `make build` exits 0, injects git commit and build timestamp
- `docker deploy version` prints 4-line output (buildTime injected): version, git commit, built timestamp, OS/arch
- `go test ./cmd/docker-deploy/... -run TestVersionCmd -v`: all 4 tests PASS
- `go test ./...`: all packages pass, no regressions
- `grep -c "gitCommit" .goreleaser.yaml` returns 1
- `grep -c "ShortCommit" .goreleaser.yaml` returns 1

## Deviations from Plan

None — plan executed exactly as written.

## Known Stubs

None.

## Threat Flags

No new security-relevant surface introduced. Version output contains only build-time constants and stdlib runtime values — no user input, no credentials.

## TDD Gate Compliance

- RED gate: commit `1cc9565` — `test(13-02): add failing tests for version subcommand`
- GREEN gate: commit `bb9ecea` — `feat(13-02): add version subcommand with ldflags vars`
- All gates satisfied.

## Self-Check: PASSED

- cmd/docker-deploy/main.go: modified (buildVersionCmd, runVersionTo, gitCommit, buildTime vars)
- cmd/docker-deploy/main_test.go: modified (4 TestVersionCmd tests added)
- Makefile: modified (extended build target with gitCommit and buildTime ldflags)
- .goreleaser.yaml: modified (ShortCommit and Date ldflags appended)
- Commits 1cc9565, bb9ecea, 1428d12: all present in git log
