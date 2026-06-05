---
phase: 10-add-phase-autosuggestion
plan: "03"
subsystem: cli
tags: [completion, cobra, shell, bash, zsh]

# Dependency graph
requires:
  - phase: 10-add-phase-autosuggestion
    provides: "internal/completion package with GenerateBash/GenerateZsh wrappers and buildCompletionCmd() wired in main.go"

provides:
  - "Dynamic completion code deleted (completion.go, completion_test.go)"
  - "completion subcommand marked Hidden and DisableFlagsInUseLine per D-02"
  - "completion.Register(cmd) call removed from buildDeployCmd() per D-03"
  - "TestCompletionCmd_Registered enforces Hidden and DisableFlagsInUseLine"

affects:
  - "10-04 (make completions target — uses hidden subcommand)"
  - "10-05 (goreleaser + install script — uses generated contrib/ files)"

# Tech tracking
tech-stack:
  added: []
  patterns:
    - "buildCompletionCmd uses Hidden: true + DisableFlagsInUseLine: true to hide maintenance commands from end-user help"
    - "Dynamic flag completion (RegisterFlagCompletionFunc) replaced by static cobra-generated scripts"

key-files:
  created: []
  modified:
    - "cmd/docker-deploy/main.go"
    - "cmd/docker-deploy/main_test.go"
  deleted:
    - "internal/completion/completion.go"
    - "internal/completion/completion_test.go"

key-decisions:
  - "D-02: Hidden: true + DisableFlagsInUseLine: true on the completion subcommand — keeps it callable by make completions but absent from docker deploy --help"
  - "D-03: All dynamic RegisterFlagCompletionFunc hooks deleted along with completion.go — static scripts are sufficient and simpler"

patterns-established:
  - "Hidden cobra subcommands for release-pipeline tooling — available but not advertised to users"

requirements-completed: []

# Metrics
duration: 8min
completed: 2026-06-02
---

# Phase 10 Plan 03: Static Cobra Completion Rework Summary

**Deleted dynamic completion hooks and completion.go, marked the completion subcommand Hidden+DisableFlagsInUseLine per D-02/D-03 — static bash/zsh scripts still generate correctly**

## Performance

- **Duration:** 8 min
- **Started:** 2026-06-02T00:00:00Z
- **Completed:** 2026-06-02T00:08:00Z
- **Tasks:** 3
- **Files modified:** 2 modified, 2 deleted

## Accomplishments

- Deleted `internal/completion/completion.go` and its test file — removing HostCompletionFunc, PathCompletionFunc, ComposeFileCompletionFunc, dedupStrings, and Register
- Removed `completion.Register(cmd)` from `buildDeployCmd()` and added `Hidden: true` + `DisableFlagsInUseLine: true` to `buildCompletionCmd()`
- Updated `TestCompletionCmd_Registered` to assert both Hidden and DisableFlagsInUseLine fields per D-02
- All tests pass (`go test ./...`), build succeeds (`go build ./...`), vet clean (`go vet ./...`)

## Task Commits

Each task was committed atomically:

1. **Task 1: Delete dynamic completion code and tests** - `e077900` (feat)
2. **Task 2: Remove completion.Register call and hide the completion subcommand** - `eeaf44b` (feat)
3. **Task 3: Update TestCompletionCmd_Registered to assert Hidden==true** - `fd008e0` (test)

**Plan metadata:** (committed below)

## Files Created/Modified

- `cmd/docker-deploy/main.go` - Removed `completion.Register(cmd)` call; added `Hidden: true` and `DisableFlagsInUseLine: true` to `buildCompletionCmd()`; updated doc comment referencing D-02 and D-03
- `cmd/docker-deploy/main_test.go` - Added `sub.Hidden` and `sub.DisableFlagsInUseLine` assertions to `TestCompletionCmd_Registered`
- `internal/completion/completion.go` - DELETED (D-03)
- `internal/completion/completion_test.go` - DELETED (D-03)

## Decisions Made

- D-02 enforced: `Hidden: true` + `DisableFlagsInUseLine: true` — the completion subcommand is a pipeline tool, not a user-facing command
- D-03 enforced: No RegisterFlagCompletionFunc hooks remain; the completion package now contains only bash.go and zsh.go

## Deviations from Plan

None - plan executed exactly as written.

## Issues Encountered

None - `.gitignore` pattern `docker-deploy` overlaps with `cmd/docker-deploy/` directory but files were already tracked by git, so `git commit` worked without re-staging.

## User Setup Required

None - no external service configuration required.

## Next Phase Readiness

- Plan 10-04 (make completions target) can proceed: the hidden `completion bash/zsh` subcommand works correctly and writes to stdout as verified
- `internal/completion/` package is clean: only bash.go, bash_test.go, zsh.go, zsh_test.go remain

---
*Phase: 10-add-phase-autosuggestion*
*Completed: 2026-06-02*
