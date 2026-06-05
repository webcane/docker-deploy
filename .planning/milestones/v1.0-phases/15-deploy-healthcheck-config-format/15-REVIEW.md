---
phase: 15-deploy-healthcheck-config-format
reviewed: 2026-05-31T00:00:00Z
depth: standard
files_reviewed: 7
files_reviewed_list:
  - cmd/docker-deploy/main.go
  - cmd/docker-deploy/main_test.go
  - internal/config/config.go
  - internal/config/config_test.go
  - internal/health/poll.go
  - internal/health/poll_test.go
  - integration/compose_test.go
findings:
  critical: 2
  warning: 4
  info: 1
  total: 7
status: issues_found
---

# Phase 15: Code Review Report

**Reviewed:** 2026-05-31T00:00:00Z
**Depth:** standard
**Files Reviewed:** 7
**Status:** issues_found

## Summary

Phase 15 introduced a `target.healthcheck` YAML sub-block, four-tier config precedence, per-container retry semantics, and `KnownFields(true)` strict parsing. The config resolution logic and YAML parsing are well-implemented with thorough test coverage for boundary cases. However, two blocker-level bugs exist in the interaction between the documented D-04 "zero means skip" contract and the actual polling behavior in `poll.go`, one of which can cause resource exhaustion on the remote host. Four additional warnings cover test accuracy, UX consistency, and documentation gaps.

## Critical Issues

### CR-01: `PollHealth` called unconditionally — violates D-04 "zero means skip" contract, can fail deploys

**File:** `cmd/docker-deploy/main.go:486`
**Issue:** `health.PollHealth` is called on every deploy regardless of whether any healthcheck fields were configured. The `HealthcheckConfig` docstring explicitly states: *"Fields are zero values when no healthcheck block is present in any config source, which signals that health polling should be skipped entirely (per D-04)."* The code does not honor this contract. When no healthcheck is configured (all fields zero), `pollHealthWithRunner` proceeds: it makes an SSH call to list containers, and if any containers have a Docker `HEALTHCHECK` instruction, it enters the poll loop with the `healthTimeout` guard of 1 second. Any container still in `"starting"` health state after 1 second causes the deploy to return a `"health: timed out"` error — a regression for every user who deploys containers with `HEALTHCHECK` but has not yet migrated to the new `target.healthcheck` config block.

**Fix:**
```go
// cmd/docker-deploy/main.go, replace line 485-488
// 10b. Poll container health after compose up completes.
// Per D-04: skip polling entirely when no healthcheck block was configured.
if resolved.Healthcheck.Interval > 0 || resolved.Healthcheck.Timeout > 0 || resolved.Healthcheck.Retries > 0 {
    if err := health.PollHealth(context.Background(), client, projectName, resolved); err != nil {
        return fmt.Errorf("health poll: %w", err)
    }
}
```

---

### CR-02: Timeout-only config hammers remote with ~1000 SSH sessions per second

**File:** `internal/health/poll.go:122-134`
**Issue:** When `Healthcheck.Interval` is zero but `Healthcheck.Timeout` is non-zero (e.g. a user sets only `timeout: 30s` in the YAML), the zero-guard on line 123 silently replaces the zero interval with `1ms`. This creates a ticker that fires 1000 times per second. Each tick opens a new SSH session via `newSession()` to call `docker inspect` for every container. For a 30-second timeout this means approximately 30,000 SSH sessions per container — effectively a denial of service against the target VPS. The user receives no warning that their partial config is being interpreted this way.

**Fix:** Add a cross-field validation in `Resolve()` that rejects an Interval-Timeout combination where exactly one of the two is zero. Alternatively, guard the zero-interval replacement to only apply when BOTH fields are zero (i.e. the fallback/test-fast path), and return an error from `Resolve()` for the partial case:

```go
// internal/config/config.go, add after the Timeout resolution block (~line 463)
// Reject partial healthcheck config: if one of Interval/Timeout is set, both must be set.
if (cfg.Healthcheck.Interval > 0) != (cfg.Healthcheck.Timeout > 0) {
    return Config{}, fmt.Errorf("healthcheck: both interval and timeout must be set together (one is zero)")
}
```

---

## Warnings

### WR-01: Interval-only config always fails due to 1-second minimum timeout guard

**File:** `internal/health/poll.go:127-128`
**Issue:** When `Healthcheck.Interval` is set (e.g. `10s`) but `Healthcheck.Timeout` is zero, the guard on line 127 sets `healthTimeout = 1s`. The first ticker tick fires at `T+10s`, but the timeout fires at `T+1s`, so the deploy always returns a timeout error immediately. The user sees `"health: timed out waiting for containers to become healthy"` with no indication that the real cause is their partial configuration. This is distinct from CR-02 (which covers the opposite order) but shares the same root cause: the cross-field validation in `Resolve()` described in the CR-02 fix would also prevent this scenario.

**Fix:** Same as CR-02 fix — add cross-field validation in `Resolve()` to reject partial interval/timeout configs.

---

### WR-02: `TestPollHealth_DeadImmediate` tests `"unhealthy"`, not `"dead"` state — misnaming and coverage gap

**File:** `internal/health/poll_test.go:118-132`
**Issue:** The test is named `TestPollHealth_DeadImmediate` and its comment says *"container with HEALTHCHECK returns 'unhealthy' immediately"* — but both the name and the intent say "dead". The `fakeSession` output is `"unhealthy"`, not `"dead"`. The `pollContainers` function has a separate `case "exited", "dead":` branch (poll.go:246-249) that is completely untested by unit tests. This means a regression in the `"exited"`/`"dead"` case would not be caught.

**Fix:** Rename the test to `TestPollHealth_UnhealthyImmediate` and add a new test `TestPollHealth_ExitedContainer` that injects `"exited"` as the inspect response and asserts the error contains `"stopped unexpectedly"`:

```go
func TestPollHealth_ExitedContainer(t *testing.T) {
    fc := newFakeClient(
        fakeSessionOut("exited-container\n"),
        fakeSessionOut("exited"),
    )
    err := pollHealthWithRunner(context.Background(), fc, "myproject", defaultCfg(10, 1))
    if err == nil {
        t.Fatal("expected non-nil error for exited container, got nil")
    }
    if !strings.Contains(err.Error(), "stopped unexpectedly") {
        t.Errorf("expected error to mention 'stopped unexpectedly', got: %v", err)
    }
}
```

---

### WR-03: `retries=1` is functionally identical to `retries=0` (immediate-fail) — silent UX trap

**File:** `internal/health/poll.go:261-273`
**Issue:** In `recordUnhealthy`, when `retries > 0`, the code increments `failCount` and checks `failCount[container] >= retries`. With `retries=1`, the check fires on the very first unhealthy result (`failCount=1 >= 1`), which is identical behavior to `retries=0` (immediate fail). A user who sets `healthcheck-retries: 1` expecting "allow one retry before failing" gets no retries at all. The flag description `"max consecutive unhealthy results before failing"` technically permits this interpretation, but `retries=1` meaning "zero retries allowed" is counterintuitive and will surprise users.

**Fix:** Either document this explicitly in the flag description, or shift the semantics so `retries=N` means "fail after N+1 consecutive unhealthy" (i.e. allow N retries), or add a minimum-effective-value validation:

```go
// cmd/docker-deploy/main.go flag registration
cmd.Flags().IntVar(&healthcheckRetries, "healthcheck-retries", 0,
    "max consecutive unhealthy results before failing the deploy (0=fail immediately, 1+ allows N-1 retries)")
```

---

### WR-04: No test for D-04 skip-polling contract

**File:** `internal/health/poll_test.go` and `cmd/docker-deploy/main_test.go`
**Issue:** The `HealthcheckConfig` documentation states that zero value signals health polling should be skipped entirely (D-04). There is no unit test that verifies this contract. If a future refactor changes the zero-guard behavior or the call site in `runDeploy`, the regression would be undetected. Combined with CR-01 (the contract is already violated), the lack of a test means the bug could ship silently.

**Fix:** After fixing CR-01, add a test to verify the skip behavior:

```go
// internal/health/poll_test.go
func TestPollHealth_ZeroConfigMakesNoSSHCalls(t *testing.T) {
    // fakeClient with no scripted responses; any call panics/errors
    fc := newFakeClient()
    cfg := config.Config{Healthcheck: config.HealthcheckConfig{}} // all zero
    // With the CR-01 fix, PollHealth should NOT be called from runDeploy.
    // This test ensures pollHealthWithRunner itself does not make SSH calls when
    // zero config is somehow passed through.
    // Currently this WILL call listContainers (1 session) and return nil only
    // if containers list is empty; adjust expectations per implementation.
    err := pollHealthWithRunner(context.Background(), fc, "proj", cfg)
    if err != nil {
        t.Fatalf("zero config should not fail: %v", err)
    }
}
```

---

## Info

### IN-01: Integration test comment implies a `time.Sleep` that does not exist

**File:** `integration/compose_test.go:111-113`
**Issue:** The comment on line 111 reads *"Allow a short pause for the container to exit before polling."* No `time.Sleep` or delay follows. The comment implies an intentional sleep was planned or removed, which could mislead future maintainers into thinking this delay is present. In practice the busybox container exits so fast that the 2-second poll interval catches it, but the comment is inaccurate.

**Fix:** Remove or rewrite the misleading comment:

```go
// No explicit delay needed: busybox exits immediately; the 2s poll interval
// in PollHealth will catch the "exited" state on the first tick.
```

---

_Reviewed: 2026-05-31T00:00:00Z_
_Reviewer: Claude (gsd-code-reviewer)_
_Depth: standard_
