# Phase 7: v2 — Leftovers - Discussion Log

> **Audit trail only.** Do not use as input to planning, research, or execution agents.
> Decisions are captured in CONTEXT.md — this log preserves the alternatives considered.

**Date:** 2026-05-20
**Phase:** 7-v2-skip-env-override
**Areas discussed:** --verbose scope, --verbose propagation, Re-include mechanism

---

## --verbose scope

### Q1: Should --verbose include pre-flight checklist rendering?

| Option | Description | Selected |
|--------|-------------|----------|
| Yes — show preflight checklist | Renders CheckResult slice Phase 5 already returns. Makes --verbose a full 'show me everything' flag. | ✓ |
| No — files + SSH commands only | Keeps --verbose scoped to transfer and execution only. | |

**User's choice:** Yes — show preflight checklist
**Notes:** Phase 5 explicitly deferred this to Phase 7. CheckResult slice is already returned and commented as awaiting verbose rendering.

---

### Q2: Should --verbose revert sudo warning dedup?

| Option | Description | Selected |
|--------|-------------|----------|
| Yes — show every sudo warning with --verbose | Consistent with quick task 260519-oax intent. warnedOnce dedup disabled when verbose. | |
| No — keep one warning even with --verbose | Dedup stays regardless. | |

**User's choice:** Custom response — "if --verbose flag given show all deduplicated warnings. without verbose flag show only one over all cases. 'WARN: there are some warnings during deployment. For more details use --verbose flag'"
**Notes:** User extended the design: without --verbose, ALL non-blocking warnings are suppressed and replaced with a single rollup message at end of deploy. With --verbose, all individual warnings print inline. The rollup message copy was specified exactly.

---

### Q3: Does the rollup apply to all warnings or just sudo?

| Option | Description | Selected |
|--------|-------------|----------|
| All suppressed warnings across the deploy | One rollup covers: sudo warnings, root-user warning, passwordless-sudo warning, and any other non-blocking warnings. | ✓ |
| Sudo warning only | Only the passwordless sudo warning from Upload() gets the rollup. | |

**User's choice:** All suppressed warnings across the deploy
**Notes:** Clean output by default; --verbose gives full detail. Applies to all non-blocking warnings, not just sudo.

---

### Q4: Where should --verbose output route?

| Option | Description | Selected |
|--------|-------------|----------|
| stderr for warnings + detail lines, stdout for deploy progress | Consistent with existing behavior. Scriptable — caller can redirect 2>/dev/null. | ✓ |
| All to stderr | Simpler routing but stdout-only parsers miss deploy progress. | |
| All to stdout | Simpler, but warning lines mix with compose output. | |

**User's choice:** stderr for warnings + detail lines, stdout for deploy progress

---

## --verbose propagation

### Q1: How should the verbose bool travel through the call stack?

| Option | Description | Selected |
|--------|-------------|----------|
| Add Verbose bool to Config struct | Follows Force/DryRun pattern. All call sites already receive resolved Config. No signature changes to packages. | ✓ |
| Pass verbose bool as explicit param to each function | More explicit. Higher friction — every call site needs updating. | |

**User's choice:** Add Verbose bool to Config struct

---

### Q2: Should Resolve() switch from positional params to a FlagOpts struct?

| Option | Description | Selected |
|--------|-------------|----------|
| Keep positional params — add flagSkipEnv + flagVerbose | Consistent with Phase 1-5 pattern. Slightly long (12 params) but explicit. | |
| Switch to FlagOpts struct for Resolve() | Resolve(opts FlagOpts, ...). Cleaner for future additions but a breaking change to callers and tests. | ✓ |

**User's choice:** Switch to FlagOpts struct for Resolve()
**Notes:** User explicitly chose the breaking refactor for long-term cleanliness. One-time change — no external API impact since Resolve() is internal.

---

### Q3: Where should FlagOpts struct live?

| Option | Description | Selected |
|--------|-------------|----------|
| config.FlagOpts in internal/config/config.go | Natural home — input side of Resolve(). Callers already import config. | ✓ |
| Inline struct at call site in main.go | Anonymous struct. Lighter, no new exported type. | |

**User's choice:** config.FlagOpts in internal/config/config.go

---

## Re-include mechanism

**User's decision:** No re-include mechanism at the moment.
**Notes:** SC-5 wording "silently skipped unless the user explicitly re-includes them" is aspirational. Phase 7 ships expanded defaults as immutable additions. Re-include deferred to a future phase if needed.

---

## Claude's Discretion

- Exact format of verbose file-transfer lines (e.g., `  -> relative/path` vs `[upload] relative/path`)
- Exact format of verbose SSH command lines (e.g., `[ssh] docker compose up -d → exit 0`)
- Whether `runDryRun` also respects `--verbose` for the SSH summary output
- Warning rollup accumulation strategy (slice vs counter)
- skip-env warning exact copy: `WARNING: .env not uploaded — remote .env left unchanged` (specified in discussion)

## Deferred Ideas

- **Re-include mechanism** (`--include` / `include:` key): explicitly out of scope for Phase 7.
- **Per-warning counts in rollup**: "3 warnings during deployment" vs generic "some warnings". Deferred to keep scope tight.
- **--verbose for --dry-run**: reasonable extension but deferred.
