---
phase: 13-cli-subcommands-deploy-ux
plan: "08"
subsystem: cli
tags: [version-cmd, bug-fix, ldflags, dev-build]
dependency_graph:
  requires: []
  provides: [correct-version-output-for-dev-builds]
  affects: [cmd/docker-deploy/main.go, cmd/docker-deploy/main_test.go]
tech_stack:
  added: []
  patterns: [conditional-ldflags-discriminator]
key_files:
  modified:
    - cmd/docker-deploy/main.go
    - cmd/docker-deploy/main_test.go
decisions:
  - "Use version != 'dev' (not buildTime != 'unknown') to gate the Built: line, because make build always injects buildTime via ldflags"
metrics:
  duration: "~2 min"
  completed: "2026-05-26T17:19:27Z"
---

# Phase 13 Plan 08: Fix dev-build Built: line suppression Summary

Fix `runVersionTo()` to use `version != "dev"` as the discriminator so dev builds never show the Built: line even when Makefile injects buildTime via ldflags.

## Tasks Completed

| Task | Name | Commit | Files |
|------|------|--------|-------|
| 1 | Fix runVersionTo condition and update tests | 6f36cf1 | cmd/docker-deploy/main.go, cmd/docker-deploy/main_test.go |

## What Was Built

The `runVersionTo()` function previously used `buildTime != "unknown"` to decide whether to print the `Built:` line. Because `make build` always injects `buildTime` via ldflags (`-X main.buildTime=$(shell date ...)`), this condition was always true for local dev builds, leaking the `Built:` line into untagged/dev output in violation of spec D-03.

The fix changes the condition to `version != "dev" && buildTime != "unknown"`, matching the correct discriminator: GoReleaser injects a semver version for tagged releases; `make build` leaves version as `"dev"` for untagged commits.

A new test `TestVersionCmd_DevBuildWithInjectedTime` provides explicit regression coverage for the scenario: `version="dev"` + non-"unknown" buildTime → 3-line output, no `Built:` line.

## Deviations from Plan

None - plan executed exactly as written.

## Verification

All TestVersionCmd_* tests pass:
- TestVersionCmd_Registered: PASS
- TestVersionCmd_DevOutput: PASS (version=dev + buildTime=unknown → no Built:)
- TestVersionCmd_TaggedOutput: PASS (version=v0.6.3 + buildTime populated → Built: present)
- TestVersionCmd_DevBuildWithInjectedTime: PASS (version=dev + injected buildTime → no Built:)
- TestVersionCmd_ExitZero: PASS

`go build ./...` succeeds with no errors.

## Self-Check: PASSED

- [x] cmd/docker-deploy/main.go modified (condition changed, comment updated)
- [x] cmd/docker-deploy/main_test.go modified (new test added)
- [x] Commit 6f36cf1 exists
- [x] `grep -n 'version != "dev"' main.go` returns line 109 (code line, not just comment)
- [x] `grep -n 'TestVersionCmd_DevBuildWithInjectedTime' main_test.go` returns 2 lines (comment + func)
