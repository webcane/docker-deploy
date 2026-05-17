---
status: partial
phase: 04-core-deploy-loop
source: [04-01-SUMMARY.md, 04-02-SUMMARY.md, 04-03-SUMMARY.md]
started: 2026-05-16T00:00:00Z
updated: 2026-05-16T00:05:00Z
---

## Current Test

[testing paused — 3 items blocked by SSH dial bug]

## Tests

### 1. Cold Start Smoke Test
expected: Kill any running server/service. Clear ephemeral state. Build the plugin binary fresh (go build ./cmd/docker-deploy). Install to ~/.docker/cli-plugins/. Run docker deploy --help — command loads without errors, shows usage output.
result: pass

### 2. Full Deploy — Streaming Output
expected: From a project directory with a compose.yaml and files to upload, run docker deploy --host ssh://user@host. Files are copied first, then docker compose up -d --remove-orphans runs on the remote. Compose output streams live line-by-line. Plugin prints "Deploy complete: N files copied to host:/path" on success and exits 0.
result: issue
reported: "SSH connection failed: dial tcp 192.168.1.99:22: connect: no route to host — but ssh from terminal to same host works fine"
severity: blocker

### 3. --compose-file Flag Override
expected: Run docker deploy --host ssh://user@host --compose-file docker-compose.yml. Plugin uses docker-compose.yml (not compose.yaml) for the remote compose command — the exec command on the remote includes -f '<remotePath>'/docker-compose.yml.
result: blocked
blocked_by: prior-phase
reason: "SSH dial fails before compose flag can be exercised — blocked by test 2 SSH bug"

### 4. Compose File Auto-Detection
expected: From a directory containing compose.yaml (no --compose-file flag), plugin auto-detects compose.yaml and uses it for the remote compose command. If only docker-compose.yml exists (no compose.yaml), that is used instead.
result: blocked
blocked_by: prior-phase
reason: "SSH dial fails before compose auto-detection can be exercised — blocked by test 2 SSH bug"

### 5. No Compose File Found — Error Before SSH
expected: From a directory with no compose.yaml or docker-compose.yml, run docker deploy --host ssh://user@host. Plugin exits with an error "no compose file found; use --compose-file to specify one" before making any SSH connection.
result: pass

### 6. Exit Code Non-Zero on Compose Failure
expected: When docker compose up fails on the remote (e.g., invalid compose file), plugin prints "Deploy failed: docker compose exited with code N" to stderr and exits non-zero. The process exit code reflects the compose failure.
result: blocked
blocked_by: prior-phase
reason: "SSH dial fails before compose execution — blocked by test 2 SSH bug"

## Summary

total: 6
passed: 2
issues: 1
pending: 0
skipped: 0
blocked: 3
skipped: 0
blocked: 0

## Gaps

- truth: "Plugin connects to remote host via SSH and streams compose output"
  status: failed
  reason: "User reported: SSH connection failed: dial tcp 192.168.1.99:22: connect: no route to host — but ssh from terminal to same host works fine"
  severity: blocker
  test: 2
  root_cause: ""
  artifacts: []
  missing: []
  debug_session: ""
