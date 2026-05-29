---
created: 2026-05-29T00:00:00Z
title: Implement OpenSSH %-token expansion in expandPath
area: sshconfig
files:
  - internal/sshconfig/sshconfig.go:181
  - internal/sshconfig/sshconfig_test.go
---

## Problem

`expandPath` in `internal/sshconfig/sshconfig.go` only handled `~/` prefix expansion.
Users with `IdentityFile %d/.ssh/id_%r` (or any OpenSSH %-token) in `~/.ssh/config`
got the literal string passed to `os.ReadFile`, which failed silently — the key was
skipped and the user saw "no keys available" with no explanation.

## Solution

Updated `expandPath` signature to accept `homeDir`, `localUser`, `hostname`,
`remoteUser`, and `port` context parameters. Expands tokens `%d`, `%u`, `%h`, `%r`,
`%p`, and `%%` via `strings.NewReplacer` in addition to `~/` prefix expansion.

`LookupHost` now collects raw `IdentityFile` values in `rawIdentityFiles []string`
during the scan loop, then expands them in a post-loop pass after `entry.HostName`
(with D-07 alias fallback applied) and `entry.User` are fully resolved — so `%h`
and `%r` always see final values regardless of directive order in the config block.

Committed: `1f7cebc` — `feat(sshconfig): expand OpenSSH % tokens in IdentityFile paths`
