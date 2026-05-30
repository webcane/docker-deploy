---
phase: 15-deploy-healthcheck-config-format
reviewed: 2026-05-30T00:00:00Z
depth: standard
files_reviewed: 6
files_reviewed_list:
  - cmd/docker-deploy/main.go
  - integration/compose_test.go
  - internal/config/config.go
  - internal/config/config_test.go
  - internal/health/poll.go
  - internal/health/poll_test.go
findings:
  critical: 2
  warning: 3
  info: 2
  total: 7
status: issues_found
---

# Phase 15: Code Review Report

**Reviewed:** 2026-05-30
**Depth:** standard
**Files Reviewed:** 6
**Status:** issues_found

## Summary

Six files reviewed across the healthcheck config format phase: the main deploy command, integration tests, config resolution, and the health poll loop. The config resolution logic is well-structured with correct four-tier precedence and thorough test coverage. The poll loop correctly handles the core healthy/unhealthy/exited/timeout paths.

Two critical defects were found. First, `listContainers` uses `docker ps -a` (all containers) rather than `docker ps` (running only), which causes the health poller to immediately fail with "stopped unexpectedly" whenever a stopped container from a previous deploy (same project name, not yet pruned) is present. Second, the `--healthcheck-retries=0` flag value is indistinguishable from "flag not provided" due to using `> 0` as the presence test, breaking the documented flag-overrides-file precedence for that specific value.

Three warnings were also found, including double-printing of SSH errors, a stale step number comment, and misleading timeout error message text.

---

## Critical Issues

### CR-01: `docker ps -a` causes false "stopped unexpectedly" errors from stale containers

**File:** `internal/health/poll.go:182`

**Issue:** `listContainers` uses `docker ps -a` (the `-a` flag enumerates all containers, including stopped/exited ones). After a deploy, any previously exited container for the same compose project that has not been pruned (e.g. containers from a prior failed deploy, or any container with `restart: "no"` that has since exited) will appear in the list. `inspectHealth` will return `"exited"` for such a container, and `pollContainers` will immediately return `"health: container X stopped unexpectedly"` — causing the deploy to report failure even though the current compose stack is running correctly.

This is triggered in any real-world scenario where:
- The operator runs `docker deploy` a second time without first running `docker compose down`.
- Any service uses `restart: "no"` (one-shot tasks, DB migrations, etc.) and has since exited.
- The integration test `TestCompose_Healthy_NoHealthcheck` would spuriously fail if a prior test run left a stopped container with project label `compose-test-healthy`.

**Fix:** Remove the `-a` flag so only running/starting containers are enumerated:

```go
// Before:
cmd := "docker ps -a --filter " + filetransfer.ShellQuote(filterVal) + " --format '{{.Names}}'"

// After:
cmd := "docker ps --filter " + filetransfer.ShellQuote(filterVal) + " --format '{{.Names}}'"
```

Containers that have truly exited/stopped will be picked up by `inspectHealth` returning `"exited"` or `"dead"` only if they were running when enumeration occurred — which is the correct model. If a container was never started or has already been cleaned up it simply will not appear.

---

### CR-02: `--healthcheck-retries=0` cannot override a non-zero value from deploy.yaml or global config

**File:** `internal/config/config.go:459-467`

**Issue:** The retries resolution switch uses `opts.HealthcheckRetries > 0` as the "flag was provided" test:

```go
switch {
case opts.HealthcheckRetries > 0:
    cfg.Healthcheck.Retries = opts.HealthcheckRetries
case file.Target.Healthcheck.Retries > 0:
    cfg.Healthcheck.Retries = file.Target.Healthcheck.Retries
case globalFile.Target.Healthcheck.Retries > 0:
    cfg.Healthcheck.Retries = globalFile.Target.Healthcheck.Retries
}
```

The cobra flag is registered with default `0` (main.go line 84). There is no mechanism to distinguish "user passed `--healthcheck-retries=0`" from "user did not pass the flag at all". Consequently, if `deploy.yaml` or the global config sets `retries: 3`, a user cannot override it back to 0 (immediate-fail behaviour) with `--healthcheck-retries=0`. The documented four-tier precedence "flag > local file > global file" is violated for this specific value.

**Fix:** Use cobra's `Changed()` method to detect explicit flag provision, or use a sentinel value (e.g. `-1` means "not set" and is rejected as invalid separately from the negative-value check). The cleanest fix without changing the flag type is to accept the sentinel pattern at the `FlagOpts` level:

Option A — sentinel in FlagOpts (requires changing flag default in main.go):
```go
// In buildDeployCmd(): register default as -1 (sentinel for "not provided")
cmd.Flags().IntVar(&healthcheckRetries, "healthcheck-retries", -1, "...")

// In Resolve(): treat -1 as "not set"; treat 0 as explicit "immediate fail"
if opts.HealthcheckRetries == -1 {
    // flag not provided — fall through to file/global
} else {
    // flag was explicitly set (0 = immediate fail, >0 = threshold)
    cfg.Healthcheck.Retries = opts.HealthcheckRetries
}
```

Option B — add a `HealthcheckRetriesSet bool` field to `FlagOpts` and set it when the cobra flag is `Changed()`.

Note: the negative retries validation at lines 390-392 must be updated accordingly if the sentinel is changed to -1.

---

## Warnings

### WR-01: Double-printing of errors in `runDryRun` and `runDeploy`

**File:** `cmd/docker-deploy/main.go:298-300, 392-394, 454-457`

**Issue:** Several error paths explicitly print to stderr and then also return the error to cobra's `RunE`. Because the deploy command does not set `SilenceErrors: true`, cobra's default error handler will also print the returned error, producing duplicate output. For example an SSH failure produces:

```
SSH connection failed: <raw error>        ← from fmt.Fprintf at line 299
Error: SSH dial: <raw error>              ← from cobra's RunE error printing
```

The same double-print occurs for upload errors (lines 454-457). The `validate` and `version` subcommands correctly set `SilenceUsage: true` but none of the commands set `SilenceErrors`.

**Fix:** Either set `SilenceErrors: true` on the deploy command and rely solely on the explicit `fmt.Fprintf` calls, or remove the explicit `fmt.Fprintf` calls and let cobra print the returned (wrapped) error. The former is preferred because the explicit messages give cleaner user-facing context:

```go
cmd := &cobra.Command{
    Use:          "deploy",
    Short:        "Deploy a docker-compose project to a remote host",
    SilenceErrors: true,  // add this
    SilenceUsage:  true,  // consider adding this too
    RunE: func(...) error { ... },
}
```

---

### WR-02: Duplicate step label "// 5." in `runDeploy`

**File:** `cmd/docker-deploy/main.go:360,375`

**Issue:** `runDeploy` has two separate comment blocks both numbered "// 5.": line 360 (`// 5. Validate that a host was resolved.`) and line 375 (`// 5. Build ssh.DialConfig from the resolved config.`). The second "// 5." should be "// 6." (and all subsequent steps should be renumbered). The existing "// 6." comment at line 390 then collides with this. This is a maintenance issue that causes confusion when reading through the deploy flow.

**Fix:** Renumber so the flow reads: 1 (cwd), 2 (load local config), 3 (load global config), 4 (resolve), 5 (validate host), 5b (validate compose file), 6 (build dial config), 7 (dial), etc. — or simply relabel step 5b as step 6 and push subsequent numbers up by one.

---

### WR-03: Misleading timeout error message for containers with HEALTHCHECK

**File:** `internal/health/poll.go:153`

**Issue:** When the polling timeout fires, the per-container error message reads:

```go
fmt.Fprintf(os.Stderr, "Health check timed out after %s: container %s is not yet running\n", ...)
```

The phrase "is not yet running" is inaccurate for containers that have a HEALTHCHECK defined. Those containers are running (their `State.Status` is `"running"`) but their health check is still in `"starting"` state — they are not yet *healthy*. The message misleads the operator into thinking the container process hasn't started, when in fact Docker is still executing the HEALTHCHECK probes.

**Fix:**
```go
fmt.Fprintf(os.Stderr, "Health check timed out after %s: container %s is not yet healthy\n",
    cfg.Healthcheck.Timeout, c)
```

---

## Info

### IN-01: Integration test comments misrepresent the health status mechanism

**File:** `integration/compose_test.go:26-27, 38`

**Issue:** Lines 26-27 state "PollHealth polls `{{.State.Status}}` — 'running' is the terminal-success state." Line 38 repeats "poll.go checks `{{.State.Status}}` (running/exited/dead)". This is incorrect: `inspectHealth` in poll.go uses a compound Go template that checks `State.Status` for `"exited"/"dead"` but for a running container returns `State.Health.Status` (when present) or `"no-healthcheck"` — never the string `"running"`. The actual terminal-success status handled by `pollContainers` is `"no-healthcheck"` (or `"healthy"`), not `"running"`.

The tests themselves are correct (nginx:alpine has no HEALTHCHECK so `inspectHealth` returns `"no-healthcheck"`, which `pollContainers` correctly handles). Only the comments are wrong.

**Fix:** Update the comment at line 26-27 and 38 to accurately describe what the poller returns:

```go
// PollHealth returns nil because inspectHealth() returns "no-healthcheck" for containers
// without a HEALTHCHECK directive, which pollContainers treats as a terminal-success state.
```

---

### IN-02: TODO comment for image pinning left in integration test

**File:** `integration/compose_test.go:20-23`

**Issue:** A TODO comment notes that `nginx:alpine` and `busybox` should be pinned to fixed digest or version tags to avoid pulling "latest" on every CI run. This has correctness implications (image changes could break the test) but is categorised as info because the current behaviour is functional.

**Fix:** Pin both images to a specific version tag:
```yaml
# composeHealthyYAML
image: nginx:1.27-alpine

# composeUnhealthyYAML  
image: busybox:1.36
```

---

_Reviewed: 2026-05-30_
_Reviewer: Claude (gsd-code-reviewer)_
_Depth: standard_
