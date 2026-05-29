---
phase: 14-ssh-config-host-alias-resolution
plan: "01"
subsystem: sshconfig, config
tags: [ssh-config, alias-resolution, host-parsing, tdd]
dependency_graph:
  requires: []
  provides:
    - sshconfig.HostEntry struct
    - sshconfig.LookupHost() exported function
    - config.resolveHostString() helper
    - config.Resolve() alias detection path
  affects:
    - internal/sshconfig/sshconfig.go
    - internal/config/config.go
tech_stack:
  added: []
  patterns:
    - TDD RED/GREEN with per-phase commits
    - Thin-wrapper refactor (LoadSigners delegates to LookupHost)
    - Synthetic ssh:// URL construction from HostEntry fields
key_files:
  created:
    - internal/sshconfig/sshconfig_test.go
  modified:
    - internal/sshconfig/sshconfig.go
    - internal/config/config.go
    - internal/config/config_test.go
decisions:
  - D-01/D-02: bare host string (no ssh:// prefix) treated as alias; applies to both --host flag and deploy.yaml target.host
  - D-05/D-06: HostEntry struct with HostName/User/Port/IdentityFiles exported from sshconfig package
  - D-07: HostName fallback to alias label when HostName directive absent (matches OpenSSH)
  - D-08/D-09: Port=0 and User="" mean not-set; callers apply defaults (22 and OS user)
  - D-10: parseIdentityFiles removed; LoadSigners is thin wrapper over LookupHost
  - D-11: Include directives silently skipped with TODO comment
  - D-12: HostEntry.HostName (real hostname) used as Hostname for known_hosts verification, not alias label
metrics:
  duration: ~8 min
  completed: "2026-05-29"
  tasks_completed: 2
  files_modified: 4
---

# Phase 14 Plan 01: SSH Config Host Alias Resolution Summary

**One-liner:** HostEntry struct + LookupHost() parse ~/.ssh/config aliases to real HostName/User/Port; config.Resolve() detects bare aliases and routes through LookupHost before ParseHost.

## Tasks Completed

| Task | Name | Commit | Files |
|------|------|--------|-------|
| 1 RED | Failing tests for LookupHost and HostEntry | 84d3093 | internal/sshconfig/sshconfig_test.go |
| 1 GREEN | Add HostEntry + LookupHost; refactor LoadSigners | 33e9abf | internal/sshconfig/sshconfig.go |
| 2 RED | Failing alias resolution tests for config | 86bff51 | internal/config/config_test.go |
| 2 GREEN | Wire alias detection into Resolve(); add sshconfig import | 64348cc | internal/config/config.go |

## What Was Built

**internal/sshconfig/sshconfig.go:**
- `HostEntry` struct (HostName, User, Port, IdentityFiles) exported per D-06
- `LookupHost(configPath, alias string) (HostEntry, bool)` — line-by-line ssh_config parser that collects all four directives in a single pass; implements D-07 (alias fallback), D-08 (Port=0), D-09 (User=""), D-11 (Include skipped)
- `LoadSigners()` refactored as thin wrapper over `LookupHost()` per D-10; `parseIdentityFiles()` removed entirely

**internal/config/config.go:**
- `sshConfigPath()` private helper returning `~/.ssh/config` path with fallback
- `resolveHostString(raw, configPath string) (Host, error)` — routes `ssh://` URLs to `ParseHost`, bare aliases to `sshconfig.LookupHost` then builds synthetic `ssh://[user@]hostname[:port]` for `ParseHost`; unmatched alias returns `alias %q not found in <path>`
- `Resolve()` host switch block updated to call `resolveHostString` for both `opts.Host` and `file.Target.Host` cases

## Verification Results

1. `go test ./internal/sshconfig/... -run TestLookupHost` — all 5 tests PASS
2. `go test ./internal/config/... -run "TestResolve_Alias|TestResolve_SSH"` — all 3 tests PASS
3. `go build ./...` — exits 0, no import cycles or compile errors
4. No `InsecureIgnoreHostKey` in production code (only test-only with nolint comments)
5. `LookupHost` exported in sshconfig.go — confirmed
6. `parseIdentityFiles` removed — confirmed

## TDD Gate Compliance

**Task 1:**
- RED: commit 84d3093 — `test(14-01): add failing tests for LookupHost and HostEntry`
- GREEN: commit 33e9abf — `feat(14-01): add HostEntry struct and LookupHost(); refactor LoadSigners()`

**Task 2:**
- RED: commit 86bff51 — `test(14-01): add failing alias resolution tests for config.Resolve()`
- GREEN: commit 64348cc — `feat(14-01): wire alias detection into config.Resolve(); add sshconfig import`

## Deviations from Plan

None - plan executed exactly as written.

## Known Stubs

None.

## Threat Surface Scan

T-14-01-01 (Spoofing) — mitigated: `DialConfig.Hostname` receives `HostEntry.HostName` (the real hostname from ssh config), not the alias label. The `resolveHostString` function builds a synthetic URL from `entry.HostName`, which flows into `ParseHost` and then `cfg.Host.Hostname`. No alias label leaks to the dial or known_hosts verification path.

T-14-01-04 (Elevation of Privilege) — mitigated: grep confirms no `InsecureIgnoreHostKey()` in production code; new LookupHost/resolveHostString paths do not touch host key verification.

No new threat surface beyond what is documented in the plan's threat model.

## Self-Check: PASSED
