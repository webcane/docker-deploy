---
phase: 13-cli-subcommands-deploy-ux
plan: 03
subsystem: cli
tags: [cobra, config, validate, deploy.yaml]

requires:
  - phase: 13-cli-subcommands-deploy-ux-plan-01
    provides: config.LoadFile cwd-relative loading pattern established
  - phase: 13-cli-subcommands-deploy-ux-plan-02
    provides: buildVersionCmd pattern for no-arg subcommands with SilenceUsage

provides:
  - buildValidateCmd() *cobra.Command registered on buildDeployCmd()
  - runValidate() validates deploy.yaml without SSH connection

affects: [future subcommands that need local-only config validation]

tech-stack:
  added: []
  patterns:
    - validate subcommand: local-only os.Stat existence check before LoadFile to distinguish missing vs malformed
    - SilenceUsage=true on subcommands that return errors (suppress cobra usage block)
    - Zero FlagOpts{} passed to config.Resolve for subcommands with no CLI flags

key-files:
  created: []
  modified:
    - cmd/docker-deploy/main.go
    - cmd/docker-deploy/main_test.go

key-decisions:
  - "D-07: missing deploy.yaml uses os.Stat before LoadFile to emit exact 'deploy.yaml not found' message to stderr"
  - "D-08: runValidate contains no sshpkg.Dial call — local config-only validation"
  - "D-09: success prints '✓ deploy.yaml is valid' to stdout via fmt.Fprintln"
  - "Pitfall 4: SilenceUsage=true on buildValidateCmd to suppress cobra usage block on error"

patterns-established:
  - "Local-only subcommands: os.Stat existence check + LoadFile + Resolve(FlagOpts{}) pattern"

requirements-completed: [13-03]

duration: 8min
completed: 2026-05-26
---

# Phase 13 Plan 03: Add validate subcommand Summary

**`docker deploy validate` subcommand validates deploy.yaml locally via os.Stat + LoadFile + Resolve without any SSH connection**

## Performance

- **Duration:** 8 min
- **Started:** 2026-05-26T12:15:00Z
- **Completed:** 2026-05-26T12:23:00Z
- **Tasks:** 1 (TDD: 2 commits — test RED + feat GREEN)
- **Files modified:** 2

## Accomplishments

- `buildValidateCmd()` returns cobra.Command registered on `buildDeployCmd()` via `cmd.AddCommand`
- `runValidate()` uses os.Stat to distinguish missing file from malformed YAML, then calls LoadFile + Resolve(FlagOpts{})
- Missing deploy.yaml: prints "deploy.yaml not found" to stderr, returns error (D-07)
- Valid config: prints "✓ deploy.yaml is valid" to stdout (D-09)
- No SSH Dial() call in validate code path (D-08)
- 5 new TestValidateCmd* tests cover registered, valid config, missing file, invalid YAML, and no-SSH structural check

## Task Commits

1. **Task 1 RED: Failing TestValidateCmd* tests** - `7987e25` (test)
2. **Task 1 GREEN: buildValidateCmd + runValidate implementation** - `39672b9` (feat)

**Plan metadata:** (docs commit follows)

_TDD task had 2 commits: test RED then feat GREEN_

## Files Created/Modified

- `cmd/docker-deploy/main.go` - Added buildValidateCmd(), runValidate(), AddCommand registration
- `cmd/docker-deploy/main_test.go` - Added TestValidateCmd_Registered, TestValidateCmd_ValidConfig, TestValidateCmd_MissingFile, TestValidateCmd_InvalidYAML, TestValidateCmd_NoSSH; added io/os/path/filepath imports

## Decisions Made

- Used `os.Stat` check before `config.LoadFile` so missing file produces exact "deploy.yaml not found" message (D-07) rather than the LoadFile zero-value return which would silently proceed
- Passed `config.FlagOpts{}` (zero value) to `config.Resolve` — no CLI flags for validate; file values only
- `SilenceUsage: true` on buildValidateCmd prevents cobra from printing the usage block on any error (Pitfall 4 from RESEARCH.md)
- Tests provide `compose_file` in deploy.yaml for valid-config test to avoid Resolve auto-detect failure in temp dir

## Deviations from Plan

None - plan executed exactly as written.

## Issues Encountered

None.

## User Setup Required

None - no external service configuration required.

## Next Phase Readiness

- validate subcommand complete and tested; ready for plan 04 (next plan in phase 13)
- Full test suite passes: go test ./... exits 0

---
*Phase: 13-cli-subcommands-deploy-ux*
*Completed: 2026-05-26*
