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
  duration: "~12 min"
  completed: "2026-05-15"
  tasks_completed: 2
  tasks_total: 3
  checkpoint_pending: true
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

Status: PENDING — requires real-host verification against SSH host with root-owned /opt/.

Expected test sequence:
1. First deploy against host where sshuser does NOT own /opt/ — should prompt once for sudo password, complete successfully
2. Repeat deploy — should replace existing target without sudo prompt (directory now owned by connecting user)
3. Verify no "Process exited with status 1" errors

## Commits

- `77342b1` feat(03-04): fix ShellQuote to escape single quotes; use in main.go
- `6c49f86` feat(03-04): sudo ownership+chmod, sudo atomic mv, rollback on failure

## Deviations from Plan

None — plan executed exactly as written. All four gaps closed as specified. The only structural note: Task 3 is a `checkpoint:human-verify` requiring real-host UAT which cannot be automated.

## Known Stubs

None.

## Threat Flags

No new security surface introduced. Threat mitigations T-03-CR01 and T-03-CR01b from the plan's threat model are implemented (ShellQuote fix in upload.go and main.go).

## Self-Check: PENDING

(Will be completed after checkpoint is verified and final commit made)
