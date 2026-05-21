# Phase 8: Integration Tests - Context

**Gathered:** 2026-05-21
**Status:** Ready for planning

<domain>
## Phase Boundary

Build a testcontainers-based integration test suite at `integration/` that verifies all v1 requirements against real SSH daemons and a real Docker daemon — no manual VPS access required. `go test -tags integration -timeout 5m ./integration/...` runs the full suite.

**In scope:**
- SSH connectivity verification (knownhosts, TOFU, timeout, auth chain) — SC-2
- Root-user warning (CHECK-07) triggered and asserted — SC-3
- Passwordless-sudo permission check pass/fail (CHECK-04, CHECK-06) — SC-4
- File copy atomicity: context-cancelled mid-copy leaves target in pre-deploy state — SC-5
- All preflight checks CHECK-01 through CHECK-07, at least one pass + one fail scenario each — SC-6
- Health polling: with and without HEALTHCHECK defined; healthy and unhealthy paths — SC-7
- `--skip-env` end-to-end: re-deploy with --skip-env leaves remote .env unchanged
- Full compose E2E: real compose.yaml with a real service (nginx or hello-world) running in DinD

**Out of scope:**
- Testing the `--init` wizard (Phase 6, not yet implemented)
- macOS CI runners
- End-to-end CLI subprocess testing (exec.Command) — internal package API tests only
- Re-include mechanism for excluded files

</domain>

<decisions>
## Implementation Decisions

### Test Coverage and Use Cases

- **D-01:** Full coverage target: all 7 ROADMAP success criteria are covered. No "unit tests cover it" shortcuts for things SC requires in integration.
- **D-02:** Compose and health tests use a real service running in DinD (e.g., `nginx:alpine` or `hello-world`). No stubbed `docker compose` output.
- **D-03:** Both HEALTHY and UNHEALTHY paths are tested. Unhealthy scenario: a compose.yaml with a HEALTHCHECK that deliberately fails (e.g., `curl` to a closed port). Verifies HEALTH-03 exit code propagation.
- **D-04:** `--skip-env` is tested end-to-end: pre-seed a `.env` on the remote, re-deploy with `--skip-env`, assert the remote `.env` content is unchanged.

### Test Suite Location

- **D-05:** New top-level `integration/` package. Matches ROADMAP SC-1 exactly. Existing `internal/ssh/client_test.go` integration tests stay in place — they are not moved.
- **D-06:** Build tag: `//go:build integration` at the top of every file in `integration/`. Package declaration: `package integration_test`. Consistent with the existing pattern in `internal/ssh/client_test.go`.
- **D-07:** `TestMain` starts all containers once and shares them across all test functions. Containers are not started per-test. Tests must not leave dirty state that breaks subsequent tests.
- **D-08:** Tests call internal package APIs directly (e.g., `internalssh.Dial()`, `preflight.RunPreflightChecks()`, `filetransfer.Upload()`). No full CLI invocation via `exec.Command`. Fine-grained assertions without subprocess overhead.

### Container Topology

- **D-09:** Two containers started in `TestMain`:
  - **Container A** (SSH-only): `lscr.io/linuxserver/openssh-server:latest` — for SSH dial tests (TestDial_*) and preflight checks that don't need Docker.
  - **Container B** (DinD+SSH): a custom `Dockerfile` in `integration/testdata/` that has both OpenSSH daemon and Docker daemon — for full preflight (CHECK-01, CHECK-02, CHECK-04, CHECK-06), compose, and health tests.
- **D-10:** Four users configured in the custom Dockerfile (Container B):
  1. `root` — for CHECK-07 (root-user warning) test
  2. `nosudouser` — user without sudo; tests the failing path for CHECK-04 / CHECK-06 (clear error printed)
  3. `sudopassuser` — user with sudo requiring a password; tests the sudo-with-password path (DEPLOY-07 fallback)
  4. `sshuser` — user with passwordless sudo (`NOPASSWD` in sudoers); tests the happy path for CHECK-04 / CHECK-06 auto-recovery
- **D-11:** The custom Dockerfile is committed in `integration/testdata/Dockerfile.sshd` (or similar). testcontainers uses `testcontainers.FromDockerfile` to build it once per test run.

### Atomicity Failure Simulation

- **D-12:** Mid-copy failure is triggered by cancelling the Go context passed to `Upload()` partway through the transfer.
- **D-13:** Test assertion after cancelled upload:
  - Pre-deploy: create target dir with a sentinel file (`original.txt`).
  - Upload with a cancelled context mid-transfer.
  - Assert: sentinel file is still present (original content intact), no `/tmp/docker-deploy-*` staging dir remains.
  - Proves: atomic move did not happen, cleanup ran.

### Makefile & CI

- **D-14:** New Makefile target: `test-integration: go test -tags integration -timeout 5m ./integration/...`. The existing `make test` (unit tests only) is unchanged.
- **D-15:** GitHub Actions: integration tests run on PRs, commits to `main`, and new tags. Not on every feature branch push. New job named `integration` that runs on `ubuntu-latest` after the unit-test job passes.
- **D-16:** CI runner: GitHub-hosted `ubuntu-latest` with its native Docker socket. No `docker:dind` service needed in the workflow — testcontainers-go connects to the pre-installed Docker daemon on the runner automatically.
- **D-17:** Integration test timeout: `-timeout 5m` (5 minutes per run). Sufficient for container startup (~30-90s for SSH, ~60s for DinD) plus all tests.

### Claude's Discretion

- Exact compose.yaml content for the healthy service test (nginx:alpine vs hello-world vs a custom minimal image).
- Exact compose.yaml content for the unhealthy service test (HEALTHCHECK instruction that reliably fails within the polling window).
- File layout within `integration/` (one file per concern vs. one big file).
- Helper utilities to share across integration test files (e.g., `startSSHContainer`, `captureHostKey`, `seedKnownHosts` — already exist in `internal/ssh/client_test.go`; decide whether to copy them to `integration/` or re-export).
- Exact number of files to transfer before context cancellation in the atomicity test.

</decisions>

<canonical_refs>
## Canonical References

**Downstream agents MUST read these before planning or implementing.**

### Requirements and Roadmap
- `.planning/ROADMAP.md` §Phase 8 — goal and 7 success criteria (the integration suite must satisfy all 7)
- `.planning/REQUIREMENTS.md` — CHECK-01 through CHECK-07, HEALTH-01 through HEALTH-03, DEPLOY-02, DEPLOY-03, DEPLOY-07; these are the requirements the tests verify

### Existing Integration Test Patterns (MUST read before writing new tests)
- `internal/ssh/client_test.go` — existing `//go:build integration` test using testcontainers; `startSSHContainer()`, `captureHostKey()`, `seedKnownHosts()`, `emptyKnownHosts()`, `generateTestKeyFile()` helpers; container image and env vars; pattern for TOFU / timeout / success test structure

### Internal Packages Under Test
- `internal/ssh/client.go` — `Dial()` and `DialConfig` struct; the function integration tests call directly for SSH connectivity
- `internal/preflight/checks.go` — `RunPreflightChecks()`, `SSHRunner` interface, `NewSSHRunner()` adapter, `CheckResult` slice; integration tests inject a real `*gossh.Client` here
- `internal/filetransfer/upload.go` — `Upload()` function; integration tests call this directly against the real SFTP container; `warnedOnce *bool` param; `ShellQuote()` for command args
- `internal/filetransfer/filter.go` — `ShouldExclude()` and `WalkFiles()` — used indirectly by Upload(); relevant for --skip-env E2E test
- `internal/health/poll.go` — `PollHealth()` function; integration tests call this after a real `docker compose up -d` in DinD
- `internal/compose/run.go` — `RunCompose()` function; integration tests call this directly against the DinD container

### Config Resolution
- `internal/config/config.go` — `Config` struct and `FlagOpts` struct; integration tests construct `Config` directly (no CLI flags) for internal API calls

### Existing Test Infrastructure
- `go.mod` — `github.com/testcontainers/testcontainers-go v0.42.0` already present; `github.com/stretchr/testify v1.11.1` available for assertions

### Rules
- `CLAUDE.md` — Rule 1 (no InsecureIgnoreHostKey anywhere, including test helpers), Rule 3 (one `NewSession()` per SSH command — tests must follow this too), Rule 5 (docker compose v2 only)

</canonical_refs>

<code_context>
## Existing Code Insights

### Reusable Assets
- `startSSHContainer()` in `internal/ssh/client_test.go`: starts `lscr.io/linuxserver/openssh-server:latest` with testcontainers; can be copied verbatim into `integration/` helpers
- `captureHostKey()`, `seedKnownHosts()`, `emptyKnownHosts()`, `generateTestKeyFile()` in `internal/ssh/client_test.go`: ready-made helpers for knownhosts and key management in tests
- `preflight.NewSSHRunner(*gossh.Client) SSHRunner`: adapter for injecting a real `*gossh.Client` into `RunPreflightChecks()` without wiring the full deploy loop
- `filetransfer.ShellQuote()`: exported from Phase 3; use for any SSH exec commands in integration test setup/assertion code

### Established Patterns
- **`//go:build integration` build tag**: already established; integration tests are completely excluded from `go test ./...` unless the tag is set
- **`testcontainers.GenericContainer` + `wait.ForListeningPort`**: the container startup pattern from the existing SSH test; reuse for both Container A and Container B
- **One `NewSession()` per SSH command**: test code must also follow this; never share sessions across commands
- **`gossh.ExitError` for non-zero exit codes**: the existing fake in `preflight/checks_test.go` shows how exit errors are structured; real SSH exec in integration tests will return these too

### Integration Points
- `integration/testdata/Dockerfile.sshd` (new): custom image with 4 users + Docker daemon; built via `testcontainers.FromDockerfile{Context: "testdata/", Dockerfile: "Dockerfile.sshd"}`
- `integration/helpers_test.go` (new): shared container lifecycle, helper functions extracted from or modeled on `internal/ssh/client_test.go`
- `Makefile` (update): add `test-integration` target
- `.github/workflows/*.yml` (update): add `integration` job after existing unit-test job

</code_context>

<specifics>
## Specific Ideas

- **Container image for DinD+SSH**: Custom Dockerfile based on `docker:dind` with OpenSSH daemon added. Alternatively `ubuntu:22.04` base with Docker Engine + OpenSSH installed manually — more control over user setup.
- **Four-user setup** (user names from discussion): `root`, `nosudouser`, `sudopassuser`, `sshuser` (passwordless sudo). The Dockerfile configures each with the correct sudoers entry.
- **Atomicity sentinel file**: name it something obvious like `sentinel-before-deploy.txt` with content `original`. After cancelled upload, SSH into the container and verify `cat /opt/<project>/sentinel-before-deploy.txt` returns `original`.
- **Context cancellation for atomicity test**: use `context.WithCancel`, start upload in a goroutine, call cancel() after a short sleep or after monitoring partial SFTP progress. The cleaner approach is a wrapper around the `sftp.Client` that cancels after N `Write` calls.

</specifics>

<deferred>
## Deferred Ideas

- **macOS CI runner support**: macOS GitHub-hosted runners don't have Docker pre-installed (since 2024); adding macOS would require colima setup — deferred unless macOS-specific SSH behavior needs coverage.
- **Full CLI subprocess tests** (`exec.Command` black-box): not in scope for Phase 8. If a true black-box CLI test is ever needed, it belongs in its own test package.
- **Re-include mechanism** (`--include` flag): surfaced in Phase 7 deferred items; still not in scope.
- **Interactive sudo password prompt** (`sudopassuser` path): DEPLOY-07 auth fallback includes an interactive sudo-password prompt. The integration test covers the "sudo with password" user but the interactive TTY path may be tricky — planner should assess difficulty and mark as stretch goal if needed.

</deferred>

---

*Phase: 8-integration-tests*
*Context gathered: 2026-05-21*
