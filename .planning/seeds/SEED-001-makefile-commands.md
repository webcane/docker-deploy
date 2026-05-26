---
id: SEED-001
status: completed
planted: 2026-05-23
planted_during: v1.0 milestone (phase 9 shipped)
trigger_when: when relevant
scope: unknown
---

# SEED-001: makefile commands

## Why This Matters

_To be filled in. Run `/gsd-capture --seed --enrich SEED-001` to add context._

## When to Surface

**Trigger:** when relevant

This seed will surface during `/gsd-new-milestone` when the milestone scope matches.

## Scope Estimate

**Unknown** — run `/gsd-capture --seed --enrich SEED-001` to estimate effort.

## Breadcrumbs

- `Makefile` — existing build/install/test targets; lint and fmt targets are missing
- Add `golangci-lint` (optional, for linting) → `make lint`
- Add `goimports` (optional, for code formatting) → `make fmt`

## Notes

Add linting and formatting commands to Makefile:

* golangci-lint (optional, for linting)
* goimports (optional, for code formatting)

```makefile
make lint  # Run linter for Go code
make fmt   # Format Go code
```

Captured via one-shot seed capture. Enrich with trigger, why, and scope at your convenience.
