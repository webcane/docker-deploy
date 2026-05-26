---
created: 2026-05-26T06:31:18Z
title: Sudo path-aware detection
area: general
files:
  - internal/filetransfer/upload.go:219
  - internal/preflight/checks.go:219
---

## Problem

`sudoRunWithFallback` always tries a direct copy (Step 1) before escalating to sudo — even when the target path (e.g. `/opt/<project>`) is already known to require elevation. This results in a guaranteed-to-fail SSH command on every `sudoRunWithFallback` call (mkdir, mv, rm, cp) before falling through to the correct auth path.

## Solution

Probe once at deploy start whether the target path requires sudo (`test -w <path>`). Store the result as a `needsSudo bool` flag. When `needsSudo` is true, skip Step 1 (direct copy) in `sudoRunWithFallback` and go straight to passwordless sudo or interactive password. User-writable paths skip sudo entirely — no change to that path.

Folded into Phase 13 as 13-06-PLAN.md (touches `internal/filetransfer/upload.go` independently).
