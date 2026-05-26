---
title: Sudo only required when target path needs elevation
date: 2026-05-25
context: deploy loop / remote file operations
promoted: true
promoted_to: .planning/todos/pending/2026-05-26-sudo-path-aware-detection.md
---

Sudo is not universally required during deploy — it depends on the target path:

| Target path | Sudo needed |
|-------------|-------------|
| `/opt/<project>` (default) | Yes — root-owned directory |
| `~/<project>` or other user-writable path | No |

## Implication

Detect at deploy start whether the configured target path requires elevation. If not, skip all sudo scaffolding entirely. This makes deploys to home directories fully passwordless with no special configuration.

The check can be a simple remote probe: attempt to stat/write the target as the SSH user; if it fails with permission denied, fall back to sudo path.
