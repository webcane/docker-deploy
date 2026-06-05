---
phase: 13
phase_name: "cli-subcommands-deploy-ux"
project: "docker-deploy"
generated: "2026-06-05"
counts:
  decisions: 8
  lessons: 7
  patterns: 6
  surprises: 4
missing_artifacts: []
---

# Phase 13 Learnings: CLI Subcommands & Deploy UX

## Decisions

### LoadFile Never Calls os.Getwd() Internally
`config.LoadFile(cwd string)` constructs the deploy.yaml path as `filepath.Join(cwd, "deploy.yaml")` and never calls `os.Getwd()` internally. The caller owns cwd acquisition (`os.Getwd()` in `runDeploy`/`runDryRun`).

**Rationale:** Subcommands and tests need to supply an explicit directory; internalizing os.Getwd() would make the function non-injectable and break test isolation.
**Source:** 13-01-PLAN.md, 13-01-SUMMARY.md

---

### version != "dev" (not buildTime != "unknown") Gates the Built: Line
`runVersionTo()` uses `version != "dev" && buildTime != "unknown"` to decide whether to print the `Built:` line. The original condition (`buildTime != "unknown"` only) was wrong because `make build` always injects `buildTime` via ldflags regardless of tag status.

**Rationale:** GoReleaser injects a semver version for tagged releases; `make build` leaves version as `"dev"` for local/untagged builds. The `version` var is the correct discriminator for "is this a release build?"
**Source:** 13-08-PLAN.md, 13-08-SUMMARY.md

---

### SudoCreds Stores Password as []byte for Safe Zeroing
`SudoCreds.pw` is `[]byte` (not `string`) so `Zero()` can range-wipe the bytes before nilting the slice. Go strings are immutable — a string password cannot be zeroed in place.

**Rationale:** Passwords in memory should be cleared after use; `[]byte` is the standard Go approach for erasable secrets (used by `golang.org/x/crypto`).
**Source:** 13-04-PLAN.md, 13-04-SUMMARY.md

---

### SudoExec as Exported Function (Not Closure)
The `sudoRunWithFallback` closure was extracted into an exported `SudoExec` function. A closure inside `Upload()` is not unit-testable and rollback paths could not call it.

**Rationale:** Testability of the four-step sudo fallback requires an exported function; rollback paths in `Upload()` also need the same fallback chain (D-15 / feedback_sudo_rollback.md).
**Source:** 13-04-PLAN.md

---

### Confirm Prompt Lives Inside Upload(), Not in main.go
The "Replace all contents?" confirm prompt was moved from `main.go` into `Upload()`. The SFTP client (needed for the remote file listing diff) only opens inside `Upload()`, so the diff cannot be shown before the prompt if the prompt stays in `main.go`.

**Rationale:** The verbose pre-confirm diff requires access to `sftpClient.ReadDir`; moving the prompt into `Upload()` keeps the diff and prompt in sequence without an extra SSH connection.
**Source:** 13-05-PLAN.md, 13-05-SUMMARY.md

---

### needsSudo Probe Checks Only the Parent Directory
The sudo detection probe is `test -w path.Dir(remoteBase)` — only the parent directory. The original OR probe (`test -w remoteBase || test -w parent`) short-circuited when the user owned `remoteBase`, but `mkdir`/`mv`/`rm` all operate on entries _within_ the parent, requiring the parent to be writable.

**Rationale:** Filesystem operations that create, rename, or delete entries require write permission on the _containing directory_ (parent), not on the entry itself.
**Source:** 13-10-PLAN.md, 13-10-SUMMARY.md

---

### SilenceUsage: true on Subcommands That Return Errors
Every subcommand (`version`, `validate`) sets `SilenceUsage: true`. Without this, cobra prints the full usage block on any error return, which is noisy and misleading when the error is about input data (e.g., missing deploy.yaml) rather than a usage mistake.

**Rationale:** Usage output is appropriate when the user misused the CLI syntax; it is not appropriate when the user's config file is missing or invalid.
**Source:** 13-03-PLAN.md, 13-03-SUMMARY.md

---

### path.Dir (Not filepath.Dir) for Remote Linux Paths
The sudo probe and all remote path operations use `path.Dir` (the `path` package) instead of `filepath.Dir` (the `path/filepath` package). The remote host is always Linux; `filepath.Dir` is platform-aware and would use backslashes on Windows.

**Rationale:** Cross-compilation: the tool is built locally (potentially on Windows) but runs against a Linux remote; `path.Dir` always produces forward-slash paths.
**Source:** 13-06-PLAN.md

---

## Lessons

### make build Always Injects buildTime, Breaking the buildTime != "unknown" Guard
`make build` runs `$(shell date -u +%FT%TZ)` unconditionally and injects it as `buildTime`. Every local dev build therefore had a non-"unknown" buildTime, and the original guard showed the `Built:` line for every dev build. This was not caught in automated tests because tests set `buildTime = "unknown"` directly.

**Context:** UAT caught this: user saw "Built: 2026-05-26T12:09:30Z" in a dev build. The fix (plan 13-08) added a regression test `TestVersionCmd_DevBuildWithInjectedTime` that explicitly sets `buildTime` to a non-"unknown" value and asserts no `Built:` line.
**Source:** 13-UAT.md, 13-08-SUMMARY.md

---

### The OR Probe test -w target || test -w parent Masks False Negatives
A two-part OR probe for sudo detection was logically flawed: when the user owned the target directory (`/opt/test-deploy`), the first operand exited 0 and short-circuited the OR, making `needsSudo=false`. However, `mv /opt/test-deploy /opt/test-deploy-old-*` renames an entry _in `/opt`_, which requires write permission on `/opt`, not `/opt/test-deploy`. The mv then failed with exit 1.

**Context:** UAT caught this as a major bug (plan 13-06 OR probe → plan 13-10 parent-only fix). The lesson: when the correctness condition is "parent must be writable", checking the child's writability is both unnecessary and actively deceptive.
**Source:** 13-UAT.md, 13-10-SUMMARY.md

---

### Verbose Per-File Arrows and Pre-Confirm Diff Independently List the Same Files
The upload loop had per-file `-> filename` arrows (printed during SFTP staging) and the verbose pre-confirm diff had a `Local files (N):` section — both showing the same local file list in verbose mode. The arrows printed unconditionally during staging (before the confirm check); the diff printed them again before the prompt.

**Context:** UAT caught double-listing as a minor bug. Fix (plan 13-09): replace per-file arrows with a single `Uploading N files...` summary line before the loop; keep the pre-confirm diff block intact as the authoritative per-file listing.
**Source:** 13-UAT.md, 13-09-SUMMARY.md

---

### Existing Repeat-Deploy Tests Block When Confirm Prompt Moves Into Upload()
When the confirm prompt moved from `main.go` into `Upload()` (plan 13-05), existing repeat-deploy tests that used `force=false` would block waiting for stdin because `existsBefore=true` triggered the prompt. The fix updated those tests to `force=true` — they test atomic-swap mechanics and `.env` preservation, not the prompt behavior.

**Context:** Discovered during the GREEN phase of plan 13-05; four tests (`TestUploadRepeatDeploy_ThreeStepSwapUnchanged` and the three `TestUploadSkipEnvPreservesRemoteEnv` subtests) were updated to use `force=true`.
**Source:** 13-05-SUMMARY.md

---

### promptSudoPasswordFunc as Package-Level Variable Enables Test Injection
The interactive password prompt uses a `promptSudoPasswordFunc` package-level variable (type `func() (string, error)`). Tests inject a custom function that returns a predetermined password without needing a real terminal. The production value prompts via stderr/stdin.

**Context:** Without this injection point, the interactive prompt path in `SudoExec` step 4 would be untestable in unit tests. `TestSudoExec_SinglePromptMultiFile` depends on this.
**Source:** 13-04-SUMMARY.md

---

### Prompt Read Error (io.EOF) Must Break Out of Attempt Loop, Not Return Directly
In `SudoExec` step 4, if `promptSudoPasswordFunc()` returns an error (e.g., `io.EOF` when stdin is closed), the original code returned that error directly. This meant callers received `EOF` instead of the canonical "no valid auth path available" error message.

**Context:** Caught during the GREEN phase of plan 13-04 via `TestSudoExec_AllStepsExhausted`. Fix: `break` on prompt read error so the attempt loop exhausts and falls through to the canonical error message.
**Source:** 13-04-SUMMARY.md

---

### test -w Is a Safe Read-Only Probe That Leaves No Partial State
The `test -w` shell command probes write permission on a directory without creating or modifying any file. It is safe to run unconditionally at the start of `Upload()` — it cannot leave partial state if the path doesn't exist yet.

**Context:** Correctness of the parent-only probe relies on `test -w /opt` returning exit 1 when `/opt` is root-owned, regardless of whether `/opt/myapp` exists. Confirmed in plan 13-06 and plan 13-10.
**Source:** 13-06-PLAN.md

---

## Patterns

### os.Stat Before LoadFile for Precise "Not Found" Error
`runValidate()` calls `os.Stat(filepath.Join(cwd, "deploy.yaml"))` before `config.LoadFile(cwd)`. If `os.IsNotExist(err)` is true, it prints "deploy.yaml not found" to stderr and returns — rather than relying on LoadFile's zero-value return which silently continues.

**When to use:** Any subcommand that needs to distinguish "file missing" (actionable user error) from "file present but invalid" (a different error). Without the Stat check, both cases produce the same silent zero-value behavior.
**Source:** 13-03-PLAN.md, 13-03-SUMMARY.md

---

### Zero FlagOpts{} for Subcommands With No CLI Flags
Subcommands that perform local-only operations (e.g., `validate`) pass `config.FlagOpts{}` (zero value) to `config.Resolve()`. This reuses the existing resolution logic with no flags overriding file config, avoiding duplicate resolution code.

**When to use:** Any subcommand that reads deploy.yaml but does not expose per-run override flags (--host, --user, etc.).
**Source:** 13-03-PLAN.md

---

### runVersionTo(w io.Writer) for Writer-Injected Testability
The version output function takes an `io.Writer` parameter (`runVersionTo(w io.Writer)`). The public `runVersion()` calls it with `os.Stdout`. Tests inject a `bytes.Buffer` to capture and assert on output without stdout redirection.

**When to use:** Any function that produces user-facing output that needs to be tested. `io.Writer` injection is idiomatic Go; avoids `os.Stdout` capture hacks in tests.
**Source:** 13-02-SUMMARY.md

---

### Four-Step Sudo Fallback Order: Direct → Cached → Passwordless → Interactive
`SudoExec` tries operations in this order: (1) direct command (no sudo), (2) cached password if available, (3) passwordless `sudo -n`, (4) interactive password prompt (up to 3 attempts). This minimizes password prompts while handling both passwordless-sudo and password-required configurations.

**When to use:** Any SSH command execution where the target path may or may not require elevation; the step order ensures the least-privileged path is tried first.
**Source:** 13-04-PLAN.md, 13-04-SUMMARY.md

---

### execCmd Closure for Conditional SudoExec vs sshRun Dispatch
After computing `needsSudo`, an `execCmd(cmd string) error` closure dispatches to `SudoExec` (elevated) or `sshRun(nil)` (direct) based on the probe result. All 8 remoteBase operations call `execCmd` instead of either function directly, keeping the dispatch logic in one place.

**When to use:** When the same set of remote operations may need to be run with or without sudo depending on a runtime decision; a dispatch closure eliminates 8 conditional branches.
**Source:** 13-06-PLAN.md

---

### Verbose Best-Effort Diagnostics: Best-Effort With No Error Propagation
The verbose `sudo -l` block in `checkDockerGroup()` has no `else` branch — non-zero exit or error from `sudo -l` is silently swallowed. The block only runs when `cfg.Verbose` is true. This pattern ("best-effort diagnostic, failure never blocks") is reused for any verbose-only diagnostic command.

**When to use:** Any verbose diagnostic that retrieves additional context for debugging but whose failure does not affect correctness (e.g., listing sudo policy, showing remote system info). Never block a deploy because a diagnostic command failed.
**Source:** 13-07-PLAN.md

---

## Surprises

### The needsSudo OR Probe Introduced a Security-Adjacent Bug
The original OR probe (`test -w target || test -w parent`) was designed to handle the first-deploy case where `target` doesn't exist yet. But it introduced a logic inversion: a user-owned target with a root-owned parent correctly satisfies the first operand (`test -w /opt/test-deploy` exits 0) and skips the fallback entirely — causing the subsequent `mv` inside the root-owned parent to fail silently with exit 1. The fix (parent-only probe) is both simpler and more correct. The key insight: `mkdir`/`mv`/`rm` semantics depend on the _parent_, not the target.

**Impact:** UAT revealed this as a major deploy failure. The root cause was a well-intentioned probe design that checked the wrong permission layer. Regression test `TestUpload_PathAwareSudo_OwnsTargetButParentElevated` anchors the fix.
**Source:** 13-UAT.md, 13-10-SUMMARY.md

---

### Initial Verification Returned human_needed; Three Plans Were Added to Close Gaps
The initial Phase 13 verification scored 7/8 and returned `human_needed` status for three issues: (1) dev builds showing Built: line, (2) double file listing in verbose mode, (3) needsSudo false-negative. Three additional plans (13-08, 13-09, 13-10) were added to close the gaps. Re-verification scored 8/8 with no overrides.

**Impact:** Demonstrates that UAT-driven gap closure is effective — observed behavior in UAT translated directly into targeted bug-fix plans with regression tests. The phase effectively self-corrected through its own verification loop.
**Source:** 13-VERIFICATION.md, 13-UAT.md

---

### TDD RED Gate Forced Upfront Test Design for Five Behavior Tests
Plans 13-02, 13-03, 13-04, 13-05, 13-06, 13-07 all used TDD with separate RED commits (failing tests) before GREEN implementation commits. In plan 13-04, the RED gate surfaced the `io.EOF` prompt error bug during GREEN verification — the test `TestSudoExec_AllStepsExhausted` caught the wrong return value before the code was considered complete.

**Impact:** The TDD RED-GREEN split caught a real bug (prompt error bypassing canonical error message) that would have been harder to catch in a manual review. The explicit behavioral test spec in each plan made the RED commit mechanical to write.
**Source:** 13-04-SUMMARY.md

---

### ROADMAP SC-3 Wording "untagged builds print the short git commit hash" Was Ambiguous
ROADMAP success criterion 3 said "untagged builds print the short git commit hash." The implementation prints the hash on a separate `Git commit:` line, while `Version dev` appears on line 1. The human UAT raised the question: does SC-3 mean the hash appears _anywhere_ in the output, or in the _version field itself_? The human approved the `Git commit:` line as satisfying SC-3 — the version field intentionally shows `"dev"`.

**Impact:** Ambiguous success criteria in a ROADMAP can generate unnecessary verification questions. Future success criteria should specify the field name and format, not just the presence of a value.
**Source:** 13-HUMAN-UAT.md
