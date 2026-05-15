---
phase: "04"
plan: "01"
subsystem: config
tags: [config, tdd, compose-file, resolve]
dependency_graph:
  requires: []
  provides: [Config.ComposeFile, TargetConfig.ComposeFile, Resolve-8-arg-signature]
  affects: [cmd/docker-deploy/main.go (broken until Plan 03), internal/config/config.go]
tech_stack:
  added: []
  patterns: [flag > deploy.yaml > auto-detect precedence, os.Stat for auto-detection]
key_files:
  created: []
  modified:
    - internal/config/config.go
    - internal/config/config_test.go
decisions:
  - "Resolve() accepts flagComposeFile and localDir as two new params after flagForce, before file FileConfig (matches plan interface spec)"
  - "Auto-detect checks compose.yaml first, then docker-compose.yml; returns named error if neither found"
  - "ComposeFile stored as supplied basename; Plan 02 is responsible for filepath.Base() validation before constructing remote commands (T-04-01-01)"
  - "Existing tests updated to pass 'compose.yaml' as flagComposeFile to avoid auto-detect side effects in non-compose-file tests"
metrics:
  duration: "~8 min"
  completed: "2026-05-15"
  tasks_completed: 1
  files_modified: 2
---

# Phase 4 Plan 01: Config ComposeFile Resolution Summary

Config package extended with ComposeFile field and updated Resolve() signature supporting flag > deploy.yaml > auto-detect priority for compose file selection.

## What Was Built

Extended `internal/config` package with compose file resolution:

- `Config.ComposeFile string` — resolved basename of the compose file (e.g. "compose.yaml")
- `TargetConfig.ComposeFile string` with `yaml:"compose_file"` — deploy.yaml key
- `Resolve()` updated from 6 to 8 args: added `flagComposeFile string` (after `flagForce`) and `localDir string` (after `projectName`)
- Auto-detection via `os.Stat` checks `compose.yaml` then `docker-compose.yml` in `localDir`
- Error `"no compose file found; use --compose-file to specify one"` when all three resolution methods yield nothing

## TDD Gate Compliance

RED commit: `029c2f4` — `test(04-01): add failing tests for compose file resolution`
GREEN commit: `9fc8e68` — `feat(04-01): extend Config with ComposeFile; update Resolve() signature`

Both gate commits present in correct order. All 6 new `TestResolveComposeFile_*` tests pass. All pre-existing tests also pass with updated 8-arg call sites.

## Deviations from Plan

**1. [Rule 1 - Existing tests updated] Updated 5 pre-existing Resolve() call sites to new signature**

- Found during: GREEN phase
- Issue: Pre-existing tests called Resolve() with the old 6-arg signature; updating Resolve() broke them
- Fix: Updated each old-style call to pass `"compose.yaml"` as flagComposeFile (bypassing auto-detect) and `""` as localDir — semantically equivalent for those tests since they don't exercise ComposeFile behavior
- Files modified: internal/config/config_test.go (lines 121, 178, 190, 282, 338)
- Commit: 9fc8e68

Otherwise plan executed exactly as written. The planned main.go breakage is intentional and expected — fix is Plan 03's responsibility.

## Known Stubs

None. ComposeFile resolution is complete. main.go is intentionally broken (tracked as Plan 03 work).

## Threat Flags

T-04-01-01 (Tampering — ComposeFile value from deploy.yaml) is partially mitigated: ComposeFile is stored as supplied without normalization. Plan 02 is responsible for `filepath.Base(ComposeFile) == ComposeFile` validation before constructing remote shell commands. No additional threat surface introduced beyond what was modeled.

## Self-Check: PASSED

- internal/config/config.go: FOUND
- internal/config/config_test.go: FOUND
- Commit 029c2f4 (RED): FOUND
- Commit 9fc8e68 (GREEN): FOUND
