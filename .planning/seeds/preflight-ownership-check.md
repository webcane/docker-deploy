---
name: preflight-ownership-check
description: Replace sudo-based checkTargetDir with stat+id ownership introspection — preflight as diagnostics, not execution
trigger_condition: When working on preflight checks (Phase 5) or any cleanup of sudo logic in upload.go
planted_date: 2026-05-26
---

## Idea

Replace the current `checkTargetDir` implementation (which tries `sudo -n mkdir -p`) with pure
ownership introspection. Preflight should only **diagnose**, never execute with elevated privileges.

## Current behaviour (problematic)

`checkTargetDir` runs three side-effectful commands in order:
1. `test -w <path>` — ok
2. `mkdir -p <path> && test -w <path>` — creates dirs as side-effect
3. `sudo -n mkdir -p <path>` — runs sudo during preflight

This mixes "check" and "fix" responsibilities. Upload already handles the full sudo fallback
sequence (direct → sudo-n → interactive password), so preflight sudo is redundant.

## Proposed algorithm

```
1. stat -c "%u %g %a" <path>
   → if not found: walk up (stat parent, grandparent...) until existing ancestor found
2. id
   → parse uid, gid, all supplementary groups
3. Compare:
   - uid == stat.uid  → check owner write bit  (mode & 0200)
   - gid in groups    → check group write bit  (mode & 0020)
   - else             → check other write bit  (mode & 0002)
4. Report:
   - pass → path exists and is writable by current user
   - warn → "parent /opt owned by root:root (755) — Upload will need sudo"
```

No `mkdir`, no `sudo` in preflight. The message is diagnostic and actionable.

## Benefits

- No side effects during preflight
- Better error message: tells the user *why* sudo will be needed, not just *that* it will
- Removes the only `sudo` call from `checks.go`
- Single responsibility: preflight = read-only diagnosis

## Notes

- `stat -c` format works on Linux (remote VPS); macOS uses `stat -f` but remotes are Linux
- Walk up stops at `/` — if nothing writable found, warn and let Upload handle it
- `id` output format: `uid=1000(ubuntu) gid=1000(ubuntu) groups=1000(ubuntu),998(docker)` — parse with regex or split
