---
phase: 03-file-copy
plan: "01"
subsystem: config
tags: [go, config, excludes, force, precedence, tdd]

# Dependency graph
requires:
  - phase: 02-ssh-transport-config
    provides: Config struct with Host/Path/DryRun and Resolve() function

provides:
  - Config.Excludes []string — merged defaultExcludes + file + flag with dedup
  - Config.Force bool — flag > file > false precedence
  - TargetConfig.Exclude []string and Force bool yaml-tagged fields
  - defaultExcludes package-level var (6 built-in patterns)
  - Updated Resolve() signature accepting flagExcludes and flagForce

affects: [03-02-sftp-package, 03-03-deploy-wiring, 04-core-deploy-loop]

# Tech tracking
tech-stack:
  added: []
  patterns:
    - "mergeExcludes helper: start with built-ins, append file, append flags, dedup by string equality preserving insertion order"
    - "Force precedence: flagForce || file.Target.Force (no switch needed, boolean OR is sufficient)"

key-files:
  created: []
  modified:
    - internal/config/config.go
    - internal/config/config_test.go
    - cmd/docker-deploy/main.go

key-decisions:
  - "Resolve() signature extended to 6 params; main.go wired with nil/false placeholders pending Phase 3 flag wiring"
  - "mergeExcludes uses a seen-map for O(n) deduplication, preserving insertion order"

patterns-established:
  - "Exclude merge order: defaultExcludes -> file.Target.Exclude -> flagExcludes (dedup, later duplicates dropped)"
  - "Built-in defaults cannot be removed by user input — append-only model enforced in mergeExcludes"

requirements-completed: [FILES-03]

# Metrics
duration: 3min
completed: 2026-05-14
---

# Phase 3 Plan 01: Config Extension Summary

**Config.Excludes and Config.Force added to Resolve() with defaultExcludes (6 built-ins), file-then-flag merge order, and deduplication by string equality**

## Performance

- **Duration:** 3 min
- **Started:** 2026-05-14T16:32:50Z
- **Completed:** 2026-05-14T16:35:49Z
- **Tasks:** 2
- **Files modified:** 3

## Accomplishments

- Extended TargetConfig with Exclude []string (yaml:"exclude") and Force bool (yaml:"force") fields
- Extended Config with Excludes []string and Force bool, populated by updated Resolve()
- mergeExcludes() implements the 3-tier merge: defaultExcludes + file + flag with O(n) dedup
- 9 new test cases (5 Excludes + 4 Force) via TestResolveExcludes and TestResolveForce
- Updated all existing Resolve() call sites (config_test.go x3, main.go x1) to new 6-arg signature

## Task Commits

Each task was committed atomically:

1. **Task 1: Extend config structs and Resolve() signature** - `1da471f` (feat)
2. **Task 2: Table-driven tests for Excludes and Force precedence** - `3f491c7` (test)

**Plan metadata:** (pending docs commit)

## Files Created/Modified

- `internal/config/config.go` - Added defaultExcludes, extended structs, updated Resolve(), added mergeExcludes()
- `internal/config/config_test.go` - Updated 3 existing Resolve() call sites; added TestResolveExcludes (5 cases) and TestResolveForce (4 cases)
- `cmd/docker-deploy/main.go` - Updated Resolve() call to 6-arg signature with nil/false placeholders

## Decisions Made

- Resolve() call in main.go wired with `nil, false` for the new excludes/force params — actual flag wiring will be added in plan 03-03 when the cobra flags are defined. A TODO comment marks the location.
- mergeExcludes() is a private helper rather than an exported function — callers only interact with Resolve() output.

## Deviations from Plan

### Auto-fixed Issues

**1. [Rule 3 - Blocking] Updated existing Resolve() call sites as part of Task 1**
- **Found during:** Task 1 verification (go vet)
- **Issue:** Changing Resolve() to 6 args broke existing call sites in config_test.go and main.go, preventing compilation of the test binary
- **Fix:** Updated 3 call sites in config_test.go and 1 in main.go to pass nil/false for the new params; this is explicitly called out in the plan's Task 2 action but was required earlier to allow Task 1 verification to pass
- **Files modified:** internal/config/config_test.go, cmd/docker-deploy/main.go
- **Verification:** go build ./... passes, all pre-existing tests pass
- **Committed in:** 1da471f (Task 1 commit)

---

**Total deviations:** 1 auto-fixed (blocking - call site updates moved to Task 1 from Task 2)
**Impact on plan:** No scope change. The call site updates are explicitly specified in Task 2's action; they were applied in Task 1 to satisfy the verify step. Task 2 then added only the new test functions as planned.

## Issues Encountered

None beyond the blocking deviation documented above.

## User Setup Required

None - no external service configuration required.

## Next Phase Readiness

- Config contract is stable: downstream plans 03-02 (sftp package) and 03-03 (deploy wiring) can reference Config.Excludes and Config.Force
- main.go has a TODO comment marking where --exclude and --force flags need to be wired (plan 03-03)
- All 24 config tests pass; no blockers for next plan

---
*Phase: 03-file-copy*
*Completed: 2026-05-14*
