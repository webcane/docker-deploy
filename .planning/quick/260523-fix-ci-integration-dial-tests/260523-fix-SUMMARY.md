---
status: complete
---

# Summary: Fix CI integration test failures

Fixed two integration tests failing on GitLab CI.

## What changed

**`internal/ssh/client.go`**
- Removed the early-return guard (`len(authMethods) == 0`) that fired before any TCP connection
- Changed `ClientConfig.Timeout` to `timeout + 100ms` to ensure `time.After(timeout)` always
  fires first, making the "timed out" error message deterministic across OS/network configurations

**`integration/dial_test.go`**
- Removed stale comment that wrongly credited `emptyKnownHosts` as the CI fix

## Why it works

- `TestDial_Timeout`: CI has no SSH agent/keys → `buildAuthMethods` returns empty → previously
  bailed before TCP; now proceeds to `gossh.Dial` → `time.After(500ms)` fires → "SSH connection
  timed out after 500ms" → ✓
- `TestDial_UnknownHost_TOFU_Accepted`: SSH key exchange (HostKeyCallback / TOFU) happens before
  auth in the SSH protocol; removing the early return lets TOFU fire and write to known_hosts
  even when no auth methods are configured

## Commits

Changes staged for atomic commit.
