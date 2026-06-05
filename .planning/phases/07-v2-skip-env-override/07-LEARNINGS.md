---
phase: 7
phase_name: "v2 Skip-Env Override"
project: "docker-deploy"
generated: "2026-06-05"
counts:
  decisions: 6
  lessons: 5
  patterns: 5
  surprises: 3
missing_artifacts:
  - "07-CONTEXT.md was present but not in the input file list — read as supplemental context"
  - "07-SECURITY.md was present but not in the input file list"
---

# Phase 7 Learnings: v2 Skip-Env Override

## Decisions

### FlagOpts struct replaces 10-positional-parameter Resolve() signature
`Resolve()` was refactored from `func Resolve(flagHost, flagPath string, flagExcludes []string, flagForce bool, flagComposeFile string, flagHealthTimeout, flagHealthInterval int, file FileConfig, projectName string, localDir string)` to `func Resolve(opts FlagOpts, file FileConfig, projectName string, localDir string)`.

**Rationale:** The positional signature had grown to 10 parameters across Phases 4 and 5. Adding two more (SkipEnv, Verbose) would have made it 12. Struct literals are self-documenting, allow zero-value omission, and eliminate positional-order mistakes.
**Source:** 07-01-PLAN.md, 07-01-SUMMARY.md

---

### defaultExcludes expanded from 6 to 16 entries with dev-tooling directories
Ten new entries added: `.claude/`, `.github/`, `.planning/`, `.idea/`, `.vscode/`, `*.swp`, `*.swo`, `coverage/`, `dist/`, `.terraform/`.

**Rationale:** These are development artifacts that are rarely (if ever) intended for deployment. Including them by default reduces accidental uploads without requiring per-project deploy.yaml configuration.
**Source:** 07-01-PLAN.md, 07-01-SUMMARY.md

---

### SkipEnv appended in mergeExcludes() after the dedup loop, not as a separate step
When `cfg.SkipEnv` is true, `.env` is appended inside `mergeExcludes()` using the same seen-map dedup guard as other excludes. This ensures `.env` appears exactly once even if the user also listed it manually.

**Rationale:** Centralizing the append in mergeExcludes keeps the deduplication logic in one place. The alternative (appending after mergeExcludes) would require a separate dedup pass.
**Source:** 07-01-PLAN.md

---

### buildDeployCmd() extracted from main() for testable flag registration
The cobra command construction was moved from `main()` into `buildDeployCmd()`, enabling `main_test.go` to call `buildDeployCmd()` and assert that specific flags are registered — without invoking the plugin infrastructure.

**Rationale:** Flag registration tests (TestSkipEnvFlagRegistered, TestVerboseFlagRegistered) require access to the cobra command object. Embedding command construction in main() makes this impossible without running the full binary.
**Source:** 07-02-SUMMARY.md

---

### Interactive sudo command redacted in verbose output
When verbose mode logs SSH commands, the interactive sudo command (which contains the literal sudo password as `echo <pw> | sudo -S ...`) is printed as `[ssh] (sudo password cmd redacted)` rather than the full command string.

**Rationale:** Verbose output is written to stderr, which may be captured by logging pipelines. Printing a password to stderr even once would constitute a credential leak (T-07-02-05).
**Source:** 07-02-PLAN.md, 07-02-SUMMARY.md

---

### Warning rollup: non-blocking warnings accumulated then printed once without --verbose
A `var warnings []string` accumulator in `runDeploy()` collects non-blocking warning messages. At the end of the deploy, if `len(warnings) > 0 && !resolved.Verbose`, a single rollup line is printed: `WARN: there are some warnings during deployment. For more details use --verbose flag`.

**Rationale:** Printing every warning inline during a normal deploy creates noise. The rollup tells the operator that something worth examining occurred, while directing them to --verbose for details. With --verbose, each warning prints inline as it occurs — no rollup.
**Source:** 07-02-PLAN.md

---

## Lessons

### main.go call sites must be fixed immediately when Resolve() signature changes, even before the feature flag is wired
When Plan 01 introduced FlagOpts, `go build ./...` failed because main.go still used the old 10-positional-arg signature. The fix was applied in the same Plan 01 commit (Task 2), with SkipEnv and Verbose as zero values pending Plan 02 flag registration.

**Context:** This is the same lesson as Phase 4 Plan 01 (the Resolve() signature change broke main.go then too). The repeated pattern confirms that any Resolve() signature change must include a main.go call site fix in the same commit to maintain a buildable tree.
**Source:** 07-01-SUMMARY.md

---

### Upload() and RunCompose() call sites must be patched ahead of their task when go build is gated
During Task 1b (RunCompose verbose param), `go build ./...` verification failed because main.go still called Upload() and RunCompose() with the old signatures (from Task 1 having just added the verbose param to Upload()). The Task 2 patches were applied early with zero values.

**Context:** When multiple tasks in a wave each add a parameter to a different function, each task's GREEN commit must patch all downstream call sites — even if the final wiring is a later task's responsibility. Build integrity is a continuous invariant.
**Source:** 07-02-SUMMARY.md

---

### --skip-env requires a backup/restore of the remote .env during atomic swap
The original --skip-env implementation excluded `.env` from the SFTP upload but did not account for the atomic swap: the swap replaces the entire remote directory, which would delete the existing remote `.env`. A bug fix was required to add a backup/restore sequence.

**Context:** Before the atomic swap, Upload() checks if a remote `.env` exists (`test -f remoteBase/.env`), backs it up to `/tmp/docker-deploy-env-<ts>`, performs the swap, then restores the backup. Cleanup runs on both success and error paths. The original plan did not specify this behavior — it was discovered as a gap during verification.
**Source:** 07-VERIFICATION.md

---

### FlagOpts zero values in main.go are not stubs — they are documented transition states
When Plan 01 updated main.go call sites to use `config.FlagOpts{}` with zero values for SkipEnv and Verbose, the SUMMARY explicitly noted: "SkipEnv and Verbose remain zero values, pending Phase 7 Plan 02 flag registration." This is intentional, not a stub.

**Context:** The distinction matters for verification: a zero value with a documented pending-wiring comment passes verification; a zero value with no explanation is a stub that fails verification. Documenting the intent in the commit message and SUMMARY prevented false positive gap findings.
**Source:** 07-01-SUMMARY.md

---

### Verbose output verification requires a live SSH host and real TTY — unit tests cannot cover it fully
SC-6 (verbose output end-to-end) was marked UNCERTAIN in the automated verification report because the full call chain (Upload → SSH exec → RunCompose → TTY detection) cannot be exercised without a live SSH daemon and an interactive terminal.

**Context:** The unit tests cover the verbose code paths in isolation (sshExecVerbose, RunCompose verbose param). The human UAT (07-HUMAN-UAT.md) confirmed all four output categories appeared in stderr. The gap is inherent to SSH-dependent tools: behavioral verification always requires a real target.
**Source:** 07-VERIFICATION.md, 07-HUMAN-UAT.md

---

## Patterns

### FlagOpts struct for CLI flag aggregation
Define a `FlagOpts` struct that mirrors the cobra flag variables used in the command layer. Pass the struct to `Resolve()` rather than individual parameters. The struct is defined in the config package (not the cmd package) so downstream packages can import it without a cmd dependency.

**When to use:** Any CLI tool where the number of flags is growing and the Resolve() function is the primary consumer. The struct adds fields without changing the function signature, makes call sites self-documenting, and enables partial initialization (zero values for unset fields).
**Source:** 07-01-PLAN.md, 07-01-SUMMARY.md

---

### sshExecVerbose wrapper for SSH command logging
Define a package-private `sshExecVerbose(client, cmd, verbose bool) error` that logs `[ssh] <cmd>` before and `-> exit 0` / `-> exit N` after, when verbose=true. Replace direct `sshExec()` calls with `sshExecVerbose()` at all call sites where logging is desired.

**When to use:** Any function that executes SSH commands and needs to support a verbose mode without cluttering the main logic with conditional logging. The wrapper is thin and does not change error semantics. Note: redact commands that contain secrets (passwords, tokens) by substituting a redacted placeholder string.
**Source:** 07-02-PLAN.md, 07-02-SUMMARY.md

---

### verbose bool parameter propagated through the call chain as the last parameter
`Upload()` and `RunCompose()` each gained `verbose bool` as their last parameter. The value flows from `resolved.Verbose` (set in Resolve() from FlagOpts.Verbose) through runDeploy() to each function.

**When to use:** For cross-cutting behavioral toggles (verbose output, dry-run simulation) that affect multiple layers. Making it a parameter (rather than a global or context value) keeps the function signature honest about its behavior and makes testing straightforward — pass `false` in tests that don't exercise verbose output.
**Source:** 07-02-PLAN.md

---

### Warning rollup accumulator pattern
Declare `var warnings []string` at the start of the deploy function. For each non-blocking warning condition, either print inline (if verbose) or append to `warnings`. At the end of the function, if `len(warnings) > 0 && !verbose`, print a single rollup message directing the user to --verbose.

**When to use:** Command-line tools where non-blocking warnings should not interrupt the primary output flow in normal mode, but should be surfaced for investigation. Ensures warnings are never silently discarded while keeping terse output as the default.
**Source:** 07-02-PLAN.md, 07-02-SUMMARY.md

---

### Backup/restore for preserving remote files during atomic directory swap
When a file must be preserved through an atomic directory swap: (1) before the swap, check if the file exists remotely (`test -f`), (2) copy it to a temp path (`/tmp/<prefix>-<timestamp>`), (3) perform the swap, (4) restore from temp, (5) remove temp. Handle cleanup on both success and error paths.

**When to use:** Any deploy operation that atomically replaces a remote directory but needs to preserve specific files that were explicitly excluded from the upload (e.g., secrets files, local override configs). The backup/restore sequence must use the same auth fallback as other remote operations.
**Source:** 07-VERIFICATION.md, 07-02-SUMMARY.md

---

## Surprises

### Atomic directory swap silently deletes remote .env when --skip-env is active
The --skip-env feature was designed to exclude `.env` from the SFTP upload. However, the atomic swap (rename staging dir to target) replaces the entire target directory, including any existing `.env`. This bug was not caught during plan review or initial implementation.

**Impact:** Required a post-implementation bug fix that added backup/restore logic to Upload() (~170 lines of new code). Three new tests were added: `TestUploadSkipEnvPreservesRemoteEnv` with subtests for env-exists, no-env-needed, and env-not-on-remote. The feature's user-visible behavior was correct only after this fix.
**Source:** 07-VERIFICATION.md

---

### Phase 5 CheckResult struct designed for Phase 7 required no changes when Phase 7 arrived
The `CheckResult{Name, Status, Message}` struct defined in Phase 5 was usable directly by Phase 7's verbose preflight rendering (`[STATUS] name: message`). No modifications to the preflight package were needed.

**Impact:** This confirms the value of designing return types with future consumers in mind. The "discard with `_`" pattern in Phase 5 was a deliberate placeholder, and it paid off. The specific field names (Name, Status, Message) chosen in Phase 5 mapped directly to the verbose output format needed in Phase 7.
**Source:** 07-02-SUMMARY.md, 05-02-SUMMARY.md

---

### warnedOnce guard modification (verbose || !*warnedOnce) prevents per-verbose-run regression
The change to the sudoRunWithFallback `warnedOnce` guard required care: `if verbose || !*warnedOnce` with `*warnedOnce = true` only when `!verbose`. If the condition had been simplified incorrectly (e.g., `*warnedOnce = false` when verbose), the once-only suppression behavior in non-verbose mode would have broken.

**Impact:** A subtle two-state behavior: verbose=true → every warning prints, `*warnedOnce` never flips; verbose=false → first warning prints, `*warnedOnce` flips to true, subsequent warnings suppressed. The guard logic must handle both states simultaneously. The plan specified this exactly, preventing ambiguity during implementation.
**Source:** 07-02-PLAN.md, 07-02-SUMMARY.md
