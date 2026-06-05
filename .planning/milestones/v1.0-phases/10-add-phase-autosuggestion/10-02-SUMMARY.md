---
phase: 10-add-phase-autosuggestion
plan: "02"
subsystem: completion
tags: [tdd, completion, shell, bash, zsh, dynamic-completion]
dependency_graph:
  requires: [sshconfig.ListHosts, config.LoadFile]
  provides: [completion.Register, completion.GenerateBash, completion.GenerateZsh, completion.HostCompletionFunc, completion.PathCompletionFunc, completion.ComposeFileCompletionFunc, buildCompletionCmd]
  affects: [cmd/docker-deploy/main.go]
tech_stack:
  added: []
  patterns: [cobra RegisterFlagCompletionFunc, cobra GenBashCompletionV2, cobra GenZshCompletion, internal package separation per D-08]
key_files:
  created:
    - internal/completion/completion.go
    - internal/completion/bash.go
    - internal/completion/zsh.go
    - internal/completion/completion_test.go
    - internal/completion/bash_test.go
    - internal/completion/zsh_test.go
  modified:
    - cmd/docker-deploy/main.go
    - cmd/docker-deploy/main_test.go
decisions:
  - "Test assertions use cmd.GetFlagCompletionFunc() rather than flag.Annotations — cobra v1.10.2 stores completion functions in a global mutex-protected map, not as flag annotations"
  - "buildCompletionCmd() uses cobra.ExactValidArgs(1) + ValidArgs=[bash,zsh] to reject unsupported shells before RunE fires (T-10-02-04)"
  - "completion.Register(cmd) placed after all cmd.Flags().*Var() calls to satisfy RegisterFlagCompletionFunc precondition"
metrics:
  duration: "~8 min"
  completed: "2026-06-01"
  tasks: 2
  files_modified: 8
---

# Phase 10 Plan 02: internal/completion Package + main.go Wiring Summary

**One-liner:** TDD implementation of `internal/completion/` package (bash/zsh generation, dynamic flag completions) wired into main.go via thin `buildCompletionCmd()` factory and `completion.Register(cmd)` call per D-08.

## Tasks Completed

| Task | Name | Commits | Files |
|------|------|---------|-------|
| 1 RED | Add failing tests for internal/completion | a824a7a | completion_test.go, bash_test.go, zsh_test.go |
| 1 GREEN | Implement internal/completion package | 650c503 | completion.go, bash.go, zsh.go |
| 2 RED | Add failing TestCompletionCmd_Registered | b6f7672 | cmd/docker-deploy/main_test.go |
| 2 GREEN | Wire buildCompletionCmd() + completion.Register() | 9654d63 | cmd/docker-deploy/main.go |

## What Was Built

### internal/completion/ package

**completion.go** — All dynamic completion logic per D-08:
- `Register(cmd *cobra.Command)` — wires three `RegisterFlagCompletionFunc` registrations
- `HostCompletionFunc` — merges `deploy.yaml` host value + `~/.ssh/config` aliases, deduplicated
- `PathCompletionFunc` — returns `/opt/<cwd-basename>` (D-06)
- `ComposeFileCompletionFunc` — suggests `compose.yaml`/`docker-compose.yml` when present in cwd (D-07)
- `dedupStrings` — order-preserving deduplication helper

**bash.go** — `GenerateBash(cmd, w)` wrapping `cmd.Root().GenBashCompletionV2(w, false)`

**zsh.go** — `GenerateZsh(cmd, w)` wrapping `cmd.Root().GenZshCompletion(w)`

### cmd/docker-deploy/main.go additions

- `buildCompletionCmd()` — thin factory with `Use: "completion [bash|zsh]"`, `cobra.ExactValidArgs(1)`, delegates to `completion.GenerateBash` / `completion.GenerateZsh`
- `completion.Register(cmd)` call after all flag definitions
- `cmd.AddCommand(buildCompletionCmd())` alongside existing subcommands

## TDD Gate Compliance

- Task 1 RED gate: commit `a824a7a` — `test(10-02):` prefix, 9 tests confirming build failure (no production files)
- Task 1 GREEN gate: commit `650c503` — `feat(10-02):` prefix, all 9 tests pass
- Task 2 RED gate: commit `b6f7672` — `test(10-02):` prefix, 1 failing test
- Task 2 GREEN gate: commit `9654d63` — `feat(10-02):` prefix, all tests pass

## Deviations from Plan

### Auto-fixed Issues

**1. [Rule 1 - Bug] Test assertion strategy for RegisterFlagCompletionFunc**
- **Found during:** Task 1 GREEN phase
- **Issue:** Plan specified asserting `f.Annotations` is non-nil after `Register()`. cobra v1.10.2 stores completion functions in a package-level global map (`flagCompletionFunctions[flag] = f`) — NOT as flag annotations. The `Annotations` map on the flag remains nil.
- **Fix:** Changed test assertions to use `cmd.GetFlagCompletionFunc("flagname")` which queries the actual storage mechanism cobra uses.
- **Files modified:** `internal/completion/completion_test.go`
- **Commit:** 650c503

## Verification Results

```
go test ./internal/completion/... -v
# 9 tests: TestRegister_{Host,Path,ComposeFile}FlagAnnotation,
#   TestHostCompletionFunc_SilentOnMissingFiles, TestPathCompletionFunc_ReturnsPrefixOptSlash,
#   TestComposeFileCompletionFunc_{Empty,Suggests}, TestGenerateBash_OutputContainsBashHeader,
#   TestGenerateZsh_OutputContainsCompdef — all PASS

go test ./...
# ok  github.com/webcane/docker-deploy/cmd/docker-deploy
# ok  github.com/webcane/docker-deploy/internal/completion
# ok  github.com/webcane/docker-deploy/internal/... (all packages)

go build ./cmd/docker-deploy/...
# exit 0

grep -c 'func buildCompletionCmd' cmd/docker-deploy/main.go
# 1

grep -c 'completion.Register' cmd/docker-deploy/main.go
# 1

grep -v '^[[:space:]]*//' cmd/docker-deploy/main.go | grep -c 'RegisterFlagCompletionFunc'
# 0

go run ./cmd/docker-deploy/... deploy completion --help
# Usage:  docker deploy completion [bash|zsh]
# Generate shell completion script
```

## Known Stubs

None — all completion functions are fully implemented and wired.

## Threat Flags

No new threat surface introduced beyond what the plan's threat model covers. No new network endpoints, auth paths, or schema changes.

## Self-Check: PASSED

- [x] `internal/completion/completion.go` exists, contains `func Register`, `func HostCompletionFunc`, `func PathCompletionFunc`, `func ComposeFileCompletionFunc`
- [x] `internal/completion/bash.go` exists, contains `func GenerateBash`
- [x] `internal/completion/zsh.go` exists, contains `func GenerateZsh`
- [x] `internal/completion/completion_test.go`, `bash_test.go`, `zsh_test.go` exist with all 9 tests
- [x] `cmd/docker-deploy/main.go` contains `func buildCompletionCmd` and `completion.Register`
- [x] `cmd/docker-deploy/main_test.go` contains `TestCompletionCmd_Registered`
- [x] Commit `a824a7a` (Task 1 RED) exists
- [x] Commit `650c503` (Task 1 GREEN) exists
- [x] Commit `b6f7672` (Task 2 RED) exists
- [x] Commit `9654d63` (Task 2 GREEN) exists
- [x] `go test ./...` exits 0
- [x] `go build ./cmd/docker-deploy/...` exits 0
