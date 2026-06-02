---
phase: 10-add-phase-autosuggestion
plan: "04"
subsystem: cli
tags: [completion, makefile, release-tooling, shell]

# Dependency graph
requires:
  - phase: 10-add-phase-autosuggestion
    plan: "03"
    provides: "hidden completion subcommand in buildCompletionCmd()"

provides:
  - "completions Makefile target that regenerates contrib/_docker-deploy and contrib/docker-deploy.bash"
  - "contrib/_docker-deploy: static zsh completion script (212 lines, #compdef docker-deploy)"
  - "contrib/docker-deploy.bash: static bash V2 completion script (426 lines)"
  - "contrib/install-completions.sh: POSIX install helper for non-Homebrew users"
  - "buildStandaloneRootForCompletion() generates cobra root named 'docker-deploy' (not docker plugin root)"

affects:
  - "10-05 (goreleaser extra_files — contrib/ files committed and ready)"

# Tech tracking
tech-stack:
  added: []
  patterns:
    - "Makefile completions target depends on build target, uses 'make dependency' pattern"
    - "Standalone cobra root command built from deploy parent flags for correct completion command name"
    - "POSIX install script uses raw.githubusercontent.com for direct file download (no tarball extraction)"

key-files:
  created:
    - "contrib/_docker-deploy"
    - "contrib/docker-deploy.bash"
    - "contrib/install-completions.sh"
  modified:
    - "Makefile"
    - "cmd/docker-deploy/main.go"
    - "internal/completion/bash.go"
    - "internal/completion/zsh.go"

key-decisions:
  - "Rule 1 fix: Use './bin/docker-deploy deploy completion zsh' not './bin/docker-deploy completion zsh' — plugin.Run intercepts 'completion' at docker root level, routing to docker's own completion command"
  - "buildStandaloneRootForCompletion() creates a cobra.Command named 'docker-deploy' from deploy parent flags so generated scripts have correct headers (#compdef docker-deploy)"
  - "INSTALL_VERSION env override defaults to 'latest' (resolved via 'main' git ref for raw download)"

# Metrics
duration: 10min
completed: 2026-06-02
---

# Phase 10 Plan 04: Completions Tooling and Generated Artifacts Summary

**Added make completions target, generated and committed static cobra bash/zsh scripts, and created a POSIX install helper — with a required fix to the completion invocation path and standalone root command naming**

## Performance

- **Duration:** 10 min
- **Started:** 2026-06-02T09:40:00Z
- **Completed:** 2026-06-02T09:50:00Z
- **Tasks:** 3
- **Files created:** 3 (contrib/_docker-deploy, contrib/docker-deploy.bash, contrib/install-completions.sh)
- **Files modified:** 4 (Makefile, main.go, bash.go, zsh.go)

## Accomplishments

- Added `completions` target to Makefile (depends on `build`, runs `deploy completion zsh/bash`, writes to `contrib/`)
- Fixed completion generation to produce `docker-deploy`-named scripts (not docker plugin root)
- Generated and committed `contrib/_docker-deploy` (212 lines, `#compdef docker-deploy`) and `contrib/docker-deploy.bash` (426 lines, `bash completion V2 for docker-deploy`)
- Verified idempotency: re-running `make completions` produces byte-identical files
- Created `contrib/install-completions.sh`: POSIX shell script, auto-detects shell, handles Homebrew + fallback paths, supports `INSTALL_VERSION` override

## Task Commits

Each task was committed atomically:

1. **Task 1: Add completions target to Makefile** - `1cbf520` (feat) — includes Rule 1 fix to completion generation
2. **Task 2: Generate and commit contrib/_docker-deploy and contrib/docker-deploy.bash** - `060c421` (feat)
3. **Task 3: Create contrib/install-completions.sh** - `c167a97` (feat)

## Files Created/Modified

- `Makefile` - Added `completions` to .PHONY; added `completions: build` target with correct invocation
- `cmd/docker-deploy/main.go` - Added `buildStandaloneRootForCompletion()` helper; updated `buildCompletionCmd()` RunE to build standalone root; added `pflag` import
- `internal/completion/bash.go` - Updated `GenerateBash` to accept root command directly (not cmd.Root())
- `internal/completion/zsh.go` - Updated `GenerateZsh` to accept root command directly (not cmd.Root())
- `contrib/_docker-deploy` - CREATED: 212-line zsh completion script
- `contrib/docker-deploy.bash` - CREATED: 426-line bash V2 completion script
- `contrib/install-completions.sh` - CREATED: 121-line POSIX install helper (executable)

## Decisions Made

- Correct invocation for `make completions` is `./bin/docker-deploy deploy completion zsh` (the `deploy` subcommand prefix is required because `plugin.Run()` routes bare `completion` args to Docker's own built-in completion command, not the hidden completion subcommand)
- Completion scripts target standalone `docker-deploy` invocation (direct binary, not via `docker CLI`) — this matches the Homebrew `_docker-deploy` naming convention where `_docker-deploy` completes the `docker-deploy` command

## Deviations from Plan

### Auto-fixed Issues

**1. [Rule 1 - Bug] Completion invocation and command name mismatch**
- **Found during:** Task 1 investigation
- **Issue:** Plan specified `./bin/docker-deploy completion zsh` but `plugin.Run()` intercepts `completion` at the docker root level, routing to Docker's built-in completion system for the full docker CLI (not our hidden completion subcommand). The generated scripts also named `docker` not `docker-deploy` because `cmd.Root()` was the docker plugin root.
- **Fix:** (a) Changed Makefile target to use `./bin/docker-deploy deploy completion zsh` (the deploy subcommand prefix routes correctly). (b) Added `buildStandaloneRootForCompletion()` in main.go that creates a cobra.Command named `docker-deploy` from the deploy parent's flags — this ensures generated scripts use `docker-deploy` as the command name. (c) Updated `internal/completion/bash.go` and `internal/completion/zsh.go` to accept the root command directly rather than calling `cmd.Root()` internally.
- **Files modified:** Makefile, cmd/docker-deploy/main.go, internal/completion/bash.go, internal/completion/zsh.go
- **Commit:** 1cbf520

## Known Stubs

None — all completion content is cobra-generated from actual command tree; no hardcoded placeholder values.

## Threat Flags

None — new files are static scripts committed to repo; no new network endpoints, auth paths, or trust boundary changes introduced.

## Self-Check: PASSED

- [x] `contrib/_docker-deploy` exists and starts with `#compdef docker-deploy`
- [x] `contrib/docker-deploy.bash` exists and mentions `bash completion V2 for docker-deploy`
- [x] `contrib/install-completions.sh` exists, is executable, passes `sh -n`
- [x] `make completions` is idempotent (`git diff --exit-code contrib/` exits 0 after re-run)
- [x] All 3 task commits present: 1cbf520, 060c421, c167a97
- [x] All tests pass (`go test ./...`)
