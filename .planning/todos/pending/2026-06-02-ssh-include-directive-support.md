---
created: 2026-06-02T00:00:00Z
title: "Add SSH Include directive support in sshconfig.go"
area: general
files: [internal/sshconfig/sshconfig.go]
---

## Problem

`~/.ssh/config` files often use `Include` directives (e.g. Colima injects
`Include /Users/<user>/.colima/ssh_config`). The current `sshconfig.go` parser
does not follow `Include` lines, so hosts declared in included files are
invisible to `docker deploy`'s host alias resolution.

## Deferred from

Phase 14 (SSH Config Host Alias Resolution) — a TODO comment was left in
`sshconfig.go` noting the limitation and the recursive-file-open pattern.

## Solution

In `sshconfig.go`, when parsing a `Host` block scan, handle the `Include`
keyword:
- Expand globs in the Include path (tilde + glob expansion)
- Recursively parse each included file
- Guard against cycles (track visited paths)

Known affected use case: Colima-managed hosts (e.g. `colima` alias) are not
reachable via short name until Include support is implemented.
