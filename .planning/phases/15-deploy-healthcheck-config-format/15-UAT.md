---
status: complete
phase: 15-deploy-healthcheck-config-format
source: [15-01-SUMMARY.md, 15-02-SUMMARY.md]
started: 2026-05-31T00:00:00Z
updated: 2026-05-31T12:00:00Z
---

## Current Test

[testing complete]

## Tests

### 1. New healthcheck CLI flags visible in help
expected: Run `go run ./cmd/docker-deploy deploy --help` and see --healthcheck-timeout, --healthcheck-interval, and --healthcheck-retries flags listed with Docker-style duration descriptions.
result: pass

### 2. New YAML healthcheck block accepted
expected: Create a deploy.yaml with a structured `healthcheck:` block — e.g. `target: {healthcheck: {interval: "30s", timeout: "10s", retries: 3}}` — and run `docker deploy validate` (or `--dry-run`). Should parse without error and reflect the values.
result: issue
reported: "old config (health_timeout/health_interval) validates without any warnings. New config with typo `retrise: 3` also silently accepted — retries would be 0 with no user feedback"
severity: major

### 3. Invalid duration string rejected with clear error
expected: Run `go run ./cmd/docker-deploy deploy --healthcheck-interval=not-a-duration --host ssh://user@host`. Should print a clear error such as "invalid duration 'not-a-duration'" referencing the source (--healthcheck-interval), not a panic or cryptic message.
result: pass

### 4. Old flat YAML keys silently ignored (no crash)
expected: Create a deploy.yaml with old-style `health_timeout: 30` and `health_interval: 10` keys. Running `--dry-run` or `validate` should not crash — yaml.v3 silently ignores unknown fields. The deploy proceeds (or dry-run completes) without a fatal error about unrecognised keys.
result: pass

### 5. CLI flag overrides deploy.yaml healthcheck
expected: Set `healthcheck: {timeout: "60s"}` in deploy.yaml, then run with `--healthcheck-timeout=5s`. The resolved config (visible via `--verbose --dry-run`) should show 5s, not 60s — flag takes precedence over file.
result: issue
reported: "--dry-run with --verbose does not include resolved healthcheck values in output; only SSH connectivity info shown — impossible to verify flag/file precedence without a real deploy"
severity: minor

### 6. Retries: single unhealthy result does not abort deploy (requires live containers)
expected: With `retries: 2` configured, a container that returns unhealthy once then becomes healthy should NOT abort the deployment. The deploy should continue polling. If you can't test with live containers, skip this.
result: blocked
blocked_by: other
reason: "Timeout path fired (2s) before retries semantics could be observed; timeout and retries are independent controls — needs a container cycling through unhealthy→healthy states"

### 7. Retries: N consecutive unhealthy aborts with count in message
expected: With `retries: 2`, a container that stays unhealthy for 2+ consecutive polls should cause the deploy to fail with a message mentioning "(2 consecutive unhealthy results)". If you can't test with live containers, skip this.
result: pass

## Summary

total: 7
passed: 4
issues: 2
pending: 0

skipped: 0
blocked: 1

## Gaps

- truth: "YAML healthcheck field typos (e.g. 'retrise' instead of 'retries') should produce a warning — silently ignoring them leaves retries at 0 with no user feedback"
  status: failed
  reason: "User reported: old config (health_timeout/health_interval) validates without any warnings. New config with typo `retrise: 3` also silently accepted — retries would be 0 with no user feedback"
  severity: major
  test: 2
  root_cause: ""
  artifacts: []
  missing: []
  debug_session: ""

- truth: "--dry-run --verbose should display resolved healthcheck config so users can verify flag/file precedence"
  status: failed
  reason: "User reported: --dry-run with --verbose does not include resolved healthcheck values in output; only SSH connectivity info shown"
  severity: minor
  test: 5
  root_cause: ""
  artifacts: []
  missing: []
  debug_session: ""
