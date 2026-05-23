---
quick_id: 260523-fix
slug: fix-ci-integration-dial-tests
description: Fix integration tests failing on CI — TestDial_Timeout and TestDial_UnknownHost_TOFU_Accepted
date: 2026-05-23
status: complete
---

# Quick Task: Fix CI integration test failures

## Root Cause

On GitLab CI (Ubuntu, no SSH agent, no `~/.ssh/config`), `buildAuthMethods()` returns an empty
slice. `Dial()` had an early return guarding against empty auth methods BEFORE attempting any
TCP connection. This caused:

1. `TestDial_Timeout` — expected "timed out" but got "SSH auth failed" (never reached TCP)
2. `TestDial_UnknownHost_TOFU_Accepted` — TOFU never fired (never reached TCP → known_hosts stayed empty)

## Changes

### `internal/ssh/client.go`
- Remove early-return for `len(authMethods) == 0`; let TCP connect proceed so timeout and TOFU
  paths work regardless of whether SSH keys are configured on the runner
- Set `ClientConfig.Timeout = timeout + 100ms` (vs. our `time.After(timeout)`) so the
  goroutine+select always wins the race, guaranteeing the deterministic "SSH connection timed out
  after X" message rather than an OS-level "i/o timeout" that doesn't match the test's check
- `formatDialError` already handles "unable to authenticate" / "no supported methods remain"
  so the user-visible "SSH auth failed" message is unchanged for real auth failures

### `integration/dial_test.go`
- Remove stale comment that incorrectly attributed the CI fix to `emptyKnownHosts`

## Tasks

- [x] T1: Fix `Dial()` in `internal/ssh/client.go`
- [x] T2: Remove stale comment in `integration/dial_test.go`
