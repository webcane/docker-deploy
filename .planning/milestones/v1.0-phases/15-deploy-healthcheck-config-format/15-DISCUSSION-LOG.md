# Phase 15: Deploy Healthcheck Config Format - Discussion Log

> **Audit trail only.** Do not use as input to planning, research, or execution agents.
> Decisions are captured in CONTEXT.md — this log preserves the alternatives considered.

**Date:** 2026-05-30
**Phase:** 15-deploy-healthcheck-config-format
**Areas discussed:** YAML structure, Service mismatch handling, Per-service capabilities

---

## YAML Structure

| Option | Description | Selected |
|--------|-------------|----------|
| Under target.health_services map | `target.health_services.web.timeout: 30` | |
| Under target.services with full service config | `target.services.web.health_timeout: 30` | (initial) |
| Flat keys with service prefix | `target.health_timeouts.web: 30` | |

**User's choice:** Evolved to a global `target.healthcheck:` block (not per-service). User specified the structure should mirror Docker Compose HEALTHCHECK syntax with `interval`, `timeout`, `retries` sub-keys.

**Notes:**
- Duration format: Docker-style strings (`10s`, `1m30s`) — mirrors docker-compose.yaml syntax
- Retries: max consecutive unhealthy results before failure (matches Docker HEALTHCHECK semantics), default 3
- Old flat keys (`health_timeout`, `health_interval`) are hard-removed; deprecation warning emitted if found in deploy.yaml

---

## Service Mismatch Handling

| Option | Description | Selected |
|--------|-------------|----------|
| Warning to stderr, continue | Print warning for unmatched service names | |
| Hard error, abort | Fail immediately on mismatch | |
| Silently ignore | No feedback on unknown service names | |

**User's choice:** N/A — scope clarified to global-only healthcheck (no per-service config map). Container enumeration continues using `com.docker.compose.project` label as in Phase 5.

**Notes:** User confirmed the existing Phase 5 approach (enumerate by project label, poll all containers with shared settings) is correct. No per-service config map is needed.

---

## Per-service Capabilities

| Option | Description | Selected |
|--------|-------------|----------|
| No — existing no-HEALTHCHECK handling is sufficient | Warning + pass for containers without HEALTHCHECK | ✓ |
| Yes — add disable flag per service | Skip polling for specific containers | |

**User's choice:** No per-service override needed. Phase 5 already handles containers without HEALTHCHECK (warn and pass). Default retries: 3.

**Notes:** Retries counter is per-container; one container hitting retry limit doesn't affect others.

---

## Backward Compatibility

| Option | Description | Selected |
|--------|-------------|----------|
| Hard remove with deprecation warning | Old keys ignored with stderr message | ✓ |
| Hard remove, no warning | Silent clean break | |
| Keep old keys as fallback | Both formats accepted, new takes precedence | |

**User's choice:** Hard remove with deprecation warning. yaml.v3 silently ignores unknown fields, so explicit detection + warning is required.

---

## CLI Flags

| Option | Description | Selected |
|--------|-------------|----------|
| --healthcheck-timeout, --healthcheck-interval, --healthcheck-retries | Duration strings + integer | ✓ |
| No new flags — deploy.yaml only | Config file only | |

**User's choice:** All three flags added. Duration string format (`--healthcheck-timeout 30s`).

---

## Claude's Discretion

- Whether to detect deprecated keys via a second yaml.Node scan or a separate intermediate struct
- Whether to store `Healthcheck` as a nested struct on `Config` or flatten fields back onto `Config`
- Error message wording for invalid duration strings

## Deferred Ideas

- Per-service healthcheck overrides (`target.services.{name}.healthcheck`)
- `disable: true` to skip health polling entirely
