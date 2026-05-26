---
title: "Add ssh_dial_timeout to TargetConfig"
date: 2026-05-26
priority: medium
context: global-defaults-config
---

`sshDialTimeout` is currently a compile-time constant in `cmd/docker-deploy/main.go:31`.
It needs to become a configurable field so users can tune it via `deploy.yaml`.

## Steps

1. Add `SSHDialTimeout int` field (seconds) to `TargetConfig` in `internal/config/config.go`
2. Add resolution in `config.Resolve()`: `flag (future) > deploy.yaml > global deploy.yaml > default 10`
3. Thread the resolved value through to the SSH dial call in `main.go` (replaces `sshDialTimeout` const)
4. Remove the `const sshDialTimeout` once the field is wired

## Notes

- Keep the const as the built-in fallback value during transition, then delete it
- `ssh_dial_timeout` key in YAML, value in seconds (integer), matches `health_timeout` / `health_interval` pattern
