---
status: partial
phase: 04-core-deploy-loop
source: [04-VERIFICATION.md]
started: 2026-05-15T00:00:00Z
updated: 2026-05-15T00:00:00Z
---

## Current Test

[awaiting human testing]

## Tests

### 1. Full deploy with compose output streaming to a real TTY
expected: docker deploy --host ssh://user@host runs copy-then-compose; compose output streams line-by-line with colour; plugin prints 'Deploy complete: N files copied to host:/path' on success
result: [pending]

### 2. Compose file auto-detection from project root
expected: From a directory containing compose.yaml (no --compose-file flag), deploy uses compose.yaml on the remote
result: [pending]

### 3. Exit code non-zero on SSH connectivity loss mid-deploy
expected: If the SSH session drops during compose execution, plugin exits non-zero; context cancellation watcher closes session so session.Wait() unblocks
result: [pending]

## Summary

total: 3
passed: 0
issues: 0
pending: 3
skipped: 0
blocked: 0

## Gaps
