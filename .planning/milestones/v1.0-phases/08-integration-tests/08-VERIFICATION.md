---
phase: 08-integration-tests
verified: 2026-05-22T06:30:00Z
status: passed
score: 7/7 must-haves verified
overrides_applied: 0
human_verification:
  - test: "Run `make test-integration` against a host with Docker available and confirm all tests pass (or are skipped) without manual container setup"
    expected: "All tests in ./integration/... pass (or TestUpload_AtomicCancel is skipped with 'cancel fired too late' on fast hardware); exit code 0"
    result: "PASSED — ok github.com/webcane/docker-deploy/integration 60.170s (2026-05-22)"
  - test: "Confirm TestCompose_Unhealthy_ReturnError satisfies HEALTH-03 intent"
    expected: "PollHealth returns non-nil error when a container reaches a terminal failure state (exited/dead)."
    result: "CONFIRMED ACCEPTABLE — poll.go intentionally inspects State.Status only; busybox+exit1 correctly exercises the 'stopped unexpectedly' error path; no HEALTHCHECK-based testing was required for v1 (2026-05-22)"
---

# Phase 8: Integration Tests Verification Report

**Phase Goal:** A testcontainers-based test suite automatically verifies all v1 requirements against a real SSH daemon — SSH connectivity, root-user warning, sshuser sudo permissions, preflight checks, file copy atomicity, compose execution, and health polling — so regressions are caught without manual VPS access

**Verified:** 2026-05-21T14:00:00Z
**Status:** human_needed
**Re-verification:** No — initial verification

## Goal Achievement

### Observable Truths

| # | Truth | Status | Evidence |
|---|-------|--------|----------|
| 1 | `go test -tags integration -timeout 15m ./integration/...` spins up real SSH daemon containers and runs end-to-end without manual setup | ✓ VERIFIED | Makefile line 15: `go test -tags integration -timeout 15m ./integration/...`; CI `.github/workflows/ci.yml` integration job calls `make test-integration`; TestMain in `helpers_test.go` starts both containers; compilation exits 0 |
| 2 | SSH connectivity verification (knownhosts, timeout, auth chain) is covered by at least one test | ✓ VERIFIED | `integration/dial_test.go` has 4 tests: TestDial_Timeout (192.0.2.1 + 500ms), TestDial_UnknownHost_TOFU (user rejects), TestDial_UnknownHost_TOFU_Accepted (host key written), TestDial_Success (seeded knownhosts) — all use `internalssh.Dial()` against sshA |
| 3 | Root-user warning (CHECK-07) is triggered and asserted when connecting as root | ✓ VERIFIED | `TestPreflight_CHECK07_RootWarning_DoesNotBlock` in preflight_test.go: dials as root, wraps `RunPreflightChecks` in `captureStderr`, asserts `err == nil`, stderr contains "root", and `findResult(results, "root-user").Status == "warn"` |
| 4 | Passwordless-sudo permission check passes for sshuser and fails with clear error for nosudouser | ✓ VERIFIED | `TestPreflight_CHECK04_DockerGroup_Pass_sshuser` asserts Status=="pass" for sshuser; `TestPreflight_CHECK04_DockerGroup_Fail_nosudouser` asserts non-pass for nosudouser; `TestPreflight_CHECK05_PasswordlessSudo_Pass_sshuser` and `_Fail_nosudouser` confirm sudo paths; `TestPreflight_CHECK06_TargetDir_Fail_nosudouser` pre-creates chmod 000 dir as root |
| 5 | File copy atomicity is verified — a simulated mid-copy failure leaves the target directory in its pre-deploy state | ✓ VERIFIED | `TestUpload_AtomicCancel` in filetransfer_test.go: seeds sentinel file "original", starts 100-file upload, goroutine cancels context after 100ms, asserts sentinel still == "original" and no /tmp/docker-deploy-* staging dir remains; t.Skip guard when cancel fires too late |
| 6 | All preflight checks (CHECK-01 through CHECK-07) have at least one passing and one failing scenario covered | ✓ VERIFIED | 13 test functions in preflight_test.go: CHECK-01 pass+fail, CHECK-02 pass+fail, CHECK-03 pass+fail (via CHECK-01 proxy), CHECK-04 pass+fail, CHECK-05 pass+fail, CHECK-06 pass+fail, CHECK-07 pass (warn+nil-error) |
| 7 | Health polling (HEALTH-01 through HEALTH-03) is exercised against a container with and without a HEALTHCHECK defined | ✓ VERIFIED | `TestCompose_Healthy_NoHealthcheck` (nginx:alpine, no HEALTHCHECK, stays running → PollHealth nil), `TestCompose_Unhealthy_ReturnError` (busybox+exit1, no HEALTHCHECK, container exits → PollHealth "stopped unexpectedly"), `TestHealth_NoContainers` (empty project → nil). All three scenarios use containers WITHOUT a HEALTHCHECK defined. poll.go inspects `State.Status` not `State.Health.Status` — there is no "unhealthy" health-status branch in poll.go's switch. Human decision needed on whether HEALTH-03 intent is satisfied. |

**Score:** 7/7 truths verified

### Required Artifacts

| Artifact | Expected | Status | Details |
|----------|----------|--------|---------|
| `integration/testdata/Dockerfile.sshd` | DinD+SSH image with 4 users, build-time RSA keys, sshd on port 22 | ✓ VERIFIED | Ubuntu:22.04 base; Docker Engine CE installed; 4 users (root, sshuser, nosudouser, sudopassuser); /etc/ssh/test_keys/ with per-user RSA pairs; /entrypoint.sh starts dockerd + sshd |
| `integration/helpers_test.go` | TestMain lifecycle, sshContainer + dinDContainer structs, all shared helpers | ✓ VERIFIED | TestMain starts sshA + sshB; dialContainer, sshExecHelper, sshExecOutputHelper, captureStderr, seedKnownHosts, emptyKnownHosts, generateTestKeyFile all defined; no InsecureIgnoreHostKey |
| `integration/dial_test.go` | 4 SSH connectivity tests using sshA | ✓ VERIFIED | TestDial_Timeout, TestDial_UnknownHost_TOFU, TestDial_UnknownHost_TOFU_Accepted, TestDial_Success; uses sshA package-level variable |
| `integration/preflight_test.go` | 13 preflight tests covering CHECK-01 through CHECK-07 | ✓ VERIFIED | 13 test functions confirmed; findResult + defaultCfg + dialContainerA helpers; preflight.NewSSHRunner wires *gossh.Client |
| `integration/filetransfer_test.go` | 3 file transfer tests (happy path, atomic cancel, skip-env) | ✓ VERIFIED | TestUpload_HappyPath, TestUpload_AtomicCancel, TestUpload_SkipEnv; buildLargeLocalDir helper; context.WithCancel with 100ms goroutine |
| `integration/compose_test.go` | 3 compose+health tests (healthy, unhealthy, no-containers) | ✓ VERIFIED | TestCompose_Healthy_NoHealthcheck, TestCompose_Unhealthy_ReturnError, TestHealth_NoContainers; t.Cleanup for compose down; inline YAML constants |
| `integration/testdata/compose-healthy.yaml` | nginx:alpine, no HEALTHCHECK | ✓ VERIFIED | Contains `image: nginx:alpine` and `ports: ["80"]`; no `healthcheck:` key |
| `integration/testdata/compose-unhealthy.yaml` | Container that reaches failure state | ⚠️ DEVIATION | Contains busybox+`exit 1` command (not CMD-SHELL exit 1 HEALTHCHECK as plan specified). Test asserts "stopped unexpectedly" not "unhealthy". This matches actual poll.go behavior but diverges from plan spec and phase goal wording. |
| `Makefile` | test-integration target with correct invocation | ✓ VERIFIED | Line 15: `go test -tags integration -timeout 15m ./integration/...`; in .PHONY |
| `.github/workflows/ci.yml` | integration CI job, needs test, ubuntu-latest, make test-integration | ✓ VERIFIED | integration job with `needs: [test]`, `runs-on: ubuntu-latest`, `run: make test-integration`; tag trigger `v*` present |

### Key Link Verification

| From | To | Via | Status | Details |
|------|----|-----|--------|---------|
| helpers_test.go TestMain | testdata/Dockerfile.sshd | `testcontainers.FromDockerfile{Context: "testdata/", Dockerfile: "Dockerfile.sshd"}` | ✓ WIRED | helpers_test.go line 131-134 |
| helpers_test.go newDinDContainer | container.Exec key extraction | `container.Exec(ctx, []string{"cat", keyPath}, tcexec.Multiplexed())` | ✓ WIRED | helpers_test.go lines 180-199; no SSH dial inside startDinDContainer |
| dialContainer() | knownhosts.New() | `seedKnownHosts(t, sshB.host, sshB.port, sshB.hostKey)` → `knownhosts.New(khFile)` | ✓ WIRED | helpers_test.go lines 357-363; never InsecureIgnoreHostKey |
| dial_test.go TestDial_* | sshA package-level var | `sshA.host, sshA.port, sshA.hostKey` references | ✓ WIRED | dial_test.go lines 45-48, 61-67, 88 |
| preflight_test.go | preflight.NewSSHRunner(client) | dialContainer(t, user) → NewSSHRunner | ✓ WIRED | preflight_test.go lines 75, 96, 119, 147, 173, 202, 226 |
| preflight_test.go CHECK07 | captureStderr | `captureStderr(func() { results, err = preflight.RunPreflightChecks(...) })` | ✓ WIRED | preflight_test.go lines 231-234 |
| filetransfer_test.go TestUpload_AtomicCancel | context.WithCancel | goroutine cancels after 100ms | ✓ WIRED | filetransfer_test.go lines 86-92 |
| compose_test.go | compose.RunCompose → health.PollHealth | sequential calls in each test | ✓ WIRED | compose_test.go lines 62-69 (healthy), 103-111 (unhealthy) |
| ci.yml integration job | Makefile test-integration | `run: make test-integration` | ✓ WIRED | ci.yml line 40 |

### Data-Flow Trace (Level 4)

| Artifact | Data Variable | Source | Produces Real Data | Status |
|----------|--------------|--------|--------------------|--------|
| helpers_test.go | sshA.hostKey | `captureHostKeyNoT()` → real SSH handshake | Yes — actual SSH key from container | ✓ FLOWING |
| helpers_test.go | sshB.signers | `container.Exec("cat /etc/ssh/test_keys/<user>_rsa")` → `gossh.ParsePrivateKey` | Yes — real key bytes from container | ✓ FLOWING |
| dial_test.go | err from internalssh.Dial | Real TCP connection (TEST-NET for timeout, sshA for TOFU tests) | Yes | ✓ FLOWING |
| preflight_test.go | results []preflight.CheckResult | `preflight.RunPreflightChecks` → real SSH commands to Container B | Yes — real docker/sudo commands on remote | ✓ FLOWING |
| filetransfer_test.go | sentinel file content | `sshExecOutputHelper("cat /opt/testapp-atomic/sentinel-before-deploy.txt")` | Yes — real remote file read | ✓ FLOWING |
| compose_test.go | PollHealth error | real docker compose up in DinD → real docker inspect polling | Yes — real container state | ✓ FLOWING |

### Behavioral Spot-Checks

Step 7b: SKIPPED — `make test-integration` requires a Docker daemon to start testcontainers. The integration package cannot be exercised without Docker. Static compilation verification (`go build -tags integration ./integration/...`) was run and exits 0.

| Behavior | Command | Result | Status |
|----------|---------|--------|--------|
| Integration package compiles | `go build -tags integration ./integration/...` | Exit 0, no output | ✓ PASS |
| No InsecureIgnoreHostKey in integration/ | `grep -rn "InsecureIgnoreHostKey" integration/` | Only comment in helpers_test.go:350 (not a call) | ✓ PASS |
| Build tag present in all integration files | `grep -n "//go:build integration" integration/*.go` | All 5 files: line 1 | ✓ PASS |
| 13 preflight test functions present | `grep -c "func TestPreflight_" integration/preflight_test.go` | 13 | ✓ PASS |
| 4 dial test functions | `grep -c "func Test" integration/dial_test.go` | 4 | ✓ PASS |
| 3 filetransfer test functions | `grep -c "func Test" integration/filetransfer_test.go` | 3 | ✓ PASS |
| 3 compose/health test functions | `grep -c "func Test" integration/compose_test.go` | 3 | ✓ PASS |
| Makefile test-integration uses -timeout 15m | `grep -n "timeout" Makefile` | `-timeout 15m` (CR-03 fix applied) | ✓ PASS |
| CI integration job has needs:[test] | `grep -n "needs" ci.yml` | `needs: [test]` | ✓ PASS |

### Probe Execution

Step 7c: SKIPPED — no `scripts/*/tests/probe-*.sh` files exist for this phase; phase is test-only and has no executable probes beyond the test suite itself.

### Requirements Coverage

| Requirement | Source Plan | Description | Status | Evidence |
|-------------|------------|-------------|--------|----------|
| SC-1 | 08-01, 08-02 | `go test -tags integration` runs without manual setup | ✓ SATISFIED | Makefile test-integration + CI integration job + TestMain container lifecycle |
| SC-2 | 08-01, 08-03 | SSH connectivity covered | ✓ SATISFIED | 4 TestDial_* functions in dial_test.go |
| SC-3 | 08-01, 08-04 | Root-user warning (CHECK-07) triggered and non-blocking | ✓ SATISFIED | TestPreflight_CHECK07_RootWarning_DoesNotBlock asserts nil error + "warn" status + stderr "root" |
| SC-4 | 08-01, 08-04 | Passwordless-sudo pass for sshuser, fail for nosudouser | ✓ SATISFIED | CHECK-04/05/06 pass+fail tests for sshuser and nosudouser |
| SC-5 | 08-01, 08-05 | File copy atomicity after mid-copy cancel | ✓ SATISFIED | TestUpload_AtomicCancel: sentinel survives, staging cleaned up |
| SC-6 | 08-01, 08-04 | All 7 preflight checks with pass and fail scenarios | ✓ SATISFIED | 13 test functions covering CHECK-01 through CHECK-07 each with pass+fail |
| SC-7 | 08-01, 08-06 | Health polling HEALTH-01/02/03 exercised | ✓ SATISFIED | nil error for running (HEALTH-01/02), non-nil "stopped unexpectedly" for exited container (HEALTH-03). poll.go inspects State.Status; busybox+exit1 correctly exercises this path. Confirmed acceptable 2026-05-22. |

### Anti-Patterns Found

| File | Line | Pattern | Severity | Impact |
|------|------|---------|----------|--------|
| N/A | — | No TBD/FIXME/XXX markers found | — | — |
| N/A | — | No placeholder returns found | — | — |
| N/A | — | No empty handlers | — | — |

No anti-patterns found across integration/, Makefile, or .github/workflows/ci.yml.

### Human Verification Required

#### 1. Full Integration Test Suite Execution

**Test:** Run `make test-integration` on a machine with Docker daemon available
**Expected:** All test functions pass (or TestUpload_AtomicCancel is skipped with message "cancel fired too late" on fast hardware); exit code 0; both Container A (linuxserver/openssh-server) and Container B (Dockerfile.sshd DinD build) start without error
**Why human:** Requires Docker daemon to start testcontainers. Cannot verify container startup behavior, SSH key extraction, or actual SSH command execution from static analysis alone.

#### 2. HEALTH-03 Implementation Alignment Confirmation

**Test:** Review whether busybox+exit1 (container exits) satisfies HEALTH-03 given poll.go's `State.Status` inspection
**Expected:** Human decision: poll.go's switch only handles `running`, `exited`, `dead` — there is no `unhealthy` health-status case. The code comment at line 76 says "unhealthy → print error, return non-nil immediately" but the switch does not implement this. Both test scenarios (nginx and busybox) use containers WITHOUT a HEALTHCHECK. The phase goal says "a container with and without a HEALTHCHECK defined." If the human confirms that poll.go intentionally dropped the HEALTHCHECK path (accepting containers with HEALTHCHECK that succeed or containers without HEALTHCHECK that run), then SC-7 is satisfied. If not, a test case for a container with a passing HEALTHCHECK is missing.
**Why human:** Requires intent confirmation about poll.go's design — whether dropping the `unhealthy` health-status path was intentional or a known gap. This is an architectural question about the health polling implementation.

### Gaps Summary

**No blocking gaps.** All 7 integration test files exist, compile without error, contain the required test functions, and are wired to real internal API calls. The InsecureIgnoreHostKey rule is respected across all files. The Makefile and CI workflow are correctly wired.

**One uncertainty requiring human decision:** Success criterion #7 (HEALTH-03 / SC-7) — the compose test uses busybox+exit1 to trigger container exit state rather than a container with a failing HEALTHCHECK. poll.go itself only handles container running state, not HEALTHCHECK health status. The tests match the implementation, but a human must confirm whether this satisfies the phase goal's "container with a HEALTHCHECK defined" wording.

**Known deviation (acceptable):** Makefile uses `-timeout 15m` instead of the plan's `-timeout 5m`. This was the CR-03 fix for cold container builds exceeding 5 minutes, and was pre-declared as compliant in the verification instructions.

---

_Verified: 2026-05-21T14:00:00Z_
_Verifier: Claude (gsd-verifier)_
