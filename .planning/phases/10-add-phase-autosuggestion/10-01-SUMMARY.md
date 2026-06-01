---
phase: 10-add-phase-autosuggestion
plan: "01"
subsystem: sshconfig
tags: [tdd, completion, sshconfig, enumeration]
dependency_graph:
  requires: []
  provides: [sshconfig.ListHosts]
  affects: [internal/completion/completion.go]
tech_stack:
  added: []
  patterns: [bufio.Scanner SSH config parser, wildcard exclusion via strings.ContainsAny]
key_files:
  created: []
  modified:
    - internal/sshconfig/sshconfig.go
    - internal/sshconfig/sshconfig_test.go
decisions:
  - "ListHosts uses strings.ContainsAny(pattern, \"*?\") for wildcard detection — matches D-03 silent-fail contract"
  - "Returns nil (not empty slice) on all error paths — file open error and scanner error"
  - "No refactor commit needed: implementation is minimal and non-duplicative vs LookupHost"
metrics:
  duration: "~3 min"
  completed: "2026-06-01"
  tasks: 1
  files_modified: 2
---

# Phase 10 Plan 01: sshconfig.ListHosts TDD Summary

**One-liner:** TDD implementation of `sshconfig.ListHosts` — enumerate non-wildcard SSH config Host aliases using bufio.Scanner skeleton from LookupHost.

## Tasks Completed

| Task | Name | Commit | Files |
|------|------|--------|-------|
| RED | Add failing TestListHosts_* tests | 5022181 | internal/sshconfig/sshconfig_test.go |
| GREEN | Implement ListHosts in sshconfig.go | 44ac600 | internal/sshconfig/sshconfig.go |

## What Was Built

`ListHosts(configPath string) []string` added to `internal/sshconfig/sshconfig.go`.

The function:
- Scans Host blocks in the SSH config file using the same `bufio.Scanner` + `strings.Fields` skeleton as `LookupHost`
- Collects all Host pattern values that do NOT contain `*` or `?` (wildcard detection via `strings.ContainsAny`)
- Returns `nil` on file-open error and on `scanner.Err() != nil` (satisfies D-03: silent fail)
- Returns `nil` for empty files (no Host blocks found)
- Preserves file order for aliases

Six test cases cover: happy path (two aliases), wildcard exclusion, multi-pattern Host line (mixed wildcards and real aliases), missing file, empty file, comments and blank lines.

## TDD Gate Compliance

- RED gate: commit `5022181` — `test(10-01):` prefix, 6 failing tests confirming `undefined: ListHosts`
- GREEN gate: commit `44ac600` — `feat(10-01):` prefix, all 6 tests pass
- REFACTOR gate: no changes needed — implementation was already minimal and clean

## Verification Results

```
go test ./internal/sshconfig/... -run TestListHosts -v
# All 6 TestListHosts_* PASS

go test ./internal/sshconfig/...
# ok  github.com/webcane/docker-deploy/internal/sshconfig (no regressions)

grep -c 'func ListHosts' internal/sshconfig/sshconfig.go
# 1
```

## Deviations from Plan

None — plan executed exactly as written.

## Known Stubs

None.

## Threat Flags

No new threat surface introduced. `ListHosts` reads user-owned `~/.ssh/config` via a fixed caller-supplied path — same trust boundary as `LookupHost`. Covered by T-10-01-01 and T-10-01-02 in the plan threat register.

## Self-Check: PASSED

- [x] `internal/sshconfig/sshconfig.go` exists and contains `func ListHosts`
- [x] `internal/sshconfig/sshconfig_test.go` exists and contains `TestListHosts`
- [x] Commit `5022181` (RED) exists
- [x] Commit `44ac600` (GREEN) exists
- [x] All 6 TestListHosts_* pass; no regressions to existing LookupHost tests
