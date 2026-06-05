# Phase 5: Pre-flight & Health Polling - Discussion Log

> **Audit trail only.** Do not use as input to planning, research, or execution agents.
> Decisions are captured in CONTEXT.md — this log preserves the alternatives considered.

**Date:** 2026-05-16
**Phase:** 5-Pre-flight & Health Polling
**Areas discussed:** Pre-flight output UX, sudo mechanics, Health polling parameters, Pre-flight failure sequencing

---

## Pre-flight Output UX

| Option | Description | Selected |
|--------|-------------|----------|
| Live checklist (default) | Each check prints as it completes | |
| Silent on pass | No output when checks pass; only errors/warnings | ✓ |
| Summary table at end | Run all silently, then print a table | |

**User's choice:** Silent on pass by default; live checklist only when `--verbose` is used  
**Notes:** `--verbose` is a Phase 7 feature. Phase 5 builds silent-only. CHECK-07 root warning always prints regardless of verbosity. The check runner should return structured results so Phase 7 can render the live checklist without rewriting check logic.

---

## sudo Mechanics

| Option | Description | Selected |
|--------|-------------|----------|
| Assume passwordless sudo (Recommended) | Attempt sudo directly; print error + manual fix on failure | ✓ |
| Hard-fail with instructions | Don't attempt sudo; tell operator to fix manually | |
| Use -S flag (stdin password) | Prompt for password, pipe to sudo -S | |

**User's choice:** Assume passwordless sudo; on failure print clear error with the exact manual fix command  
**Notes:** SSH exec sessions have no interactive TTY for password entry. Documentation on how to configure passwordless sudo is a separate `.md` file deferred to Phase 7, linked from README.md.

---

## Health Polling Parameters

| Option | Description | Selected |
|--------|-------------|----------|
| 60s timeout / 5s interval | Generous but not indefinite | ✓ |
| 30s timeout / 3s interval | Faster, less forgiving | |
| Configurable via deploy.yaml | Maximum flexibility | (also selected) |

**User's choice:** 60s timeout / 5s interval, with both configurable via `deploy.yaml` keys (`health_timeout`, `health_interval`)  
**Notes:** User initially said "1 and 3" which was clarified via follow-up to mean 60s/5s with config support. Timeout expiry with container still in `starting` = non-zero exit (treat as failure, not a warning).

---

## Pre-flight Failure Sequencing

| Option | Description | Selected |
|--------|-------------|----------|
| Fail-fast on first blocking error (Recommended) | Stop and report on first hard block | ✓ |
| Run all checks, report everything | Collect all failures, report once | |

**User's choice:** Fail-fast on first blocking error  
**Notes:** User modified the check severity from the written requirements: CHECK-03 (daemon not running) is a warning only, not a block — files can still be copied even if daemon is down. Only CHECK-01 and CHECK-02 are hard blocks. CHECK-04 attempts auto-fix via sudo. CHECK-05 (sudo access) is checked only when a sudo attempt is actually needed. CHECK-06 tries mkdir first, escalates to sudo on EACCES.

---

## Claude's Discretion

- Which `docker inspect` format string produces health status across Docker versions
- Whether to enumerate containers by compose project label or `docker compose ps --format json`
- Error message wording (keep concise, actionable, consistent with Phase 4 patterns)
- Pre-flight check runner should return structured results (check name, status, message) to support Phase 7 verbose extension

## Deferred Ideas

- `--verbose` live checklist for pre-flight checks — Phase 7
- Passwordless sudo setup documentation (`.md` file, README link) — Phase 7
- `--pull always` / image pull control — Phase 7 v2 leftovers
