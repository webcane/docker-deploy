---
status: complete
---

# Summary: Fix CI DinD SSH host key capture race condition

## Root cause

`wait.ForListeningPort("22/tcp")` fires as soon as sshd binds the TCP port, but on
CI (slower/more loaded) sshd is not yet ready to complete SSH handshakes. A single
dial attempt → `HostKeyCallback` never called → `captured == nil` → fatal error.
Works locally because local Docker starts sshd faster.

## What changed

**`integration/helpers_test.go` — `captureHostKeyFromContainer`**
- Replaced single-attempt logic with a retry loop: retries every 500ms for up to 30s
- Reduced per-attempt `Timeout` from 15s → 5s (fail faster per attempt, retry more)
- Added context cancellation check between retries
- Updated error message to include the 30s timeout duration for clarity

## Why it works

The retry loop bridges the window between "port is open" (testcontainers' wait strategy)
and "sshd is fully ready to handshake." On CI this gap may be 1–5 seconds; locally it's
near-zero. The 30s window is generous enough for even slow CI without adding meaningful
test time when sshd is ready quickly.

## Commits

- fix(integration): retry SSH host key capture to fix CI race condition
