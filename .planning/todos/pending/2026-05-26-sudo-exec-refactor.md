---
title: "Refactor sudoRunWithFallback into exported SudoExec package function"
status: pending
priority: P2
created: 2026-05-26
area: tooling
files:
  - internal/filetransfer/upload.go
---

## Problem

`sudoRunWithFallback` is a closure defined inside `Upload()`. It captures `sudoPw`, `warnedOnce`,
`client`, and `verbose` from the outer scope. This makes it untestable in isolation and prevents
reuse (e.g. from error/rollback paths that currently call `sshExec` directly, missing the sudo
fallback — see memory: `feedback_sudo_rollback.md`).

Additionally `sshExec` and `sshExecWithSudoPassword` are two separate functions that differ only
in whether they pipe a password via stdin. This creates unnecessary duplication.

## Solution

1. **Merge `sshExec` + `sshExecWithSudoPassword`** into one private primitive:
   ```go
   // pw=="" → session.Run(cmd), no sudo
   // pw!="" → session.Start("sudo -S -p '' sh -c <cmd>") + stdin pipe
   func sshRun(client *gossh.Client, cmd, pw string) error
   ```

2. **Extract `sudoRunWithFallback` into an exported package function**:
   ```go
   // SudoExec runs cmd on the remote with automatic privilege escalation.
   // Steps: direct → cached password → sudo -n → interactive password (up to 3 attempts).
   // creds and warnedOnce are in/out params to share state across calls.
   func SudoExec(client *gossh.Client, cmd string, creds *SudoCreds, warnedOnce *bool, verbose bool) error
   ```
   Step ordering: direct → cached password (skip prompt if already known) → passwordless sudo → interactive prompt.

3. **Replace `*string` with `*[]byte` for password storage** to allow zeroing after use:
   ```go
   type SudoCreds struct{ pw []byte }
   func (c *SudoCreds) Zero() { for i := range c.pw { c.pw[i] = 0 }; c.pw = nil }
   ```
   Caller defers `creds.Zero()` after `Upload()` returns. Go strings are immutable and cannot
   be zeroed — keeping as `[]byte` is standard practice (`golang.org/x/crypto` does the same).
   Also update `sshRun` to accept `[]byte` instead of `string` for the password argument.

4. Replace the closure in `Upload()` with `SudoExec(client, cmd, creds, &warnedOnce, verbose)`.

4. Remove `tryDirectCopy` and `tryPasswordlessSudo` — they become implementation details of
   `SudoExec`, not standalone exported helpers.

## Acceptance Criteria

- [ ] `SudoExec` is an exported function in `internal/filetransfer/`
- [ ] `Upload()` closure is gone; replaced with `SudoExec` calls
- [ ] `sshExecWithSudoPassword` is merged into `sshRun` (or equivalent)
- [ ] Password stored as `[]byte` (`SudoCreds`), not `string`; caller zeroes after use
- [ ] `SudoExec` uses cached password on 2nd+ call — no re-prompt within a single deploy
- [ ] Error/rollback paths in `Upload()` that need sudo use `SudoExec`, not bare `sshExec`
- [ ] All existing upload integration tests pass unchanged

## Related

- [[2026-05-26-sudo-path-aware-detection]] — probe whether sudo is needed at all before calling SudoExec
- [[preflight-ownership-check]] — seed: replace checkTargetDir sudo with stat+id introspection
