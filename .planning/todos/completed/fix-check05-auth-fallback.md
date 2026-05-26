---
name: Fix CHECK-05 — downgrade to warning and implement fallback auth sequence
description: CHECK-05 currently blocks deployment when passwordless sudo is not configured; it should warn and let deploy proceed via fallback auth paths
type: todo
priority: high
date: 2026-05-18
---

## Status

Already implemented (verified 2026-05-26):
- `internal/preflight/checks.go:252–262` — CHECK-05 returns `"warn"`, never blocks
- `internal/filetransfer/upload.go:219–288` — `sudoRunWithFallback` covers all 3 fallback steps

## Task

Downgrade CHECK-05 from a blocking error to a warning, and implement a structured auth fallback sequence in deploy execution.

### Preflight (CHECK-05)

- Change from hard fail to warning: `"passwordless sudo not configured — will attempt interactive fallback during deploy"`
- Never block deployment at preflight stage based on sudo configuration alone

### Deploy execution — fallback auth sequence

**If SSH user is root:**
- Copy directly (no sudo required)
- Print warning: deploying as root is dangerous (CHECK-07 already covers this)

**If normal SSH user:**
1. Try copy to target directly
2. Fallback: try copy with passwordless sudo
3. Fallback: prompt user for sudo password interactively; warn that passwordless sudo is not configured
4. Fail deployment only if: no password entered, wrong password, or prompt timeout

### Files likely affected

- `internal/preflight/` — remove blocking behavior from CHECK-05
- `internal/deploy/` or `internal/ssh/` — add fallback copy logic with sudo and interactive password prompt
