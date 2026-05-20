# Phase 7: v2 — Leftovers - Context

**Gathered:** 2026-05-20
**Status:** Ready for planning

<domain>
## Phase Boundary

Phase 7 ships three contained quality-of-life improvements as a single wave:

1. **Expanded default excludes** — Add `.claude/`, `.github/`, `.planning/`, `.idea/`, `.vscode/`, `*.swp`, `*.swo`, `coverage/`, `dist/`, and `.terraform/` to the built-in `defaultExcludes` list. These are silently skipped on every deploy. No re-include mechanism is in scope for Phase 7.
2. **`--skip-env` / `skip_env`** — An opt-in flag that appends `.env` to the effective exclude list, leaving the remote `.env` untouched. Prints a visible warning when active.
3. **`--verbose` flag** — Enables full deploy detail output: each file transferred, each SSH command and its exit code, and the pre-flight checklist. Without `--verbose`, all non-blocking warnings are suppressed and a single rollup message is printed instead.

</domain>

<decisions>
## Implementation Decisions

### --verbose Scope

- **D-01:** `--verbose` enables four categories of additional output: (1) each file being transferred (one line per file, to stderr), (2) each SSH command executed and its exit code (to stderr), (3) the pre-flight checklist rendered from the `CheckResult` slice Phase 5 already returns (to stderr), (4) per-instance sudo warnings (disabling the `warnedOnce` dedup from quick task 260519-oax).
- **D-02:** Without `--verbose`, all non-blocking warnings across the deploy (sudo warnings, root-user warning, passwordless-sudo warning, any other non-blocking warnings) are suppressed. A single rollup message is printed at the end: `WARN: there are some warnings during deployment. For more details use --verbose flag`. All detail lines are withheld until `--verbose` is given.
- **D-03:** Output routing: warning rollup and all `--verbose` detail lines go to **stderr**. Normal deploy progress (`Uploading N files...`, compose output) stays on **stdout**. Scriptable — callers can redirect `2>/dev/null` to silence detail.

### --verbose Propagation

- **D-04:** `Verbose bool` is added to the `Config` struct. It is set in `Resolve()` via the `FlagOpts` struct (see D-05). All call sites that receive `resolved Config` can read `cfg.Verbose` without signature changes to individual packages (consistent with the `Force`/`DryRun` pattern).
- **D-05:** The `Resolve()` signature is refactored from positional params to a `config.FlagOpts` struct. The struct lives in `internal/config/config.go`. All callers (`runDeploy`, `runDryRun`) and all config tests are updated. This is a one-time breaking change on an internal function — no external API impact.

```go
// FlagOpts holds all CLI-flag values passed to Resolve().
type FlagOpts struct {
    Host           string
    Path           string
    Excludes       []string
    Force          bool
    ComposeFile    string
    HealthTimeout  int
    HealthInterval int
    SkipEnv        bool
    Verbose        bool
}
```

- **D-06:** `--verbose` is a persistent flag on the root `deploy` cobra command (same level as `--host`, `--force`, etc.). It is passed to `runDeploy()` and `runDryRun()` and flows into `FlagOpts.Verbose`.

### --skip-env / skip_env

- **D-07:** `SkipEnv bool` is added to `Config` and to `TargetConfig` with yaml tag `skip_env`. Precedence: `--skip-env` flag > `skip_env: true` in deploy.yaml > default false. Wired in `Resolve()` via `FlagOpts.SkipEnv || file.Target.SkipEnv`.
- **D-08:** When `SkipEnv` is true, `.env` is appended to the effective exclude list inside `mergeExcludes()` (or equivalently after it). The logic is additive — no other configured excludes are replaced.
- **D-09:** When `.env` is skipped, a warning is printed to stderr: `WARNING: .env not uploaded — remote .env left unchanged`. This warning is part of the warning rollup system (D-02): without `--verbose`, it contributes to the rollup; with `--verbose`, it prints inline.

### Expanded Default Excludes

- **D-10:** The following entries are appended to `defaultExcludes` in `internal/config/config.go`:
  ```go
  ".claude/", ".github/", ".planning/", ".idea/", ".vscode/",
  "*.swp", "*.swo", "coverage/", "dist/", ".terraform/",
  ```
- **D-11:** No re-include mechanism is added in Phase 7. The existing behavior stands: `defaultExcludes` is immutable; user excludes are additive only. SC-5's "unless the user explicitly re-includes them" is aspirational — deferred to a future phase.

### Claude's Discretion

- Exact format of verbose file-transfer lines (e.g., `  -> relative/path` vs `[upload] relative/path`).
- Exact format of verbose SSH command lines (e.g., `[ssh] docker compose up -d → exit 0`).
- Whether `runDryRun` also respects `--verbose` for the SSH summary output (reasonable to include).
- Warning rollup accumulation strategy: collect warnings in a `[]string` slice and print the rollup at the end of `runDeploy()`, or use a shared `warningCount int` and print the rollup based on count.

</decisions>

<canonical_refs>
## Canonical References

**Downstream agents MUST read these before planning or implementing.**

### Requirements and Roadmap
- `.planning/ROADMAP.md` §Phase 7 — goal, wave structure, success criteria (6 items across Wave 1 and Wave 2).
- `.planning/REQUIREMENTS.md` §FILES-02 — default exclude list baseline (what Phase 7 expands upon).

### Existing Config Implementation
- `internal/config/config.go` — `defaultExcludes` var, `TargetConfig` struct, `Config` struct, `Resolve()` function, `mergeExcludes()`. **Phase 7 modifies all of these** (new excludes, new fields, FlagOpts refactor).

### Existing Flag and Entry Points
- `cmd/docker-deploy/main.go` — `runDeploy()`, `runDryRun()`. Phase 7 adds `--skip-env` and `--verbose` flags here, updates Resolve() call sites to use `config.FlagOpts{}`.

### Existing Upload Function
- `internal/filetransfer/upload.go` — `Upload()` function with `warnedOnce *bool` param (quick task 260519-oax). Phase 7 connects `cfg.Verbose` to the `warnedOnce` behavior: when `cfg.Verbose` is true, `warnedOnce` is never flipped (every warning prints).

### Pre-flight CheckResult
- `internal/preflight/checks.go` — `CheckResult` slice returned by `RunPreflightChecks()`. Phase 5 comment in the file explicitly notes: "returned by RunPreflightChecks so that Phase 7 can render a verbose live checklist". Phase 7 renders this when `cfg.Verbose` is true.

### Prior Phase Context
- `.planning/phases/05-preflight-health-polling/05-CONTEXT.md` §D-03 — "Verbose checklist output (--verbose) deferred to Phase 7." Phase 7 fulfills this deferral.
- `.planning/phases/05-preflight-health-polling/05-04-PLAN.md` — Phase 5 wiring notes that `CheckResult` slice is discarded at call site in Phase 5, waiting for Phase 7.

### Rules
- `CLAUDE.md` — Rule 1 (no InsecureIgnoreHostKey), Rule 2 (SSH dial timeout pattern), Rule 5 (docker compose v2 only). No changes to these rules in Phase 7.

</canonical_refs>

<code_context>
## Existing Code Insights

### Reusable Assets
- `internal/config/config.go` `mergeExcludes()`: Phase 7 extends defaultExcludes directly — `mergeExcludes()` itself needs no logic change for the expanded list, just the source slice grows.
- `internal/filetransfer/upload.go` `warnedOnce *bool`: Already plumbed for verbose behavior. Phase 7 wires `cfg.Verbose` to control whether `warnedOnce` stays false (verbose) or flips to true after first warning (default).
- `internal/preflight/checks.go` `CheckResult` slice: Already returned by `RunPreflightChecks()` and explicitly commented as awaiting Phase 7 verbose rendering. No changes to preflight logic needed.

### Established Patterns
- **Config struct as the verbose carrier**: `Force bool`, `DryRun bool` precedents — `Verbose bool` follows the same pattern. No new patterns needed.
- **stderr for warnings**: All existing warning output (root user, docker group, passwordless sudo) goes to `os.Stderr`. Phase 7 is consistent.
- **Additive excludes**: `mergeExcludes()` deduplicates on insertion. Adding `.env` via SkipEnv and appending to defaultExcludes both follow the same dedup logic.

### Integration Points
- `runDeploy()` in `main.go`: New `--skip-env` and `--verbose` flags registered here. `FlagOpts{}` struct populated here and passed to `Resolve()`. Warning rollup accumulation and final print happen here.
- `filetransfer.Upload()` call site: `warnedOnce` behavior conditioned on `cfg.Verbose`.
- `preflight.RunPreflightChecks()` call site: `CheckResult` slice captured and rendered when `cfg.Verbose` is true.

</code_context>

<specifics>
## Specific Ideas

- **Warning rollup message copy**: `WARN: there are some warnings during deployment. For more details use --verbose flag` — exact copy per user discussion.
- **skip-env warning copy**: `WARNING: .env not uploaded — remote .env left unchanged` — print to stderr.
- **No re-include mechanism**: SC-5's "unless the user explicitly re-includes them" is aspirational for a future phase. Phase 7 ships the expanded defaults as immutable additions only.
- **FlagOpts struct refactor**: User explicitly chose this over adding more positional params to `Resolve()`. It's a one-time breaking change to the internal config package — all callers and tests must be updated.

</specifics>

<deferred>
## Deferred Ideas

- **Re-include mechanism** (`--include` flag or `include:` yaml key): SC-5 mentions it but it is explicitly NOT in scope for Phase 7. Implement in a future phase if user feedback shows `.github/` or similar are needed on deploy.
- **Per-warning counts in rollup**: E.g., "3 warnings during deployment" vs generic "some warnings". Could be added to the rollup message — deferred to keep Phase 7 scope tight.
- **`--verbose` for `--dry-run`**: Reasonable extension (verbose SSH summary in dry-run). Deferred — planner can include it if low effort, otherwise Phase 8+.

</deferred>

---

*Phase: 7-v2-skip-env-override*
*Context gathered: 2026-05-20*
