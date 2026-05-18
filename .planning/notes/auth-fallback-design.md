---
name: Auth fallback design decision
description: Passwordless sudo is preferred but not required; root is supported with a danger warning; interactive password prompt is the last fallback before failure
type: note
date: 2026-05-18
context: Discovered during Phase 4 UAT — CHECK-05 was blocking deploys where sudo password or root access would have worked fine
---

## Decision

Passwordless sudo is the **preferred** configuration but not a hard requirement for deployment to succeed.

The deploy command supports a structured fallback sequence rather than failing at preflight.

## Auth fallback sequence (normal SSH user)

1. Try file copy directly (no privilege escalation)
2. Try file copy with `sudo` (passwordless)
3. Prompt for sudo password interactively → warn that passwordless sudo isn't configured
4. Fail **only** when all paths exhausted (bad/no password, timeout)

## Root user handling

Root can copy directly — no sudo path needed. Always show a warning that deploying as root is dangerous. This mirrors CHECK-07 behavior.

## Why not block at preflight?

Preflight can only probe for capability, not guarantee execution. Blocking at CHECK-05 prevented valid deployments where:
- The user had sudo access requiring a password
- The SSH user was root (no sudo needed at all)

The real gate is deploy execution — fail there with a clear message if every auth path is exhausted.

## Warning text guidance

- Root user: `"WARNING: deploying as root — this is dangerous"`
- Passwordless sudo missing: `"WARNING: passwordless sudo not configured for <user>; you may be prompted for a password"`
- All paths exhausted: `"ERROR: could not write to target directory — no valid auth path available"`
