---
phase: 5
phase_name: "Pre-flight & Health Polling"
project: "docker-deploy"
generated: "2026-06-05"
counts:
  decisions: 6
  lessons: 5
  patterns: 5
  surprises: 2
missing_artifacts:
  - "05-CONTEXT.md was present but not in the input file list — read as supplemental context"
---

# Phase 5 Learnings: Pre-flight & Health Polling

## Decisions

### SSHRunner interface accepted by RunPreflightChecks instead of *gossh.Client directly
`RunPreflightChecks` accepts an `SSHRunner` interface, not a concrete `*gossh.Client`. A `NewSSHRunner(*gossh.Client) SSHRunner` exported adapter is provided for production call sites.

**Rationale:** `*gossh.Client.NewSession()` returns `(*gossh.Session, error)`, but Go interfaces do not support covariant return types. A direct interface match is impossible. The adapter pattern solves this while keeping tests free of real SSH connections. The pattern was independently rediscovered during GREEN phase when the compile-time guard failed.
**Source:** 05-02-SUMMARY.md

---

### CHECK-05 (passwordless sudo) is conditional, not unconditional
`checkSudo()` is only called from inside `checkDockerGroup` or `checkTargetDir` when those checks need to escalate to sudo. It is not run unconditionally as part of the ordered check sequence.

**Rationale:** Running a sudo check unconditionally would cause spurious failures for users who own their target directory and are already in the docker group — the common case. The check is only meaningful when sudo is actually needed.
**Source:** 05-02-PLAN.md, 05-02-SUMMARY.md

---

### CHECK-03 (daemon not running) and CHECK-07 (root user) are warn-only, never block
Both checks print a warning to `os.Stderr` and append a `CheckResult{Status: "warn"}` but never return an error.

**Rationale:** A stopped daemon is a transient condition (will likely be running by deploy time). Deploying as root is inadvisable but valid. Neither condition justifies preventing the deploy.
**Source:** 05-02-PLAN.md, 05-02-SUMMARY.md

---

### HealthTimeout and HealthInterval accepted in Resolve() signature before CLI flags are registered
The flag parameters `flagHealthTimeout int` and `flagHealthInterval int` were added to `Resolve()` in Phase 5, but no cobra flags were registered for them. Callers pass `0, 0`.

**Rationale:** Future-proofing the signature so that adding `--health-timeout` and `--health-interval` flags later requires no signature change. Zero is treated as unset; the same `> 0` guard pattern is used for all int fields.
**Source:** 05-01-SUMMARY.md

---

### HealthInterval=0 treated as 1ms inside pollHealthWithRunner for test-fast mode
When `cfg.HealthInterval` is 0, the poll loop uses a 1ms effective interval rather than a zero-duration ticker (which would busy-spin).

**Rationale:** Production `Resolve()` always sets `HealthInterval >= 1` (default 5). Test cases pass `HealthInterval: 0` to make polls fire immediately without real sleep. The 1ms floor prevents test hangs and infinite loops.
**Source:** 05-03-SUMMARY.md

---

### fakeClient session pool exhaustion as a test safety net
The fake SSH client in poll_test.go uses an ordered slice of fakeSession instances. When the slice is exhausted, the fake returns an error. This catches tests that loop more times than expected.

**Rationale:** An infinite or unexpectedly long poll loop in a test with an unbounded fake would cause the test to hang silently. Exhaustion-as-error converts that into a detectable test failure.
**Source:** 05-03-SUMMARY.md

---

## Lessons

### Interface adapter is required when gossh.Session return type prevents direct interface satisfaction
`*gossh.Client.NewSession()` returns `(*gossh.Session, error)`. Defining an interface with `NewSession() (Session, error)` (where Session is a local interface) does not compile — the concrete type does not satisfy the interface because the return types differ. The adapter wrapper is necessary, not optional.

**Context:** This was discovered during the GREEN phase when a compile-time interface guard failed. The fix (NewSSHRunner adapter) was added immediately. The same pattern will apply to any future package that needs to inject fake SSH clients.
**Source:** 05-02-SUMMARY.md

---

### Pre-flight UAT was committed before Plans 02–04 were implemented
The 05-UAT.md file was committed at 08:21; Plans 02 (preflight), 03 (health), and 04 (wiring) were committed between 08:43 and 09:02. The UAT source field explicitly lists only `[05-01-SUMMARY.md]`, meaning the UAT covered only the config extension, not the pre-flight or health behaviors.

**Context:** The 05-VERIFICATION.md flagged this as a gap: "05-04-SUMMARY claims human verification passed but no UAT test records exist for SC-1 through SC-6." The gap was closed by the 05-HUMAN-UAT.md which ran the six scenarios against a real SSH host. When UAT is generated incrementally, ensure it is regenerated after all plans in the phase complete.
**Source:** 05-VERIFICATION.md

---

### docker compose label filter requires ShellQuote on projectName
`docker ps --filter label=com.docker.compose.project=<projectName>` injects `projectName` directly into a shell command. Project names derived from `filepath.Base(cwd)` can contain spaces or special characters if the user has an unusual directory name.

**Context:** ShellQuote was applied at `poll.go:161` for projectName and `poll.go:239` for container names from docker ps output. Container names from docker ps are Docker-managed identifiers and generally safe, but ShellQuote is applied defensively.
**Source:** 05-03-PLAN.md, 05-03-SUMMARY.md

---

### Inspect session errors during polling should be treated as warnings, not hard failures
If `docker inspect` fails mid-poll (e.g., the container exited between the docker ps and the inspect call), the error is treated as unknown with a per-container warning rather than causing PollHealth to return an error.

**Context:** Race conditions between container enumeration and health inspection are normal in ephemeral environments. Hard-failing on inspect errors would produce false negatives. The container continues to be polled on the next tick.
**Source:** 05-03-SUMMARY.md

---

### Resolve() call sites in main.go updated by Plan 01, not Plan 04
The Resolve() 10-arg signature was applied to main.go as part of Plan 01 (config extension), not as a separate step in Plan 04 (wiring). When Plan 04 ran, the main.go call sites were already updated.

**Context:** 05-04-SUMMARY.md notes: "The Resolve() 10-arg signature was already applied in main.go (done as part of Plan 01 which updated the signature), so no update was needed there." This is an example of Plan 01 doing slightly more than scoped — it fixed the build break immediately rather than leaving main.go broken until Plan 04.
**Source:** 05-04-SUMMARY.md

---

## Patterns

### Narrow Session/SSHRunner interface injection for SSH-dependent packages
Define a narrow interface (e.g., `Session{Output, Run, Close}` and `SSHRunner{NewSession}`) in the package, provide an adapter constructor for the concrete `*gossh.Client`, and have the exported function accept the interface. Tests inject a fake that returns scripted per-command responses based on command substring matching.

**When to use:** Any new package that needs to call SSH exec commands and needs unit-testable behavior. The Session interface matches gossh.Session method signatures exactly, so no runtime type assertion is needed. The pattern was established in Phase 5 preflight and reused in health.
**Source:** 05-02-SUMMARY.md

---

### ticker + timer select loop for configurable polling with timeout
Use `time.NewTicker(healthInterval)` and `time.NewTimer(healthTimeout)` in a select loop with a `ctx.Done()` case. The select fires on whichever channel delivers first — tick, timeout, or context cancellation.

**When to use:** Any polling loop that needs a configurable interval, a maximum duration, and context-cancellation support. This is the standard Go timeout pattern. The `HealthInterval=0 → 1ms` floor enables test-fast mode without changing the loop structure.
**Source:** 05-03-PLAN.md, 05-03-SUMMARY.md

---

### done map for tracking terminal container states across poll ticks
Maintain a `done map[string]bool` keyed by container name. Mark a container done when it reaches "healthy" or "no-healthcheck". Check `len(done) == len(containers)` at each tick to detect all-complete.

**When to use:** Any polling loop that needs to track partial completion across multiple ticks without re-evaluating already-terminal items. Avoids repeated inspect calls for containers that have already reached a stable state.
**Source:** 05-03-SUMMARY.md

---

### Actionable error messages: include the exact fix command in the error string
Error messages for CHECK-04 and CHECK-05 failures include the exact command the operator should run: `"fix: sudo usermod -aG docker <user>"` and `"fix: add '<user> ALL=(ALL) NOPASSWD: ALL' to /etc/sudoers.d/<user>"`.

**When to use:** Any error resulting from a missing but fixable configuration. The operator runs docker deploy on a new server; they need to know exactly what to do next without consulting documentation.
**Source:** 05-02-PLAN.md

---

### Zero-value int as "unset" sentinel for int config fields
Int config fields use `> 0` as the "is set" guard: `if flagHealthTimeout > 0 { cfg.HealthTimeout = flagHealthTimeout } else if file.Target.HealthTimeout > 0 { ... } else { cfg.HealthTimeout = 60 }`. This treats zero and negative YAML values as unset and applies the default.

**When to use:** Int-typed config fields where zero is not a valid configuration value. Consistent with the empty-string-as-unset pattern used for string fields. Documents that zero and negative values are ignored, preventing surprising behavior from a deploy.yaml typo.
**Source:** 05-01-SUMMARY.md

---

## Surprises

### REQUIREMENTS.md checkboxes not updated after phase completion
After Phase 5 was fully implemented and tested, REQUIREMENTS.md still showed all 10 Phase 5 requirements as `[ ] Pending`. The 05-VERIFICATION.md noted this explicitly: "This is a documentation gap only; the implementation is present and tested."

**Impact:** Requirement traceability documents require manual updates after each phase. The automated verification process checks implementation artifacts and tests, but does not update source documents. A recurring cleanup task or post-phase hook would prevent this drift.
**Source:** 05-VERIFICATION.md

---

### Preflight CheckResult slice is discarded in Phase 5 but designed for Phase 7 reuse
`RunPreflightChecks` returns `([]CheckResult, error)`. In Phase 5, the slice is discarded with `_ = results`. The CheckResult struct (Name, Status, Message) was defined with Phase 7 verbose mode in mind.

**Impact:** The API was designed for a future consumer before that consumer existed. When Phase 7 implemented verbose preflight rendering, it iterated `results []preflight.CheckResult` at `main.go:251-263` without any changes to the preflight package. The upfront design paid off — no interface change was needed.
**Source:** 05-02-SUMMARY.md, 05-04-SUMMARY.md
