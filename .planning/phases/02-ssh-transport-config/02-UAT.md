---
status: complete
phase: 02-ssh-transport-config
source: [02-01-SUMMARY.md, 02-02-SUMMARY.md, 02-03-SUMMARY.md]
started: 2026-05-14T11:00:00Z
updated: 2026-05-14T11:00:00Z
---

## Current Test

[testing complete]

## Tests

### 1. Successful dry-run against a real SSH host
expected: Run `docker deploy --host ssh://sshuser@192.168.1.99 --dry-run`. Prints Host, Remote path, Auth method, Server version, Status: OK — exits 0.
result: pass

### 2. --help shows --host, --path, --dry-run flags
expected: Run `docker deploy --help`. The output lists three new flags with descriptions: --host (ssh://user@host:port format), --path (remote target directory), --dry-run (verify SSH connectivity).
result: pass

### 3. No host configured → actionable error
expected: Run `docker deploy --dry-run` (no --host, no deploy.yaml). Error message says something like "no host configured: use --host flag or set target.host in deploy.yaml". Exits non-zero.
result: pass

### 4. Flag overrides deploy.yaml
expected: Create a deploy.yaml with `version: 1\ntarget:\n  host: ssh://user@host1:22`. Run `docker deploy --host ssh://user@host2:22 --dry-run`. The resolved output shows host2, not host1.
result: pass

### 5. deploy.yaml loaded when no flag set
expected: Create a deploy.yaml with your real host (`version: 1\ntarget:\n  host: ssh://sshuser@192.168.1.99`). Run `docker deploy --dry-run` (no --host flag). Connects successfully using the file value.
result: issue
reported: "no host configured: use --host flag or set target.host in deploy.yaml — deploy.yaml not being read"
severity: major

### 6. Invalid --host scheme rejected
expected: Run `docker deploy --host http://user@host:22 --dry-run`. Returns an error about invalid scheme or format (not an SSH panic/crash). Exits non-zero.
result: pass

### 7. TOFU prompt on first connection to unknown host
expected: Remove the known_hosts entry (`ssh-keygen -R 192.168.1.99`), then run `docker deploy --host ssh://sshuser@192.168.1.99 --dry-run`. A fingerprint is printed and you're prompted to confirm [yes/no]. Typing "yes" connects successfully and appends the host to known_hosts.
result: pass

### 8. TOFU rejected → exits non-zero
expected: Same as above — at the yes/no prompt, type "no". The command exits non-zero with a message about the connection being rejected. known_hosts is not modified.
result: pass

### 9. Timeout against non-responsive host
expected: Run `docker deploy --host ssh://user@192.0.2.1 --dry-run`. Within approximately 10 seconds, returns an error containing "timed out" or "deadline exceeded". Exits non-zero.
result: pass

## Summary

total: 9
passed: 8
issues: 1
pending: 0
skipped: 0
blocked: 0

## Gaps

- truth: "deploy.yaml in cwd is loaded when present; fields populate config when not overridden by flags"
  status: failed
  reason: "User reported: no host configured: use --host flag or set target.host in deploy.yaml — deploy.yaml not being read"
  severity: major
  test: 5
  root_cause: ""
  artifacts: []
  missing: []
  debug_session: ""
