---
phase: 03-file-copy
plan: "04"
subsystem: filetransfer
tags: [gap-closure, shell-quoting, sudo, atomic-deploy, rollback]
dependency_graph:
  requires: [03-01, 03-02, 03-03]
  provides: [safe-shell-quoting, sudo-ownership-setup, sudo-atomic-mv, rollback-on-failure]
  affects: [internal/filetransfer/upload.go, cmd/docker-deploy/main.go]
tech_stack:
  added: []
  patterns: [shellquote-escape, sudoRun-closure, four-step-atomic-swap, rollback-on-step-failure]
key_files:
  created:
    - internal/filetransfer/shellquote_test.go
  modified:
    - internal/filetransfer/upload.go
    - cmd/docker-deploy/main.go
decisions:
  - "Export ShellQuote (was unexported shellQuote) so main.go can call filetransfer.ShellQuote without duplicating the fix"
  - "sudoPw captured as local var in Upload() so it persists across mkdir and mv steps within single invocation"
  - "sudoRun closure wraps sshExec with echo <pw> | sudo -S -p '' sh -c <cmd> when sudoPw non-empty"
  - "Four-step swap (staging->new, remoteBase->old, new->remoteBase, rm old) enables rollback at every step"
  - "Backup-dir cleanup failure is non-fatal: deploy succeeded; warning printed but no error returned"
metrics:
  duration: "~20 min"
  completed: "2026-05-15"
  tasks_completed: 3
  tasks_total: 3
  checkpoint_pending: false
---

# Phase 03 Plan 04: UAT Gap Closure — Shell Quoting, Sudo Ownership, Atomic Swap Summary

Four gaps found during human UAT and code review of Phase 3 are now closed in upload.go and main.go, with all existing tests passing.

## What Was Fixed

**Gap 1 — Directory ownership (CR missing chmod):** The sudo sh -c pipeline now runs `mkdir -p && chown $(id -un):$(id -gn) && chmod 755` in a single command, ensuring the connecting user has write permission on the newly-created directory under /opt.

**Gap 2 — Atomic mv needs sudo:** A `sudoRun` closure captures the sudo password from the mkdir step and routes all subsequent `mv`/`rm` commands through `echo <pw> | sudo -S -p '' sh -c <cmd>` when the password was needed. No second interactive prompt.

**Gap 3 (CR-01) — shellQuote command injection:** `shellQuote` renamed to `ShellQuote` (exported) with body changed to `"'" + strings.ReplaceAll(s, "'", "'\\''") + "'"`. All call sites in upload.go and main.go:179 updated. TestShellQuote table-driven tests added and passing.

**Gap 4 (CR-02) — No rollback on atomic swap step-2 failure:** Atomic swap is now four steps with explicit rollback: step-10.2 failure rolls back step-10.1; step-10.3 failure restores remoteBase from backup and includes backup path in error message.

## Files Modified

- `internal/filetransfer/upload.go` — ShellQuote fix, sudo section rewrite, four-step atomic swap with rollback
- `cmd/docker-deploy/main.go` — line 179 uses `filetransfer.ShellQuote(resolved.Path)` instead of inline interpolation
- `internal/filetransfer/shellquote_test.go` — new, table-driven tests for ShellQuote

## Test Results

```
go test ./... — all packages PASS
go build ./... — exit 0
go vet ./... — exit 0
TestShellQuote — 5/5 cases PASS (including /opt/foo'bar embedded-quote case)
```

## Verification Checks

- grep -rn "shellQuote" internal/ cmd/ — 0 results (all renamed)
- grep -n "strings.ReplaceAll" upload.go — line 255 (fix present)
- grep -n "chmod 755" upload.go — line 141 (in sudo pipeline)
- grep -n "sudoRun" upload.go — closure at line 182, used in both swap paths
- grep -n "oldDir" upload.go — 5 references (backup, rollback step-10.2, rollback step-10.3, rm cleanup, warning message)
- grep -n '"-new-"' upload.go — line 195 (four-step swap pattern)
- grep -n "Warning: could not remove backup" upload.go — line 216 (non-fatal Fprintf)

## Human Checkpoint (Task 3)

Status: APPROVED — human verified end-to-end against real SSH host.

Results:
1. First deploy (target absent, sshuser does not own /opt/) — succeeded. Sudo password prompted once, deploy completed without error.
2. Repeat deploy — succeeded. Existing target replaced; no "Process exited with status 1" errors.

Note: An additional fix commit (`05feee4`) was required before the checkpoint passed. The initial implementation had a bug in `sudoRun`: on repeat deploy `mkdir -p` succeeds without sudo (user now owns the dir), leaving `sudoPw` empty, so subsequent `mv` ops against root-owned `/opt` failed. Fix: `sudoRun` now tries each command without sudo first; on failure it collects the password interactively (once, up to 3 attempts) and retries. The four-step swap was also reverted to three steps to avoid a `/tmp -> /opt` pre-move that required sudo before the password was collected.

## Commits

- `77342b1` feat(03-04): fix ShellQuote to escape single quotes; use in main.go
- `6c49f86` feat(03-04): sudo ownership+chmod, sudo atomic mv, rollback on failure
- `05feee4` fix(03-04): lazy sudo in sudoRun, 3-step atomic swap

## Deviations from Plan

### Auto-fixed Issues

**1. [Rule 1 - Bug] Fixed lazy-sudo logic in sudoRun so repeat deploy succeeds**
- **Found during:** Task 3 human checkpoint (real-host verification)
- **Issue:** On repeat deploy, `mkdir -p` succeeds without sudo because the user now owns the directory. This left `sudoPw` empty. The original `sudoRun` closure only prefixed sudo when `sudoPw` was already set — meaning subsequent `mv` ops against root-owned `/opt` failed silently (no password collected, command ran without sudo and was rejected).
- **Fix:** `sudoRun` now tries each command without sudo first; on failure it collects the password interactively (up to 3 attempts) and retries. Collected `sudoPw` is reused for all subsequent calls. Reverted four-step swap to three steps to avoid a `/tmp -> /opt` pre-move that required sudo before the password was available.
- **Files modified:** `internal/filetransfer/upload.go`
- **Verification:** Both first deploy and repeat deploy succeed end-to-end against real host. `go test ./...` still passes.
- **Committed in:** `05feee4` fix(03-04): lazy sudo in sudoRun, 3-step atomic swap

---

**Total deviations:** 1 auto-fixed (Rule 1 bug)
**Impact on plan:** Fix essential for correctness on repeat deploy. No scope creep — the four gaps described in the plan are all closed.

## Known Stubs

None.

## Threat Flags

No new security surface introduced. Threat mitigations T-03-CR01 and T-03-CR01b from the plan's threat model are implemented (ShellQuote fix in upload.go and main.go).

## Self-Check: PASSED

- SUMMARY.md present at .planning/phases/03-file-copy/03-04-SUMMARY.md
- All task commits verified in git log: 77342b1, 6c49f86, 05feee4
- Human checkpoint approved: first deploy and repeat deploy verified working end-to-end
