---
name: Better error when deploy.yaml exists but produces no host
description: Improve the "no host configured" error to distinguish file-not-found from file-found-but-empty-host
type: seed
trigger_condition: Phase 7 (Leftovers) or when --verbose flag is added
planted_date: 2026-05-17
---

Currently the error `no host configured: use --host flag or set target.host in deploy.yaml`
is identical whether:
- No `deploy.yaml` exists at all
- `deploy.yaml` exists but `target.host` is empty or malformed

This makes debugging hard — the user sees the same message regardless.

**Proposed improvement:** In `runDeploy` / `runDryRun`, after `config.LoadFile()` succeeds
but `resolved.Host.Hostname == ""`, check whether a deploy.yaml was actually present:

```go
if fileConfig != (config.FileConfig{}) && resolved.Host.Hostname == "" {
    return fmt.Errorf("deploy.yaml found but target.host is not set or invalid; " +
        "use --host flag or add target.host: ssh://user@host:port to deploy.yaml")
}
```

**Why:** Reduces confusion when a user has a deploy.yaml with a structural issue (wrong
indentation, missing target: wrapper, invalid ssh:// format) and can't tell if the file
is even being read.

**How to apply:** Wire this into --verbose mode output or always-on; consider adding
a `--debug-config` dry-run variant that prints the raw parsed FileConfig.
