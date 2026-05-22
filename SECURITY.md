# SECURITY — Phase 08: Integration Tests

**Audit date:** 2026-05-22
**Phase:** 08 — Integration Tests
**ASVS Level:** 1
**Auditor:** gsd-security-auditor

---

## Threat Verification

### Mitigate-disposition threats (must have code evidence)

| Threat ID | Category | Component | Status | Evidence |
|-----------|----------|-----------|--------|----------|
| T-08-01-01 | Spoofing | dialContainer() HostKeyCallback | CLOSED | `integration/helpers_test.go:356` — `kh, err := knownhosts.New(khFile)`; `HostKeyCallback: kh` at line 362. No InsecureIgnoreHostKey call anywhere in `integration/` (grep confirmed). |
| T-08-01-05 | Denial of Service | Container startup timeout | CLOSED | `integration/helpers_test.go:74` — `wait.ForListeningPort("2222/tcp").WithStartupTimeout(90 * time.Second)` (Container A); line 136 — `.WithStartupTimeout(120 * time.Second)` (Container B). TestMain calls `log.Fatalf` at lines 320 and 330 on startup failure. |
| T-08-02-02 | Tampering | Makefile test-integration target | CLOSED | `Makefile:14-17` — `test-integration` target runs `./integration/...` only, not `./...`. The existing `test:` target (`go test ./...`) is unchanged (Makefile line 12). `.PHONY` includes `test-integration` (line 1). |
| T-08-02-03 | Denial of Service | CI job timeout | CLOSED | `Makefile:17` — `go test -v -tags integration -timeout 15m ./integration/...` (15m, extended from plan's 5m per operational decision — still bounded). CI job also has `timeout-minutes: 20` at `.github/workflows/ci.yml:31`. Mitigation present; timeout extended but not removed. |
| T-08-03-01 | Spoofing | TestDial_Success HostKeyCallback | CLOSED | `integration/dial_test.go:88` — `khFile := seedKnownHosts(t, sshA.host, sshA.port, sshA.hostKey)`; `KnownHostsPath: khFile` passed to `internalssh.Dial()` at line 93. No InsecureIgnoreHostKey in dial_test.go (grep confirmed). |
| T-08-04-01 | Spoofing | dialContainer HostKeyCallback (Container B) | CLOSED | `integration/preflight_test.go:48-56` — `dialContainerA` uses `seedKnownHosts` + `knownhosts.New()` + `HostKeyCallback: kh`. Container B dials go through `dialContainer()` in helpers_test.go which also uses `knownhosts.New()`. No InsecureIgnoreHostKey in preflight_test.go (grep confirmed). |
| T-08-04-03 | Tampering | CHECK-06 fail test chmod 000 | CLOSED | `integration/preflight_test.go:421-433` — `restrictedPath := "/opt/testapp-check06-nosudo"` (distinct, isolated path). `t.Cleanup` at line 425 restores permissions via `chmod 755` after assertion completes. |
| T-08-05-01 | Tampering | AtomicCancel test modifies /opt/testapp-atomic | CLOSED | `integration/filetransfer_test.go:74,79-80` — `remoteBase := "/opt/testapp-atomic"` (distinct path); `t.Cleanup` registered before assertions removes the directory via `sudo rm -rf /opt/testapp-atomic`. HappyPath uses `/opt/testapp-upload-happy`, SkipEnv uses `/opt/testapp-skipenv` — no path overlap. |
| T-08-05-03 | Denial of Service | Upload goroutine leak on panic | CLOSED | `integration/filetransfer_test.go:89-94` — `ctx, cancel := context.WithCancel(context.Background()); defer cancel()` ensures cancel is always called. Cancel goroutine at lines 92-95 calls `cancel()` after 100ms and exits naturally. No goroutine leak path. |
| T-08-06-03 | Tampering | DinD state pollution between tests | CLOSED | `integration/compose_test.go:46` (`compose-test-healthy`) and line 87 (`compose-test-unhealthy`) — distinct project names. `t.Cleanup` at lines 74 and 125 runs `docker compose ... down --remove-orphans` after each test. |

### Accept-disposition threats (documented accepted risks)

The following threats carry `accept` disposition in the phase plan. They are recorded here as closed accepted risks.

| Threat ID | Category | Accepted Risk Summary |
|-----------|----------|-----------------------|
| T-08-01-02 | Elevation of Privilege | Root login required for CHECK-07 test; container is ephemeral and isolated to test network |
| T-08-01-03 | Elevation of Privilege | DinD privileged mode required for testcontainers-go; risk scoped to CI runner lifetime |
| T-08-01-04 | Information Disclosure | Test-only RSA keys baked into throwaway image; no production secrets |
| T-08-02-01 | Elevation of Privilege | Docker socket access on ubuntu-latest CI runner; ephemeral per-run |
| T-08-03-02 | Information Disclosure | TEST-NET 192.0.2.1 (RFC 5737) guaranteed non-routable; no real host receives connection |
| T-08-03-03 | Tampering | TOFU acceptance writes to t.TempDir() temp file; cleaned up automatically |
| T-08-04-02 | Elevation of Privilege | Root SSH required for CHECK-07; container ephemeral; test asserts warning without elevation |
| T-08-05-02 | Information Disclosure | "SECRET=original" is test-only; ephemeral container |
| T-08-05-04 | Tampering | .env intentionally absent from localDir; Upload excludes list prevents overwrite |
| T-08-06-01 | Information Disclosure | /opt/compose-test-* paths; ephemeral container; no sensitive data |
| T-08-06-02 | Denial of Service | nginx:alpine ~10MB pull; Docker pre-installed on CI runners; within 5m timeout |
| T-08-06-04 | Denial of Service | CMD-SHELL exit 1 unconditional; reaches unhealthy within interval*retries; no false-positive |

---

## Unregistered Flags

**None.** The SUMMARY.md `## Threat Flags` section for phase 08-01 explicitly states: "None — no new network endpoints or auth paths beyond what the plan's threat model covers." No unregistered attack surface was detected during implementation.

---

## Critical Security Grep Results

```
grep -rn "InsecureIgnoreHostKey" integration/
→ integration/helpers_test.go:348: (comment only — "never InsecureIgnoreHostKey")
→ No actual call to InsecureIgnoreHostKey found anywhere in integration/
```

---

## Notes

**T-08-02-03 timeout deviation:** The plan specifies `-timeout 5m`; the implementation uses `-timeout 15m` in the Makefile and `timeout-minutes: 20` in CI. This is a conservative extension (the DinD container startup, Docker image pull, and compose test sequence requires more time in practice). The mitigation is still present — the job is bounded and will fail rather than hang indefinitely. This is not a security gap; it is an operational tuning decision made during implementation.

**captureHostKeyFromContainer bootstrap:** `integration/helpers_test.go:211-229` uses a transient custom HostKeyCallback that captures the key and immediately disconnects (returns an error). This one-time capture is used only to populate `sshB.hostKey` for subsequent `seedKnownHosts` calls; it is not used for any production code path. All subsequent dials (via `dialContainer`) use `knownhosts.New()` with the captured key.
