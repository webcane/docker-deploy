---
plan: "12-04"
phase: "12"
status: complete
completed: 2026-05-24
---

# Plan 12-04 Summary: Add Feedback Section to COMPARISON.md

## What Was Built

Appended a "## Missing a tool?" section to the bottom of COMPARISON.md with a GitHub Issues link so users can suggest additions to the comparison table.

## Key Files

- `COMPARISON.md` — one new section added after "## When NOT to use docker-deploy"

## Verification

- `grep -c "## Missing a tool?" COMPARISON.md` → 1 ✓
- `grep -c "github.com/webcane/docker-deploy/issues" COMPARISON.md` → 1 ✓
- `grep -c "## When NOT to use docker-deploy" COMPARISON.md` → 1 ✓
- `grep -c "## When to use docker-deploy" COMPARISON.md` → 1 ✓
- `grep -c "Comparison Table" COMPARISON.md` → 1 ✓
- "Missing a tool?" at line 48, highest line number among all `## ` headings ✓

## Self-Check: PASSED
