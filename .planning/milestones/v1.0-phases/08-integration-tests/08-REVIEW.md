---
phase: 08-integration-tests
reviewed: 2026-05-21T00:00:00Z
depth: standard
files_reviewed: 11
files_reviewed_list:
  - integration/testdata/Dockerfile.sshd
  - integration/testdata/entrypoint.sh
  - integration/helpers_test.go
  - Makefile
  - .github/workflows/ci.yml
  - integration/dial_test.go
  - integration/preflight_test.go
  - integration/filetransfer_test.go
  - integration/compose_test.go
  - integration/testdata/compose-healthy.yaml
  - integration/testdata/compose-unhealthy.yaml
findings:
  critical: 3
  warning: 5
  info: 3
  total: 11
status: issues_found
---

# Phase 08: Code Review Report

**Reviewed:** 2026-05-21
**Depth:** standard
**Files Reviewed:** 11
**Status:** issues_found

## Summary

The integration test suite sets up two Docker containers (a plain SSH container and a Docker-in-Docker + SSH container) via testcontainers-go, then exercises Dial, preflight checks, file transfer, and compose health-poll paths against them. The overall structure is sound and CLAUDE.md Rule 1 (no InsecureIgnoreHostKey) is correctly observed in `dialContainer` and `dialContainerA`. However, three blockers exist that will prevent the suite from running at all in a fresh environment: a raw Docker-multiplexed-stream bug that corrupts private key bytes on startup, a timeout configuration that is too short for cold CI runners, and a dial-timeout test that will never observe the expected error when SSH agent / config keys are absent. Five additional warnings cover logic correctness and reliability issues.

---

## Critical Issues

### CR-01: `container.Exec` reads raw multiplexed Docker stream — private key bytes are corrupted and `TestMain` always fails

**File:** `integration/helpers_test.go:179`

**Issue:** `container.Exec(ctx, []string{"cat", keyPath})` is called with no processing option. The testcontainers-go v0.42.0 documentation for `Exec` explicitly warns: *"reading directly from the io.Reader may result in unexpected bytes due to custom stream multiplexing headers."* Each frame is prefixed by an 8-byte Docker multiplexed-stream header (stream type + 4-byte length). `io.ReadAll(reader)` therefore returns PEM content contaminated with these headers. `gossh.ParsePrivateKey(keyBytes)` immediately fails for all four users (`root`, `sshuser`, `nosudouser`, `sudopassuser`), causing `newDinDContainer` to return an error. `TestMain` then calls `log.Fatalf`, which kills the test binary before any test function runs. The entire integration test suite is non-functional.

**Fix:** Pass `tcexec.Multiplexed()` to strip the framing headers using `stdcopy.StdCopy` internally:

```go
import tcexec "github.com/testcontainers/testcontainers-go/exec"

exitCode, reader, err := container.Exec(ctx, []string{"cat", keyPath}, tcexec.Multiplexed())
```

---

### CR-02: `TestDial_Timeout` fails on any CI runner without a pre-configured SSH agent or `~/.ssh/config`

**File:** `integration/dial_test.go:17-33`

**Issue:** `DialConfig` does not set `KnownHostsPath`, `PrivateKey`, or any auth fields. `internalssh.Dial` calls `buildAuthMethods(cfg.Hostname, cfg.User)` as its first step. On a fresh GitHub Actions `ubuntu-latest` runner there is no `SSH_AUTH_SOCK` and no `~/.ssh/config`, so `buildAuthMethods` returns an empty slice. `Dial` immediately returns:

```
"SSH auth failed: ensure your key is loaded in ssh-agent or configured in ~/.ssh/config for host 192.0.2.1"
```

This happens *before* any TCP connection is attempted, so the 500 ms timeout never fires. The test then asserts `strings.Contains(err.Error(), "timed out")` — which is false — and calls `t.Fatalf`. The CI `integration` job always fails on a cold runner.

**Fix:** Generate a throwaway key pair in the test and inject it via `DialConfig` (or a compatible field), or seed a minimal `~/.ssh/config` entry. Alternatively, refactor `DialConfig` to accept an explicit `Auth []gossh.AuthMethod` field so tests can supply test credentials directly without relying on the agent / file system. Example minimal approach:

```go
// Generate a dummy key so buildAuthMethods has something to work with.
// Auth will fail (key not in container), but the TCP layer will still time out.
_, signer := generateTestKeyFile(t)
cfg := internalssh.DialConfig{
    User:     "nobody",
    Hostname: "192.0.2.1",
    Port:     22,
    Timeout:  500 * time.Millisecond,
    // pass signer via new Auth field, or via a temp ~/.ssh/config
}
```

---

### CR-03: Integration test suite `timeout 5m` in `Makefile` is shorter than the minimum cold-start time

**File:** `Makefile:15`

**Issue:** `make test-integration` passes `-timeout 5m` to `go test`. `TestMain` starts two containers sequentially:

- Container A (`linuxserver/openssh-server:latest`): `WaitingFor` allows up to 90 s for the port to become ready, plus image pull time on a cold runner.
- Container B (custom `Dockerfile.sshd`): builds the image from scratch (installs Docker CE from the internet, runs `ssh-keygen` four times), then waits up to 120 s for the port.

On a GitHub Actions `ubuntu-latest` runner with no Docker layer cache, the combined startup cost routinely exceeds 5 minutes. When `go test` times out, all tests report as failed with a panic, masking real failures. The `integration` CI job also has no `timeout-minutes` guard, so a hung build consumes runner quota silently.

**Fix:** Increase to at least 15 minutes, and add a job-level timeout in CI:

```makefile
# Makefile
test-integration:
	go test -tags integration -timeout 15m ./integration/...
```

```yaml
# .github/workflows/ci.yml
  integration:
    needs: [test]
    runs-on: ubuntu-latest
    timeout-minutes: 20
    steps:
      ...
```

---

## Warnings

### WR-01: `captureStderr` ignores `os.Pipe()` error — nil pipe causes panic in `TestPreflight_CHECK07`

**File:** `integration/helpers_test.go:407-417`

**Issue:** `r, w, _ := os.Pipe()` discards the error. `os.Pipe()` can fail (e.g., file-descriptor exhaustion). If it does, both `r` and `w` are `nil`. The immediately following `os.Stderr = w` replaces stderr with `nil`; any subsequent write to `os.Stderr` (including by the test framework itself) panics. `buf.ReadFrom(r)` where `r` is nil also panics.

**Fix:**

```go
func captureStderr(t *testing.T, fn func()) string {
    t.Helper()
    r, w, err := os.Pipe()
    if err != nil {
        t.Fatalf("os.Pipe: %v", err)
    }
    old := os.Stderr
    os.Stderr = w
    fn()
    w.Close()
    os.Stderr = old
    var buf bytes.Buffer
    buf.ReadFrom(r)
    return buf.String()
}
```

The function signature change requires updating the call site in `preflight_test.go:231`.

---

### WR-02: `TestPreflight_CHECK05_PasswordlessSudo_Fail_nosudouser` has a false-pass condition when neither check result exists

**File:** `integration/preflight_test.go:364-374`

**Issue:** The `allPass` guard logic is:

```go
allPass := true
if targetDir != nil && targetDir.Status != "pass" {
    allPass = false
}
if dockerGroup != nil && dockerGroup.Status != "pass" {
    allPass = false
}
if allPass {
    t.Error(...)
}
```

If `RunPreflightChecks` returns an empty `results` slice (or a slice that contains neither `"target-dir"` nor `"docker-group"`), both `findResult` calls return `nil`. The `nil` check skips both branches, `allPass` remains `true`, and the test reports a failure as expected — but the failure message says "must produce at least one non-passing result" when the real problem is that neither result was present at all. More dangerously: if preflight is refactored so check names change (e.g. `"dir-writable"` instead of `"target-dir"`), this test silently passes for the wrong reason.

**Fix:** Assert that at least one of the expected results exists before checking its status:

```go
if targetDir == nil && dockerGroup == nil {
    t.Fatal("neither 'target-dir' nor 'docker-group' result was present; " +
        "preflight must produce these checks for nosudouser")
}
// existing allPass logic...
```

---

### WR-03: `TestUpload_AtomicCancel` — `cancel()` is not deferred; context leaks if `t.Skip` fires before goroutine completes

**File:** `integration/filetransfer_test.go:81-86`

**Issue:** The cancel goroutine calls `cancel()` after a 100 ms sleep. `defer cancel()` is never called in the test body. If `Upload` completes before the 100 ms elapses and `t.Skip` is called, the goroutine is still sleeping. The `context.CancelFunc` is not called until the goroutine wakes and calls it — only then is the context object released by the runtime. This is a resource leak (minor) and can also cause spurious cancellation of a context that is no longer in use if the test harness reuses goroutines across test runs. Additionally, because `t.Cleanup` for `rm -rf /opt/testapp-atomic` is registered *after* the potential `t.Skip` call on line 94, the pre-seeded remote directory is never cleaned up when the skip fires.

**Fix:**

```go
ctx, cancel := context.WithCancel(context.Background())
defer cancel() // always release, even on t.Skip

// Register cleanup before any skip/fatal points.
t.Cleanup(func() {
    sshExecHelper(t, client, "rm -rf /opt/testapp-atomic")
})

go func() {
    time.Sleep(100 * time.Millisecond)
    cancel()
}()
```

---

### WR-04: `entrypoint.sh` proceeds to start `sshd` even when `dockerd` fails to start within the 30-second window

**File:** `integration/testdata/entrypoint.sh:8-13`

**Issue:** The readiness loop polls `docker info` up to 30 times (30 seconds). If `dockerd` never becomes ready (e.g., overlay2 not supported on the kernel, cgroup issues), the loop exits silently and `exec /usr/sbin/sshd -D -e` starts anyway. testcontainers' `WaitingFor: wait.ForListeningPort("22/tcp")` will then succeed (sshd is listening), and all Docker-dependent tests will fail with opaque SSH command errors rather than a clear "docker daemon not running" message.

**Fix:** After the loop, assert that Docker is available before handing off to sshd:

```sh
if ! docker info >/dev/null 2>&1; then
    echo "ERROR: Docker daemon failed to start within 30 seconds" >&2
    exit 1
fi
exec /usr/sbin/sshd -D -e
```

---

### WR-05: CLAUDE.md specifies staging path `/opt/<project>/.deploy-tmp-<timestamp>`; implementation uses `/tmp/docker-deploy-<timestamp>`; test validates the wrong path

**File:** `integration/filetransfer_test.go:104`, `internal/filetransfer/upload.go:99`

**Issue:** CLAUDE.md Rule 3 specifies: *"stage to `/opt/<project>/.deploy-tmp-<timestamp>`, move atomically."* The implementation uses `/tmp/docker-deploy-<timestamp>` instead. The test asserts against `/tmp/docker-deploy-*`, so it is consistent with the implementation but validates behaviour that diverges from the specification. This matters because:

1. `/tmp` is typically a different filesystem partition than `/opt`, which means the final `mv` from `/tmp/...` to `/opt/...` is a cross-filesystem rename rather than an atomic same-filesystem rename. On many Linux systems `mv` across filesystems falls back to copy+delete, breaking the atomicity guarantee.
2. The spec requirement exists precisely to ensure both staging and target are on the same filesystem (so `rename(2)` is used).

**Fix:** Move the staging directory to be co-located with the target. Update `upload.go` line 99 and the test assertion on line 104:

```go
// upload.go
stagingDir := filepath.Dir(remoteBase) + "/.deploy-tmp-" + timestamp

// filetransfer_test.go
out2 := sshExecOutputHelper(t, client, "ls /opt/.deploy-tmp-* 2>/dev/null && echo found || echo none")
```

---

## Info

### IN-01: `compose-healthy.yaml` and `compose-unhealthy.yaml` testdata files are dead code

**File:** `integration/testdata/compose-healthy.yaml`, `integration/testdata/compose-unhealthy.yaml`

**Issue:** `compose_test.go` inlines both compose documents as string constants (`composeHealthyYAML`, `composeUnhealthyYAML`) and never reads from the testdata files. The YAML in the files and constants is identical, but having both creates a maintenance hazard: a future editor updating one may miss the other.

**Fix:** Either remove the testdata files and rely solely on the inline constants, or switch the tests to read from the files with `os.ReadFile(filepath.Join("testdata", "compose-healthy.yaml"))` and remove the constants.

---

### IN-02: `TestDial_Success` variable name `keyFile` holds a `gossh.Signer`, not a file path

**File:** `integration/dial_test.go:85`

**Issue:**

```go
_, keyFile := generateTestKeyFile(t)
_ = keyFile // key is not injected into container; auth will fail
```

`generateTestKeyFile` returns `(string, gossh.Signer)`. The blank identifier discards the file path (first return) and `keyFile` receives the `gossh.Signer` (second return). The name `keyFile` implies it is a file path, causing confusion for readers. Both values are unused — the entire two-line block can be removed.

**Fix:** Remove the unused call entirely:

```go
// No key injection needed — auth failure is the expected path.
```

---

### IN-03: CI `integration` job has no `timeout-minutes`, and the `test` job has no linter step

**File:** `.github/workflows/ci.yml`

**Issue:** The `integration` job has no `timeout-minutes` guard. A hung container build (e.g., Docker image pull stalling) will consume GitHub Actions runner quota for the full default 6-hour limit without surfacing a clear failure. This is distinct from the Makefile test timeout (CR-03) — even with a corrected `go test -timeout 15m`, the job itself has no ceiling.

Separately, the CI pipeline has `go build`, `go test`, and `go vet` steps but no `staticcheck` or `golangci-lint` step. The `captureStderr` unchecked-error issue (WR-01) and the multiplexed-stream bug (CR-01) would both be surfected by a linter with `errcheck` enabled.

**Fix:** Add `timeout-minutes: 20` to the `integration` job (see also CR-03 fix). Consider adding a `golangci-lint` step to the `test` job.

---

_Reviewed: 2026-05-21_
_Reviewer: Claude (gsd-code-reviewer)_
_Depth: standard_
