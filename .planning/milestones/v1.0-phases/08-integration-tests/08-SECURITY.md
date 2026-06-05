---
phase: 8
slug: integration-tests
status: verified
threats_open: 0
asvs_level: 1
created: 2026-05-22
---

# Phase 8 — Security

> Per-phase security contract: threat register, accepted risks, and audit trail.

---

## Trust Boundaries

| Boundary | Description | Data Crossing |
|----------|-------------|---------------|
| test runner → SSH container | Test code connects to container SSH port; containers are ephemeral and isolated | SSH auth credentials, host keys (test-only) |
| Dockerfile.sshd build → Docker daemon | Image is built locally or in CI; no external registry pull for custom image | Docker build context (test fixtures) |
| Container SSH keys → test code | Private keys are read from container via testcontainers exec API, never written to disk permanently | RSA host key bytes (ephemeral) |
| CI runner → Docker socket | testcontainers-go connects to /var/run/docker.sock on ubuntu-latest runner | Container lifecycle commands |
| CI workflow → repository | GitHub Actions has write access to trigger builds | Source code, test results |
| test runner → DinD daemon | RunCompose and PollHealth execute docker commands inside Container B's Docker daemon | docker compose invocations |
| Container B DinD → Docker Hub | nginx:alpine pulled from Docker Hub during test run | Container image (~10MB) |

---

## Threat Register

| Threat ID | Category | Component | Disposition | Mitigation | Status |
|-----------|----------|-----------|-------------|------------|--------|
| T-08-01-01 | Spoofing | dialContainer() HostKeyCallback | mitigate | `helpers_test.go:356-362` — `knownhosts.New(khFile)` assigned to `HostKeyCallback`; no `InsecureIgnoreHostKey` in codebase (grep verified) | closed |
| T-08-01-02 | Elevation of Privilege | Dockerfile.sshd PermitRootLogin | accept | Root login required for CHECK-07 test coverage; container is ephemeral and isolated to test network; no production traffic | closed |
| T-08-01-03 | Elevation of Privilege | DinD privileged container | accept | Privileged mode required for Docker-in-Docker; testcontainers isolates to test run; risk is CI runner compromise, not data exfiltration | closed |
| T-08-01-04 | Information Disclosure | Test key files /etc/ssh/test_keys/ | accept | Keys are test-only, baked into a throwaway image; no production secrets involved | closed |
| T-08-01-05 | Denial of Service | Container startup timeout | mitigate | `helpers_test.go:74` — Container A: `WithStartupTimeout(90s)`; line 136 — Container B: `WithStartupTimeout(120s)`; `log.Fatalf` on startup failure | closed |
| T-08-02-01 | Elevation of Privilege | Native Docker socket in CI | accept | ubuntu-latest runners are ephemeral and isolated per run; Docker socket access required for testcontainers-go; risk scoped to CI job lifetime | closed |
| T-08-02-02 | Tampering | Makefile test-integration target | mitigate | `Makefile:14-17` — `test-integration` runs `./integration/...` only; existing `test:` target unchanged | closed |
| T-08-02-03 | Denial of Service | CI job timeout | mitigate | `Makefile:17` — `-timeout 15m` (extended from 5m for DinD + image pull overhead; bounded); `ci.yml:31` — `timeout-minutes: 20` | closed |
| T-08-03-01 | Spoofing | TestDial_Success HostKeyCallback | mitigate | `dial_test.go:88` — `seedKnownHosts()` pre-seeds real host key; `KnownHostsPath` passed to `internalssh.Dial()`; no `InsecureIgnoreHostKey` | closed |
| T-08-03-02 | Information Disclosure | TestDial_Timeout target (192.0.2.1) | accept | TEST-NET address guaranteed non-routable per RFC 5737; no real host receives the connection | closed |
| T-08-03-03 | Tampering | TOFU acceptance writes to known_hosts | accept | `emptyKnownHosts()` creates temp file via `t.TempDir()`; cleaned up automatically after test; no risk to user's real known_hosts | closed |
| T-08-04-01 | Spoofing | dialContainer HostKeyCallback | mitigate | `preflight_test.go:48-56` — `dialContainerA` uses `seedKnownHosts` + `knownhosts.New()` + `HostKeyCallback: kh`; Container B via `dialContainer()` same pattern | closed |
| T-08-04-02 | Elevation of Privilege | RunPreflightChecks as root | accept | Root SSH required for CHECK-07 test; container is ephemeral; test asserts warning emitted without blocking | closed |
| T-08-04-03 | Tampering | CHECK-06 chmod 000 cleanup | mitigate | `preflight_test.go:421,425-433` — isolated path `/opt/testapp-check06-nosudo`; `t.Cleanup` restores `chmod 755` after assertion | closed |
| T-08-05-01 | Tampering | AtomicCancel remoteBase isolation | mitigate | `filetransfer_test.go:74,79-80` — distinct path `/opt/testapp-atomic`; `t.Cleanup` removes directory; other tests use non-overlapping paths | closed |
| T-08-05-02 | Information Disclosure | .env content SECRET=original | accept | Test-only value; no real secret; contained to ephemeral container | closed |
| T-08-05-03 | Denial of Service | Upload goroutine leak on panic | mitigate | `filetransfer_test.go:89-94` — `ctx, cancel := context.WithCancel(...); defer cancel()`; cancel goroutine exits naturally | closed |
| T-08-05-04 | Tampering | SkipEnv .env exclusion guarantee | accept | .env intentionally absent from localDir; Upload() with excludes prevents overwriting remote .env — test verifies this guarantee | closed |
| T-08-06-01 | Information Disclosure | compose test remote paths | accept | All paths use `/opt/compose-test-*` prefixes; ephemeral container; no sensitive data | closed |
| T-08-06-02 | Denial of Service | nginx:alpine pull in CI | accept | nginx:alpine is a tiny image (~10MB); test timeout of 15m sufficient for pull + startup | closed |
| T-08-06-03 | Tampering | DinD state pollution between tests | mitigate | `compose_test.go:46,87` — distinct project names `compose-test-healthy` / `compose-test-unhealthy`; `t.Cleanup` runs `docker compose ... down --remove-orphans` | closed |
| T-08-06-04 | Denial of Service | unhealthy healthcheck CMD-SHELL exit 1 | accept | exit 1 unconditional; no external dependency; reaches unhealthy within interval×retries; no false-positive healthy state risk | closed |

*Status: open · closed*
*Disposition: mitigate (implementation required) · accept (documented risk) · transfer (third-party)*

---

## Accepted Risks Log

| Risk ID | Threat Ref | Rationale | Accepted By | Date |
|---------|------------|-----------|-------------|------|
| AR-08-01 | T-08-01-02 | PermitRootLogin required for CHECK-07 coverage; container is ephemeral and isolated; not a production path | plan author | 2026-05-22 |
| AR-08-02 | T-08-01-03 | DinD privileged mode required for testcontainers Docker-in-Docker; isolated to CI job; risk is runner compromise | plan author | 2026-05-22 |
| AR-08-03 | T-08-01-04 | Test SSH keys baked into throwaway Dockerfile image; test-only, no production secrets | plan author | 2026-05-22 |
| AR-08-04 | T-08-02-01 | Docker socket access required for testcontainers-go; ubuntu-latest runners are ephemeral and scoped per job | plan author | 2026-05-22 |
| AR-08-05 | T-08-03-02 | 192.0.2.1 is RFC 5737 TEST-NET; no real host receives connection attempts | plan author | 2026-05-22 |
| AR-08-06 | T-08-03-03 | TOFU test uses t.TempDir() temp file; auto-cleanup after test; no risk to user's real known_hosts | plan author | 2026-05-22 |
| AR-08-07 | T-08-04-02 | Root SSH for CHECK-07 test; ephemeral container; confirms warning-without-block production behavior | plan author | 2026-05-22 |
| AR-08-08 | T-08-05-02 | SECRET=original is a test-only value in an ephemeral container; no real credential at risk | plan author | 2026-05-22 |
| AR-08-09 | T-08-05-04 | .env absence intentional by design; test validates the skip-env guarantee, not a gap | plan author | 2026-05-22 |
| AR-08-10 | T-08-06-01 | /opt/compose-test-* paths in ephemeral container; no sensitive data present | plan author | 2026-05-22 |
| AR-08-11 | T-08-06-02 | nginx:alpine ~10MB; bounded by 15m test timeout; CI runners have Docker pre-installed | plan author | 2026-05-22 |
| AR-08-12 | T-08-06-04 | CMD-SHELL exit 1 has no external dependency; deterministic unhealthy path; no false-positive risk | plan author | 2026-05-22 |

*Accepted risks do not resurface in future audit runs.*

---

## Security Audit Trail

| Audit Date | Threats Total | Closed | Open | Run By |
|------------|---------------|--------|------|--------|
| 2026-05-22 | 22 | 22 | 0 | gsd-security-auditor (Phase 8 initial audit) |

### Audit 2026-05-22 Summary

| Metric | Count |
|--------|-------|
| Threats found | 22 |
| Closed | 22 |
| Open | 0 |

**Notable finding (non-blocking):** T-08-02-03 timeout deviation — plan specified `-timeout 5m`, implementation uses `-timeout 15m` (Makefile) and `timeout-minutes: 20` (CI). Mitigation intent (bounded job, fails fast) is satisfied; extension was operationally necessary for DinD startup + Docker image pull overhead.

**Bootstrap key-capture pattern:** `helpers_test.go:211-229` uses a transient custom callback to capture `sshB.hostKey` at startup (disconnects immediately after capture). All test-time dials use `knownhosts.New()` with the captured key — the `InsecureIgnoreHostKey` invariant is fully preserved for all actual test connections.

---

## Sign-Off

- [x] All threats have a disposition (mitigate / accept / transfer)
- [x] Accepted risks documented in Accepted Risks Log
- [x] `threats_open: 0` confirmed
- [x] `status: verified` set in frontmatter

**Approval:** verified 2026-05-22
