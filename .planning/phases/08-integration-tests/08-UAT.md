---
status: complete
phase: 08-integration-tests
source: [08-01-SUMMARY.md, 08-02-SUMMARY.md, 08-03-SUMMARY.md, 08-04-SUMMARY.md, 08-05-SUMMARY.md, 08-06-SUMMARY.md]
started: 2026-05-22T00:00:00Z
updated: 2026-05-22T00:10:00Z
---

## Current Test

[testing complete]

## Tests

### 1. Cold Start Smoke Test
expected: Kill any running containers. Run `make test-integration` from scratch (no pre-running SSH/Docker containers). The command starts testcontainers, builds the DinD+SSH image, runs the full suite, and exits 0. No manual container setup required.
result: pass

### 2. Full suite passes end-to-end
expected: `make test-integration` output ends with `ok github.com/webcane/docker-deploy/integration` and a duration (e.g., 60s). All tests pass or TestUpload_AtomicCancel is skipped with "cancel fired too late" on fast hardware. Exit code 0.
result: pass

### 3. SSH connectivity tests
expected: TestDial_Timeout, TestDial_UnknownHost_TOFU, TestDial_UnknownHost_TOFU_Accepted, and TestDial_Success all pass. TOFU acceptance writes the host key to known_hosts. No InsecureIgnoreHostKey is used anywhere.
result: pass

### 4. Preflight checks — pass and fail paths
expected: All 13 preflight test functions pass: every CHECK-01 through CHECK-07 has both a pass and a fail scenario covered. Root warning (CHECK-07 as root) is non-blocking: err == nil and stderr contains "root". nosudouser paths for docker-group and target-dir return non-pass (warn or fail), not an error.
result: pass

### 5. File transfer atomicity (SC-5)
expected: TestUpload_AtomicCancel: context cancelled mid-transfer leaves sentinel-before-deploy.txt intact with content "original"; no /tmp/docker-deploy-* staging directory remains after cancellation. TestUpload_HappyPath returns nil error with n > 0. TestUpload_SkipEnv leaves the pre-seeded .env unchanged after re-deploy.
result: pass

### 6. Compose + health polling tests
expected: TestCompose_Healthy_NoHealthcheck: nginx:alpine reaches "running" state, PollHealth returns nil. TestCompose_Unhealthy_ReturnError: busybox exits immediately, PollHealth returns non-nil error containing "stopped unexpectedly". TestHealth_NoContainers returns nil immediately (empty project).
result: pass

### 7. CI integration job wired
expected: `.github/workflows/ci.yml` contains an `integration` job that runs after the `test` job (`needs: [test]`) on `ubuntu-latest`. Version tags (`v*`) trigger both jobs. No docker:dind service block — native Docker socket is used.
result: pass

## Summary

total: 7
passed: 7
issues: 0
pending: 0
skipped: 0
blocked: 0

## Gaps

[none yet]
