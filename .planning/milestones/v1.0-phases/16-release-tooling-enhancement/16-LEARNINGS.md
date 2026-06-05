---
phase: 16
phase_name: "Release Tooling Enhancement"
project: "docker-deploy"
generated: "2026-06-05"
counts:
  decisions: 7
  lessons: 5
  patterns: 4
  surprises: 3
missing_artifacts:
  - "16-01-PLAN.md, 16-01-SUMMARY.md, 16-02-PLAN.md, 16-02-SUMMARY.md (Wave 2 terminal demo was explicitly skipped per D-14; plans 01 and 02 were never created)"
---

# Phase 16 Learnings: Release Tooling Enhancement

## Decisions

### Wave 0 pre-release checks run before any user-facing prompt or file change
The release skill was restructured so that `make test`, `make lint`, and `make test-ci` all execute and must pass before the version bump question is asked. No files are modified until all checks pass (D-01).

**Rationale:** Fail fast — there is no value in asking for a semver bump if the codebase is broken. Aborting before any interactive step also means the developer does not have to undo a partial release.
**Source:** 16-00-PLAN.md, 16-00-SUMMARY.md

---

### Lint failure triggers auto-fix then re-lint; only persistent failures abort
On a lint failure, `make lint-fix` is run automatically, then `make lint` is re-run. The release only aborts if the second lint run also fails (D-03).

**Rationale:** Many lint findings (import ordering, formatting) are auto-fixable. Aborting immediately on the first lint run would penalise developers for trivial, auto-correctable issues and add unnecessary friction to the release workflow.
**Source:** 16-00-PLAN.md, 16-00-SUMMARY.md

---

### Integration tests skip (with warning) when no Docker socket is detected; abort when Docker is present and tests fail
Docker socket presence is detected via `[ -S /var/run/docker.sock ] || [ -S $HOME/.colima/default/docker.sock ]`. Missing socket → non-blocking warning. Socket present but tests fail → hard abort (D-04).

**Rationale:** Integration tests require a running Docker daemon. Developers on CI or minimal environments should not be blocked from releasing, but developers with Docker available should not be able to release on a broken integration suite.
**Source:** 16-00-PLAN.md, 16-00-SUMMARY.md

---

### STATE.md last_updated and last_activity are updated in the same release commit
`last_updated` (ISO 8601 timestamp) and `last_activity` (`{YYYY-MM-DD} -- Released {NEXT_TAG}`) are mutated and staged alongside README.md and INSTALL.md in a single commit. `milestone:` is deliberately not changed (D-12).

**Rationale:** The release commit is the canonical record of what was released and when. Having STATE.md updated in the same commit means `git log` gives an accurate history of release events. `milestone:` tracks planning milestones (not semver), so it must not be auto-incremented.
**Source:** 16-00-PLAN.md, 16-00-SUMMARY.md

---

### Release commit body derived from git log with chore commits filtered out
`git log $PREV_TAG..HEAD --oneline` is run and lines starting with `chore:` or `chore(` are excluded. The filtered lines form the commit body as bullet points under "Changes since $CURRENT_TAG:". If all commits are chores, the commit uses subject line only (D-06, D-07, D-08, D-09).

**Rationale:** Release notes should surface user-visible changes. Chore commits (dependency bumps, config tweaks) add noise without informing users of what changed functionally.
**Source:** 16-00-PLAN.md, 16-00-SUMMARY.md

---

### All 12 Wave 3 linters were added in a single plan with all findings fixed in source
12 new linters (gosec, ineffassign, unused, bodyclose, noctx, gocritic, revive, errorlint, wrapcheck, gocognit, nestif, prealloc) were enabled in `.golangci.yml` and 129 surfaced findings were resolved in the same plan (16-03).

**Rationale:** Adding linters without immediately fixing their findings would leave the codebase in a permanently lint-failing state. The plan design required findings to be resolved before the plan closed, ensuring `make lint` passes from the moment the linters are enabled.
**Source:** 16-03-PLAN.md, 16-03-SUMMARY.md

---

### gocognit/nestif findings on established complex functions were suppressed with targeted nolint + reason
Functions like `Upload` (complexity 132), `SudoExec` (33), `RunCompose` (32), and `Resolve` (22) exceeded the complexity thresholds but were not refactored. Targeted `//nolint:gocognit` directives with explanations were added instead.

**Rationale:** Refactoring these architecturally-coupled functions would require threading shared state across multiple helpers and risked introducing regressions in battle-tested atomic upload and deploy logic. The suppression is intentional and documented inline.
**Source:** 16-03-PLAN.md, 16-03-SUMMARY.md

---

## Lessons

### 129 lint findings surface when enabling 12 new linters on an existing codebase
The existing codebase had 129 findings across gosec, wrapcheck, revive, noctx, unused, errorlint, and gocognit/nestif. Most (70) were gosec, followed by gocognit/nestif (22) and wrapcheck (17).

**Context:** Adding linters retroactively is expensive but one-time. The lesson is to add linters incrementally as a codebase grows, not all at once at a later phase. For this project, the tooling phase was Phase 16, so the batch approach was unavoidable.
**Source:** 16-03-SUMMARY.md

---

### gosec G304 false positives are common for user-home path expansion
File paths derived from `os.UserHomeDir()` and `os.Getwd()` triggered `G304` (file path from variable) findings in config.go, knownhosts.go, sshconfig.go, and upload.go. These were suppressed with `//nolint:gosec` and an explanation, since the path sources are controlled.

**Context:** gosec cannot statically prove that a variable-constructed file path is safe. Any path built from `os.UserHomeDir()` will trigger G304. Documenting the suppression reason makes it clear this is an intentional acceptance, not an oversight.
**Source:** 16-03-SUMMARY.md

---

### noctx findings required replacing net.Listen/net.Dial with context-aware equivalents in tests
`net.Listen("tcp", ...)` and `net.Dial("unix", ...)` (without context) triggered `noctx` findings in test helpers and in `ssh/client.go`. The fixes used `new(net.ListenConfig).Listen(ctx, ...)` and `(&net.Dialer{}).DialContext(...)`.

**Context:** The `noctx` linter enforces context propagation for all network operations. Even in tests and SSH agent dial paths, the context-aware forms are the correct API. These are non-trivial substitutions that require knowing the right stdlib types.
**Source:** 16-03-SUMMARY.md

---

### The skill file (release-tag.md) cannot be integration-tested automatically
The Wave 0 checks in `release-tag.md` could only be verified by code inspection during automated verification. Actual runtime behaviour (ordering of checks, abort paths, interactive prompts) required a live human UAT run.

**Context:** This is an inherent limitation of instruction files executed by an AI agent. Verification documented it explicitly and the single human UAT test passed on first execution.
**Source:** 16-VERIFICATION.md, 16-HUMAN-UAT.md

---

### Terminal demo recording (Wave 2) was explicitly deferred and left unassigned
The ROADMAP Phase 16 Wave 2 (terminal demo embedded in README.md, requirements SC-16-7, SC-16-8, SC-16-9) was skipped per D-14 and no later phase was assigned to cover it.

**Context:** The deferral was intentional — recording infrastructure requires VHS or asciinema setup that was out of scope. The decision to skip cleanly (rather than leave it in the backlog with a TBD status) keeps the roadmap honest about what was actually delivered.
**Source:** 16-VERIFICATION.md, 16-CONTEXT.md (referenced in VERIFICATION.md)

---

## Patterns

### Wave 0 gate pattern: ordered checks with abort-or-continue semantics
Structure pre-condition checks as a named wave (Wave 0) that runs before any interactive step or file mutation. Each check: print name with `▶` prefix, run, on failure print `ABORT:` and stop, on success print `PASS` and continue. After all checks print "All checks passed — proceeding."

**When to use:** Any multi-step workflow (release, deploy, migration) where precondition failures should prevent the user from reaching a point of no return.
**Source:** 16-00-PLAN.md, 16-00-SUMMARY.md

---

### Auto-fix retry gate for linting
Run lint. On failure: auto-fix, re-run lint. Abort only on second failure. Print `PASS (auto-fixed)` on recovery. This tolerates the common case of auto-fixable issues without requiring manual intervention.

**When to use:** Any lint gate in a workflow where the linter supports auto-fixing. The two-pass approach is strictly better than a one-pass abort since the fix is free.
**Source:** 16-00-PLAN.md, 16-00-SUMMARY.md

---

### Docker socket detection for conditional integration test execution
`[ -S /var/run/docker.sock ] || [ -S $HOME/.colima/default/docker.sock ]` covers both standard Docker Desktop and Colima on macOS. Skip with a warning if absent; run and abort on failure if present.

**When to use:** Any workflow that needs integration tests gated on infrastructure availability. The two-socket check covers the most common macOS developer setups; extend as needed for other container runtimes.
**Source:** 16-00-PLAN.md

---

### Targeted nolint with inline rationale for irreducibly complex functions
For functions that are architecturally coupled and battle-tested, add `//nolint:gocognit // <reason>` on the function declaration rather than refactoring. The reason must explain why refactoring carries more risk than the complexity.

**When to use:** Established functions with high cyclomatic complexity that are correct and well-tested, where decomposition would require threading shared state across helpers and risk regressions. Do not use for new code; reduce complexity in new code instead.
**Source:** 16-03-PLAN.md, 16-03-SUMMARY.md

---

## Surprises

### 70 of the 129 findings were from gosec — by far the largest category
gosec produced more findings than all other new linters combined. Most were G304 (file path from variable) and G104 (unchecked error in test cleanup), both of which are commonly accepted in Go codebases.

**Impact:** The gosec findings required the most decisions (suppress vs fix) and contributed most of the 18-minute plan duration. For future codebases, enabling gosec early and tuning its exclusions before the codebase grows will avoid this batch-processing cost.
**Source:** 16-03-SUMMARY.md

---

### wrapcheck required wrapping errors across the entire call stack in main.go
wrapcheck flagged 17 call sites where errors from external packages were returned without context wrapping. Most were in `main.go`: `config.LoadFile`, `config.Resolve`, `ssh.Dial`, `preflight.RunPreflightChecks`, `filetransfer.Upload`, `compose.RunCompose`, and `health.PollHealth`.

**Impact:** Adding `fmt.Errorf("context: %w", err)` wrappers improved error messages for users (errors now carry operation context). The finding also revealed that main.go was consistently not wrapping errors from package calls — a systemic pattern rather than isolated omissions.
**Source:** 16-03-SUMMARY.md

---

### Plan numbering has a gap: plans 16-01 and 16-02 do not exist
The phase has plans 16-00 and 16-03. Plans 16-01 and 16-02 were intended for Wave 1 (additional release tooling) and Wave 2 (terminal demo), both of which were deferred or skipped. The numbering gap reflects the original ROADMAP wave structure.

**Impact:** The gap in plan numbers is cosmetically confusing but functionally harmless. Future phases should consider whether to renumber or document the intent clearly in the CONTEXT file.
**Source:** Phase directory listing, 16-VERIFICATION.md
