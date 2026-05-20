---
phase: 07-v2-skip-env-override
plan: "02"
subsystem: cmd,filetransfer,compose
tags: [verbose, skip-env, flags, warning-rollup, preflight, upload, runcompose]
dependency_graph:
  requires: [07-01]
  provides: [--skip-env flag, --verbose flag, Upload.verbose, RunCompose.verbose, warning-rollup, preflight-checklist-rendering]
  affects: [cmd/docker-deploy/main.go, internal/filetransfer/upload.go, internal/compose/run.go]
tech_stack:
  added: []
  patterns: [verbose bool propagation through call chain, warning rollup accumulator, buildDeployCmd() extractor for testability]
key_files:
  created:
    - cmd/docker-deploy/main_test.go
  modified:
    - cmd/docker-deploy/main.go
    - internal/filetransfer/upload.go
    - internal/filetransfer/upload_test.go
    - internal/compose/run.go
    - internal/compose/run_test.go
decisions:
  - "buildDeployCmd() extracted from main() to allow cobra flag registration testing without plugin infrastructure"
  - "Warning rollup: []string slice accumulates non-blocking warnings; single rollup line printed when not verbose (D-02)"
  - "sudoRunWithFallback closure captures verbose from outer Upload() scope — no threading required"
  - "Interactive sudo command redacted in verbose output: '[ssh] (sudo password cmd redacted)' (T-07-02-05)"
metrics:
  duration: "7m"
  completed: "2026-05-20"
  tasks_completed: 3
  files_changed: 5
---

# Phase 7 Plan 02: Wire-up — Flags, Upload Verbose, RunCompose Verbose, Warning Rollup Summary

**One-liner:** --skip-env and --verbose flags registered on deploy command; Upload() and RunCompose() gain verbose bool param with per-file/SSH-command stderr logging; warning rollup system and preflight checklist rendering wired into runDeploy().

## What Was Built

### Task 1: Upload() verbose param

`internal/filetransfer/upload.go` modified:

1. **Signature change**: `Upload(..., verbose bool)` — verbose bool added as last parameter.
2. **Per-file output**: `fmt.Fprintf(os.Stderr, "  -> %s\n", relPath)` emitted only when `verbose=true`; suppressed entirely when `verbose=false`. The "Uploading N files..." line stays on `os.Stdout` in both modes.
3. **sshExecVerbose helper**: New package-private function logs `[ssh] <cmd>` before and `-> exit 0` / `-> exit N` after each SSH exec when verbose.
4. **sudoRunWithFallback warnedOnce guard**: Changed from `if !*warnedOnce` to `if verbose || !*warnedOnce`; sets `*warnedOnce = true` only when `!verbose` — every warning prints when verbose=true, first-only when verbose=false.
5. **Interactive sudo redaction**: When verbose=true, prints `[ssh] (sudo password cmd redacted)` instead of the full command containing the literal password (T-07-02-05).
6. **remoteExists logging**: test -d check logged with `[ssh] test -d <path>` + `-> exit 0` when verbose.
7. Updated existing upload_test.go call sites to pass `false` for the new verbose param.

### Task 1b: RunCompose() verbose param

`internal/compose/run.go` modified:

1. **Signature change**: `RunCompose(..., verbose bool)` — verbose bool added as last parameter.
2. **Pre-Start logging**: `[ssh] <cmd>` printed to stderr before `session.Start(cmd)` in both the non-TTY path and the PTY path when verbose=true.
3. **Post-Wait logging**: `-> exit 0` printed after successful `Wait()`; `-> exit N` printed after failed Wait() (where N is extracted via `errors.As(waitErr, &exitErr)`). Both paths (non-TTY and PTY) updated.
4. Updated existing run_test.go call sites to pass `false` for the new verbose param.

### Task 2: main.go — flags, FlagOpts, warning rollup, verbose preflight rendering

`cmd/docker-deploy/main.go` refactored:

1. **buildDeployCmd() extractor**: Cobra command construction extracted from `main()` into `buildDeployCmd()` to allow flag registration testing.
2. **Flag registration**: `--skip-env bool` and `--verbose bool` registered on the deploy command.
3. **Signature updates**: `runDeploy()` and `runDryRun()` gain `skipEnv bool` and `verbose bool` params.
4. **FlagOpts wiring**: Both `Resolve()` call sites updated to pass `SkipEnv: skipEnv` and `Verbose: verbose` in the `config.FlagOpts{}` struct literal.
5. **Warning rollup**: `var warnings []string` accumulates non-blocking warnings. At end of `runDeploy()`, prints `WARN: there are some warnings during deployment. For more details use --verbose flag` when `len(warnings) > 0 && !resolved.Verbose`.
6. **Skip-env warning**: Inline print to stderr when verbose; rollup accumulation when not verbose. Copy: `WARNING: .env not uploaded — remote .env left unchanged`.
7. **Preflight checklist rendering**: When verbose, iterates `results []preflight.CheckResult` and prints `  [STATUS] name: message` to stderr. When not verbose, accumulates warn-status results into `warnings`.
8. **Upload() and RunCompose() call sites**: Both receive `resolved.Verbose` as the verbose arg.

## Deviations from Plan

### Auto-fixed Issues

**1. [Rule 3 - Blocking Build] Patched Upload() and RunCompose() call sites in main.go during Task 1b GREEN phase**
- **Found during:** Task 1b verification (`go build ./...`)
- **Issue:** After adding verbose param to Upload() (Task 1 GREEN), the `go build ./...` verification for Task 1b failed because main.go still had the old signatures for both Upload() and RunCompose().
- **Fix:** Applied the Upload() and RunCompose() call site patches in main.go ahead of Task 2 (with zero values for verbose, then properly wired in Task 2). This is a standard build-fix deviation.
- **Files modified:** `cmd/docker-deploy/main.go`
- **Commit:** b267545 (included in Task 1b commit)

## TDD Gate Compliance

All three tasks followed RED/GREEN pattern:
- Task 1: RED commit 016b819 (failing tests) -> GREEN commit 49877c2 (implementation)
- Task 1b: RED commit 6cadd94 (failing tests) -> GREEN commit b267545 (implementation)
- Task 2: RED commit 9a26931 (failing tests) -> GREEN commit e168455 (implementation)

## Commits

| Task | Commit | Description |
|------|--------|-------------|
| Task 1 RED | 016b819 | test(07-02): add failing tests for Upload() verbose param |
| Task 1 GREEN | 49877c2 | feat(07-02): Upload() verbose param — per-file stderr, SSH command logging, warnedOnce conditioned on verbose |
| Task 1b RED | 6cadd94 | test(07-02): add failing tests for RunCompose() verbose param |
| Task 1b GREEN | b267545 | feat(07-02): RunCompose() verbose param — SSH command and exit code logged to stderr |
| Task 2 RED | 9a26931 | test(07-02): add failing tests for --skip-env and --verbose flag registration |
| Task 2 GREEN | e168455 | feat(07-02): main.go — --skip-env and --verbose flags, FlagOpts wiring, warning rollup, verbose preflight rendering |

## Verification Results (automated)

1. `grep -c "skip-env" cmd/docker-deploy/main.go` -> 1 (flag registered)
2. `grep -c "verbose" cmd/docker-deploy/main.go` -> 15 (flag registered + wired throughout)
3. `grep -c "FlagOpts{" cmd/docker-deploy/main.go` -> 2 (both Resolve() call sites updated)
4. Upload() signature contains `verbose bool` — PASS
5. `grep -c "warnings" cmd/docker-deploy/main.go` -> 9 (slice declared, appended, printed)
6. `grep -c "WARN: there are some warnings" cmd/docker-deploy/main.go` -> 1 (rollup message)
7. `grep -c "WARNING: .env not uploaded" cmd/docker-deploy/main.go` -> 1 (skip-env warning copy)
8. RunCompose() signature contains `verbose bool` — PASS
9. `grep -v '^//' internal/compose/run.go | grep -c "\[ssh\]"` -> 2 (both TTY and non-TTY paths)
10. `grep -v '^//' internal/filetransfer/upload.go | grep -c "\[ssh\]"` -> 5 (verbose logging calls)
11. `go build ./...` -> PASS
12. `go test ./...` -> all packages PASS

## Known Stubs

None — all new fields and parameters are fully wired through the call chain.

## Threat Flags

None new beyond what was planned in the PLAN.md threat model.

## Self-Check: PASSED

All created files confirmed present. All 6 task commits confirmed in git log.

## Checkpoint Status

Stopped at Task 3 (checkpoint:human-verify) as required. Tasks 1, 1b, and 2 are complete with all commits made. Awaiting human verification before plan is marked complete.
