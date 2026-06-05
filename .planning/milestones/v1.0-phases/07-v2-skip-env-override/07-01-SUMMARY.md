---
phase: 07-v2-skip-env-override
plan: "01"
subsystem: config
tags: [refactor, config, flagopts, skip-env, verbose, excludes]
dependency_graph:
  requires: []
  provides: [config.FlagOpts, config.Config.SkipEnv, config.Config.Verbose, config.TargetConfig.SkipEnv, expanded-defaultExcludes]
  affects: [cmd/docker-deploy/main.go, internal/config/config.go, internal/config/config_test.go]
tech_stack:
  added: []
  patterns: [FlagOpts struct replaces positional params, SkipEnv via mergeExcludes dedup, Verbose as Config carrier field]
key_files:
  created: []
  modified:
    - internal/config/config.go
    - internal/config/config_test.go
    - cmd/docker-deploy/main.go
decisions:
  - "FlagOpts struct with 9 fields replaces 10-positional-param Resolve() signature (D-05)"
  - "SkipEnv appended in mergeExcludes() after dedup loop with same seen-guard (D-08)"
  - "main.go call sites migrated to FlagOpts{} with SkipEnv/Verbose as zero values pending Plan 02"
metrics:
  duration: "15m"
  completed: "2026-05-20"
  tasks_completed: 2
  files_changed: 3
---

# Phase 7 Plan 01: Config Foundation — FlagOpts refactor, expanded excludes, SkipEnv/Verbose Summary

**One-liner:** FlagOpts struct replaces positional Resolve() params; defaultExcludes expanded from 6 to 16 entries; SkipEnv and Verbose fields added to Config/TargetConfig with full test coverage.

## What Was Built

### Task 1: FlagOpts struct, expanded defaultExcludes, Config/TargetConfig extensions

`internal/config/config.go` was refactored with the following changes:

1. **FlagOpts struct** added immediately before `Resolve()` with 9 fields: `Host`, `Path`, `Excludes`, `Force`, `ComposeFile`, `HealthTimeout`, `HealthInterval`, `SkipEnv`, `Verbose`.

2. **defaultExcludes expanded** from 6 to 16 entries by adding:
   `.claude/`, `.github/`, `.planning/`, `.idea/`, `.vscode/`, `*.swp`, `*.swo`, `coverage/`, `dist/`, `.terraform/`

3. **TargetConfig** gained `SkipEnv bool \`yaml:"skip_env"\`` field after HealthInterval.

4. **Config** gained `SkipEnv bool` and `Verbose bool` fields after HealthInterval.

5. **mergeExcludes()** signature extended with `skipEnv bool` parameter; appends `".env"` with dedup guard when true.

6. **Resolve()** signature changed to `func Resolve(opts FlagOpts, file FileConfig, projectName string, localDir string) (Config, error)`. Sets `cfg.SkipEnv = opts.SkipEnv || file.Target.SkipEnv` and `cfg.Verbose = opts.Verbose`.

### Task 2: Updated config tests + new Phase 7 coverage

`internal/config/config_test.go` updated with:

- All existing `Resolve()` call sites migrated to `FlagOpts{}` struct literals.
- `TestResolveExcludes` updated: `builtInPatterns` expanded to 16 entries; `wantLen` updated for all subtests (16/17/18/16/16).
- **TestResolveSkipEnv** added: 5 subtests covering flag activation, file activation, flag-overrides-file, deduplication when `.env` already in user excludes, and no-skip-env absence.
- **TestResolveVerbose** added: `verbose_true` and `verbose_false_default` subtests.
- **TestResolveExpandedDefaults** added: verifies all 10 new dev-tooling entries present in cfg.Excludes.

## Deviations from Plan

### Auto-fixed Issues

**1. [Rule 3 - Blocking Build] Updated main.go call sites to use FlagOpts{}**
- **Found during:** Task 2 verification (`go build ./...`)
- **Issue:** `cmd/docker-deploy/main.go` had two `config.Resolve()` call sites using the old 10-positional-arg signature. The FlagOpts refactor broke the full module build.
- **Fix:** Migrated both `runDryRun` and `runDeploy` call sites to use `config.FlagOpts{}` struct literals with existing fields (Host, Path, Excludes, Force, ComposeFile). SkipEnv and Verbose remain zero values, pending Phase 7 Plan 02 flag registration.
- **Files modified:** `cmd/docker-deploy/main.go`
- **Commit:** 27fc01b (included in Task 2 commit)

## Commits

| Task | Commit | Description |
|------|--------|-------------|
| Task 1 | 8d3f879 | feat(07-01): FlagOpts struct, expanded defaultExcludes, SkipEnv/Verbose fields |
| Task 2 | 27fc01b | feat(07-01): update config tests for FlagOpts + add Phase 7 coverage; fix main.go callers |

## Verification Results

All 8 plan verification checks passed:

1. `grep -c "type FlagOpts struct" internal/config/config.go` → 1
2. `grep -c "\.github/" internal/config/config.go` → 1
3. `grep -c "SkipEnv" internal/config/config.go` → 6 (>= 4)
4. `grep -c "Verbose" internal/config/config.go` → 4 (>= 3)
5. `go build ./internal/config/...` → OK
6. `go test ./internal/config/... -v` → 17 tests, 0 FAIL
7. `go test ./internal/config/... -run TestResolveSkipEnv` → PASS (5 subtests)
8. `go test ./internal/config/... -run TestResolveExpandedDefaults` → PASS

## Known Stubs

None — all new fields are fully wired through `Resolve()`. `SkipEnv` and `Verbose` in `main.go` are zero-valued pending Phase 7 Plan 02 flag registration; this is intentional and documented in code comments.

## Threat Flags

| Flag | File | Description |
|------|------|-------------|
| threat_flag: information_disclosure | internal/config/config.go | skip_env: true in deploy.yaml silently suppresses .env upload; warning print is Plan 02 responsibility per D-09/T-07-01-01 |
