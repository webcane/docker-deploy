---
phase: 4
phase_name: "Core Deploy Loop"
project: "docker-deploy"
generated: "2026-06-05"
counts:
  decisions: 7
  lessons: 5
  patterns: 5
  surprises: 2
missing_artifacts:
  - "04-CONTEXT.md was present but not in the input file list — read as supplemental context"
---

# Phase 4 Learnings: Core Deploy Loop

## Decisions

### Resolve() signature extended via explicit positional parameters before struct
Two new parameters (`flagComposeFile string`, `localDir string`) were appended after `flagForce` and before `file FileConfig` — not refactored into a struct. The struct refactor came later in Phase 7. This decision introduced a growing positional signature (8 args → 10 args by Phase 5) that was resolved in Phase 7 with FlagOpts.

**Rationale:** Pragmatic forward momentum; struct refactor was planned but deferred to avoid scope creep mid-phase.
**Source:** 04-01-SUMMARY.md

---

### session.Start() preferred over session.Run() for compose execution
RunCompose uses `session.Start(cmd)` instead of `session.Run(cmd)`, enabling the non-TTY path to call `wg.Wait()` to drain stdout/stderr pipes before calling `session.Wait()`.

**Rationale:** `session.Run()` does not allow goroutine pipe-drain ordering; `session.Start()` gives explicit control over the sequence: start → drain goroutines → wait for exit.
**Source:** 04-02-SUMMARY.md

---

### PTY sets session.Stderr = os.Stdout (intentionally)
When stdout is a TTY, `session.Stderr` is assigned to `os.Stdout` rather than `os.Stderr`. This is not a mistake — PTY merges both streams into a single byte stream.

**Rationale:** PTY output is inherently multiplexed; the local terminal's PTY driver handles the merged stream correctly. Separating them in PTY mode would be incorrect.
**Source:** 04-02-SUMMARY.md

---

### session.Stdin intentionally not connected to os.Stdin
In RunCompose, the PTY path does not wire `session.Stdin` to `os.Stdin`. The PTY allocates a terminal environment for compose output rendering only.

**Rationale:** Prevents interactive input from reaching the remote shell during compose execution (T-04-02-03). compose up -d is not interactive.
**Source:** 04-02-SUMMARY.md

---

### Basename validation placed in runDeploy(), not inside RunCompose()
The `filepath.Base(resolved.ComposeFile) == resolved.ComposeFile` injection guard lives in main.go, not in the compose package.

**Rationale:** The trust boundary (user-supplied input entering a remote command) is crossed at the main.go layer. Placing the guard there aligns the mitigation with where control is acquired, keeping RunCompose focused on SSH execution semantics.
**Source:** 04-03-SUMMARY.md

---

### Auth fallback happens during deploy execution, not at preflight
The three-stage auth sequence (direct copy → passwordless sudo → interactive password) is triggered during Upload(), not as a preflight check blocker.

**Rationale:** CHECK-05 (passwordless sudo verification) was reclassified as a warning. Blocking at preflight would reject valid deployment scenarios (root user, user who knows their sudo password). Deferring to execution means the deploy proceeds as far as possible.
**Source:** 04-04-PLAN.md, 04-04-SUMMARY.md

---

### Password collected once and reused across mkdir/mv/rm operations
A `*string` pointer is passed to Upload(); tryAuthFallback populates it on first interactive prompt and subsequent sudo operations reuse the cached value within the same deploy.

**Rationale:** Asking the user for their sudo password multiple times per deploy (once for mkdir, once for mv, once for rm) would be a poor UX. Collecting once and threading the pointer through solves this without global state.
**Source:** 04-04-SUMMARY.md

---

## Lessons

### Changing Resolve() signature breaks all call sites including tests
Extending Resolve() with new positional parameters caused compile failures in both main.go and config_test.go. Pre-existing test call sites required mechanical updates to pass sentinel values for the new parameters.

**Context:** Five pre-existing Resolve() call sites in config_test.go were updated to pass `"compose.yaml"` as flagComposeFile (bypassing auto-detect) and `""` as localDir. The pattern of accumulating positional params accelerates this problem — Phase 7 fixed it with FlagOpts struct.
**Source:** 04-01-SUMMARY.md

---

### PTY test coverage is practically impossible in unit tests
The TTY detection branch in RunCompose cannot be unit-tested because `term.IsTerminal` requires a real terminal file descriptor. RequestPty also requires a real SSH daemon that supports PTY allocation.

**Context:** The five TDD tests for RunCompose all run in non-TTY mode. The PTY path was verified only through the Phase 03 human checkpoint. This gap is inherent to the domain.
**Source:** 04-02-PLAN.md, 04-02-SUMMARY.md

---

### Interactive password tests deferred due to stdin mocking complexity
Five of the seven auth fallback tests were marked `t.Skip()` because testing interactive stdin input (golang.org/x/term.ReadPassword) in a unit test requires controlling the TTY file descriptor, which is non-trivial in Go test environments.

**Context:** Direct copy (TestUploadAuthFallback_DirectCopy) and passwordless sudo (TestUploadAuthFallback_PasswordlessSudo) were testable; interactive password, wrong password, timeout, root user, and exhausted paths were deferred.
**Source:** 04-04-SUMMARY.md

---

### compose up -d is detached — context.Background() is safe without a deadline
RunCompose is called with `context.Background()`. The concern about long-blocking would apply to a foreground compose run, but `-d` (detach) causes compose to return once containers are started, not once they are healthy.

**Context:** Health polling in Phase 5 handles the "did containers actually become healthy" question. The two concerns (compose launch and health verification) are correctly separated across RunCompose and PollHealth.
**Source:** 04-03-SUMMARY.md

---

### Human UAT revealed an SSH connectivity regression unrelated to the compose feature
During 04-UAT.md testing, Test 2 (full deploy with streaming output) failed with "dial tcp 192.168.1.99:22: connect: no route to host" even though direct SSH from the terminal to the same host worked.

**Context:** This was a test environment network issue, not a code regression. The 04-HUMAN-UAT.md (run separately) confirmed all 6 tests passed against the real host. The automated UAT run and the human UAT run targeted different contexts. The compose feature itself was not implicated.
**Source:** 04-UAT.md

---

## Patterns

### In-process SSH server mock for unit testing remote exec commands
Tests use golang.org/x/crypto/ssh server API to spin up a real (in-process) SSH server, inject a configurable exec handler, and verify the exact command string received — without any external process or network dependency.

**When to use:** Whenever you need to unit-test code that calls `client.NewSession()` and `session.Start()`/`session.Run()`/`session.Output()`. The mock captures the command string and returns a configurable exit code + stdout. Established in Phase 3 (upload_test.go) and reused here for compose/run_test.go.
**Source:** 04-02-PLAN.md

---

### wg.Wait() before session.Wait() for non-TTY pipe drain
In non-TTY SSH exec: launch io.Copy goroutines (stdout → os.Stdout, stderr → os.Stderr) with a sync.WaitGroup, call wg.Wait() after session.Start() returns, then call session.Wait(). This ordering ensures all pipe output is flushed before exit status is checked.

**When to use:** Any SSH session that reads from StdoutPipe/StderrPipe in goroutines. The reverse order (session.Wait() before wg.Wait()) can cause truncated output because the pipes close when the session ends.
**Source:** 04-02-SUMMARY.md

---

### Injection guard: filepath.Base(value) == value before remote path concatenation
Before concatenating user-supplied values into SSH remote commands, validate that `filepath.Base(value) == value`. This rejects any value containing path separators, preventing directory traversal in remote command construction.

**When to use:** Any user-supplied basename that will be concatenated (not shell-quoted) into a remote shell command. Used here for ComposeFile. Complements ShellQuote, which handles values with spaces and special characters in known positions.
**Source:** 04-03-SUMMARY.md

---

### Error propagation: inner functions write human-readable failure lines; outer callers propagate error
RunCompose writes "Deploy failed: docker compose exited with code N" to stderr itself. runDeploy() does not add a second error print — it just returns the error for exit code propagation.

**When to use:** When a lower-level function has full context for a user-facing message (e.g., it knows the exit code), have it write the message and return the error. The calling layer handles exit code propagation only. Prevents double-printing.
**Source:** 04-03-SUMMARY.md

---

### Three-stage auth fallback: direct → passwordless sudo → interactive password
A structured sequence of escalating privilege attempts: (1) try the operation without privilege escalation; (2) try with `sudo -n` (passwordless, non-interactive); (3) prompt user with `term.ReadPassword` up to 3 times. Fail with a clear message that lists which paths were exhausted.

**When to use:** Any SSH operation targeting a remote path that may require elevated privileges. Stage ordering ensures least-privilege is tried first. The warning at each escalation step (e.g., "passwordless sudo not configured") guides the operator toward a permanent fix without blocking the current deploy.
**Source:** 04-04-PLAN.md, 04-04-SUMMARY.md

---

## Surprises

### compose v1 `version` attribute surfaced as a warning, not an error, during human UAT
When testing compose failure scenarios, the `version` attribute in compose.yaml (valid in v1, deprecated in v2) appeared as a WARN line in compose output: "the attribute `version` is obsolete, it will be ignored, please remove it to avoid potential confusion." An empty compose file produced exit code 1.

**Impact:** Confirmed that compose v2 deprecation warnings flow through the PTY to the local terminal as expected. No code change required. Added to the human UAT record as test 6 (user-added). Also surfaced that sudo password prompts from compose appear in the streaming output ("sudo] password for remote host:") — compose may invoke sudo internally on some configurations.
**Source:** 04-03-SUMMARY.md

---

### Atomic file staging uses /opt/<project>/.deploy-tmp-<timestamp>, not /tmp
The CLAUDE.md critical rule (Rule 3) specifies staging to `/opt/<project>/.deploy-tmp-<timestamp>`, not to `/tmp`. This matters for auth fallback: /tmp is world-writable (direct copy always works), but the staging directory under /opt/<project> requires the same privilege escalation as the target directory.

**Impact:** Auth fallback must apply to the staging mkdir and mv operations, not just the final target write. The three-stage sequence in Upload() was designed around this constraint: SFTP writes to /tmp staging first (always succeeds), then auth fallback governs the mkdir/mv/rm operations on the target path.
**Source:** 04-04-PLAN.md
