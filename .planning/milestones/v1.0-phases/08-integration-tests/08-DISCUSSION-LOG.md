# Phase 8: Integration Tests - Discussion Log

> **Audit trail only.** Do not use as input to planning, research, or execution agents.
> Decisions are captured in CONTEXT.md — this log preserves the alternatives considered.

**Date:** 2026-05-21
**Phase:** 8-integration-tests
**Areas discussed:** Test coverage / use cases, Test suite location, Container topology, Atomicity failure simulation, Makefile & CI

---

## Test Coverage / Use Cases

| Option | Description | Selected |
|--------|-------------|----------|
| All 7 SC — full coverage | SSH connectivity, root-user warning, sudo check pass/fail, file copy atomicity, all CHECK-01–CHECK-07, health polling with and without HEALTHCHECK | ✓ |
| Critical path only | SSH dial, full deploy flow, at least one preflight pass/fail; skip individual CHECK coverage | |

**User's choice:** Full coverage — all 7 ROADMAP success criteria

---

| Option | Description | Selected |
|--------|-------------|----------|
| Real DinD container for compose/health tests | DinD container with SSH; compose/health tests run against a real Docker daemon | ✓ |
| Inject fake 'docker' binary into SSH container | Shell script named 'docker' returns expected output; no DinD complexity | |
| Skip compose/health in integration | Integration suite covers SSH + preflight only | |

**User's choice:** Real DinD container

---

| Option | Description | Selected |
|--------|-------------|----------|
| Real service (e.g., nginx or hello-world) | DinD container pulls and runs a real compose service | ✓ |
| Stub compose — test docker inspect only | Skip compose up; manually create fake container in DinD | |

**User's choice:** Real service running in DinD

---

| Option | Description | Selected |
|--------|-------------|----------|
| Yes — test both healthy and unhealthy | HEALTHCHECK that deliberately fails; verifies HEALTH-03 exit code | ✓ |
| No — healthy path only | Unhealthy scenario covered by unit tests | |

**User's choice:** Both healthy and unhealthy paths

---

| Option | Description | Selected |
|--------|-------------|----------|
| Yes — include --skip-env E2E test | Pre-seed .env on remote, re-deploy with --skip-env, assert .env unchanged | ✓ |
| No — skip-env covered by unit tests | filetransfer filter unit tests already cover this | |

**User's choice:** Include --skip-env E2E test

---

## Test Suite Location

| Option | Description | Selected |
|--------|-------------|----------|
| New top-level integration/ package | Matches ROADMAP SC-1 exactly; existing internal/ssh integration tests stay in place | ✓ |
| Expand internal/ssh + add integration/ for E2E | Keep per-package integration tests; add integration/ for cross-package flows | |
| Move all integration tests to integration/ | Consolidate all integration tests in one place; requires moving existing SSH tests | |

**User's choice:** New top-level integration/ package (existing internal/ssh tests stay)

---

| Option | Description | Selected |
|--------|-------------|----------|
| TestMain — shared containers | Containers started once; shared across all test functions | ✓ |
| Per-test containers | Each test starts/stops its own container; fully isolated but slow | |
| TestMain for SSH, per-test for DinD | SSH shared; DinD per test or test group | |

**User's choice:** TestMain with shared containers

---

| Option | Description | Selected |
|--------|-------------|----------|
| //go:build integration + package integration_test | Matches existing pattern; run with go test -tags integration | ✓ |
| //go:build integration + package integration | Allows testing internal helpers without export | |
| No build tag — t.Skip() when Docker unavailable | Simpler to discover; can accidentally run without containers ready | |

**User's choice:** `//go:build integration` + `package integration_test`

---

| Option | Description | Selected |
|--------|-------------|----------|
| Package-level API calls — call internal packages directly | Fine-grained assertions; no subprocess overhead | ✓ |
| Full CLI invocation via exec.Command | True black-box; harder to assert on internals | |
| Mix — package APIs for checks, CLI for E2E smoke tests | Package APIs for preflight/file copy/health; exec.Command for one E2E smoke test | |

**User's choice:** Package-level API calls (internal packages directly)

---

## Container Topology

| Option | Description | Selected |
|--------|-------------|----------|
| Two containers in TestMain | Container A: openssh-server; Container B: DinD+SSH custom image | ✓ |
| Single DinD container for everything | One container with OpenSSH + Docker; simpler topology but harder to configure | |
| Three containers: SSH-only, DinD, and root-user variant | Third container for CHECK-07 in isolation | |

**User's choice:** Two containers in TestMain

---

| Option | Description | Selected |
|--------|-------------|----------|
| sshuser with passwordless sudo pre-configured | Happy path for CHECK-04 / CHECK-06 auto-recovery | |
| Two users: one with sudo (pass), one without (fail) | Covers both pass and fail scenarios | |
| Four users (free text) | root, nosudouser, sudopassuser, sshuser (passwordless sudo) | ✓ |

**User's choice (free text):** 4 users — root, sshuser without sudo, sshuser with sudo + password, sshuser with passwordless sudo

---

| Option | Description | Selected |
|--------|-------------|----------|
| Custom Dockerfile in integration/ directory | Bakes all four users, sudo config, Docker socket; built via FromDockerfile | ✓ |
| Runtime setup via ExecInContainer | useradd/visudo commands at container start; no extra files | |
| Two separate images | openssh-server for simple tests; custom DinD+SSH only for complex tests | |

**User's choice:** Custom Dockerfile in integration/testdata/

---

## Atomicity Failure Simulation

| Option | Description | Selected |
|--------|-------------|----------|
| Cancel context mid-transfer | context.WithCancel that fires partway through Upload(); pure Go, no timing dependency | ✓ |
| Pause/disconnect SFTP via container exec | Kill sshd process mid-transfer; more realistic but timing-sensitive and flaky | |
| Test cleanup directly (not mid-transfer) | Manually create staging dir; verify cleanup logic directly | |

**User's choice:** Cancel context mid-transfer

---

| Option | Description | Selected |
|--------|-------------|----------|
| Target dir exists with original content, staging dir is gone | Pre-seed sentinel file; assert it survives cancelled upload; staging dir cleaned up | ✓ |
| Target dir does not exist (first-deploy scenario) | Start with no target dir; assert target not created after cancelled upload | |
| Both scenarios | Test both repeat-deploy and first-deploy paths | |

**User's choice:** Target dir with sentinel file (original content survives, staging dir cleaned up)

---

## Makefile & CI

| Option | Description | Selected |
|--------|-------------|----------|
| make test-integration as a separate target | 'make test' stays unit-only; CI runs both explicitly | ✓ |
| make test runs both | One command runs everything; slower on every run | |

**User's choice:** Separate `make test-integration` target

---

| Option | Description | Selected |
|--------|-------------|----------|
| Every push | Integration job runs after unit-test job on every push | |
| PRs and main branch only | | |
| PRs, main branch commits, and new tags (free text) | | ✓ |

**User's choice (free text):** PRs, main branch commits, and new tags

---

| Option | Description | Selected |
|--------|-------------|----------|
| 5 minutes per test run | -timeout 5m; enough for container startup + all tests | ✓ |
| 10 minutes | More headroom for flaky CI environments | |
| You decide | | |

**User's choice:** 5 minutes (`-timeout 5m`)

---

| Option | Description | Selected |
|--------|-------------|----------|
| Runner's native Docker socket — ubuntu-latest | No docker:dind service needed; testcontainers connects automatically | ✓ |
| Docker-in-Docker service in workflow | Required if runner doesn't have Docker | |

**User's choice:** Native Docker socket on ubuntu-latest

---

| Option | Description | Selected |
|--------|-------------|----------|
| Linux only (ubuntu-latest) | macOS runners don't have Docker pre-installed since 2024 | ✓ |
| Linux + macOS (with Docker via colima) | Much slower and complex; not recommended | |

**User's choice:** Linux only (`ubuntu-latest`)

---

## Claude's Discretion

- Exact compose.yaml content for the healthy service test (nginx:alpine vs hello-world vs custom minimal image)
- Exact compose.yaml content for the unhealthy service test (HEALTHCHECK instruction that reliably fails)
- File layout within `integration/` (one file per concern vs. one big file)
- Whether to copy SSH test helpers from `internal/ssh/client_test.go` or re-export them
- Exact number of files to transfer before context cancellation in the atomicity test
- Whether `sudopassuser` interactive TTY path (DEPLOY-07 fallback) is a full test or a stretch goal

## Deferred Ideas

- macOS CI runner support — requires colima setup; deferred unless macOS-specific behavior needs coverage
- Full CLI subprocess tests via `exec.Command` — not in scope for Phase 8
- Re-include mechanism (`--include` flag) — carried from Phase 7 deferred items; still not in scope
- Interactive sudo password prompt path (`sudopassuser`) — planner should assess difficulty; may be stretch goal
