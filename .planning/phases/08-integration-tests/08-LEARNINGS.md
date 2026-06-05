---
phase: 8
phase_name: "integration-tests"
project: "docker-deploy"
generated: "2026-06-05"
counts:
  decisions: 8
  lessons: 6
  patterns: 6
  surprises: 4
missing_artifacts: []
---

# Phase 8 Learnings: Integration Tests

## Decisions

### ubuntu:22.04 base for DinD+SSH image
Used ubuntu:22.04 as the base for the custom Dockerfile.sshd rather than docker:dind. This gave full control over user setup, SSH configuration, and Docker installation from the official apt repo.

**Rationale:** docker:dind has opinionated user and process management. ubuntu:22.04 allowed creating exactly four users (root, sshuser, nosudouser, sudopassuser) with precisely configured sudo rules, authorized_keys, and SSH settings without fighting the base image defaults.
**Source:** 08-01-SUMMARY.md

---

### Build-time RSA key generation in Dockerfile
Per-user RSA key pairs are generated at Docker image build time via `RUN ssh-keygen` and stored at `/etc/ssh/test_keys/<user>_rsa`. Keys are baked into the image, not injected at runtime.

**Rationale:** Baking keys into the image allows key extraction via `container.Exec` without any SSH connection, which avoids the chicken-and-egg problem of needing known_hosts before being able to extract keys.
**Source:** 08-01-SUMMARY.md

---

### container.Exec for private key extraction (not SSH dial)
Private keys are extracted from Container B using `container.Exec(ctx, []string{"cat", keyPath})` rather than opening an SSH connection into the container.

**Rationale:** Using SSH to extract keys requires known_hosts to be pre-populated, which in turn requires the keys. `container.Exec` bypasses this bootstrapping problem entirely by going directly through the Docker API.
**Source:** 08-01-SUMMARY.md

---

### gossh.Dial directly in dialContainer, not internalssh.Dial
The `dialContainer()` helper uses raw `gossh.Dial()` rather than the project's own `internalssh.Dial()` wrapper.

**Rationale:** CI environments have no ssh-agent and no `~/.ssh/config`. Direct `gossh.Dial()` with explicit auth methods (PublicKeys from the extracted signer) bypasses the agent chain that `internalssh.Dial()` also tries.
**Source:** 08-01-SUMMARY.md

---

### Two-layer helper design for TestMain vs per-test use
Unexported `newSSHContainer`/`newDinDContainer` functions return `(container, error)` for use in TestMain; exported `startSSHContainer`/`startDinDContainer` wrappers accept `*testing.T` and call `t.Fatal` on error.

**Rationale:** TestMain cannot use `t.Fatalf` since it has no `*testing.T`. The two-layer design gives both TestMain (error return) and individual tests (t.Fatal convenience) appropriate error handling.
**Source:** 08-01-SUMMARY.md

---

### Native Docker socket in CI, no docker:dind service
The GitHub Actions integration job uses `ubuntu-latest` with the pre-installed Docker daemon. No `services: docker:dind` block is needed.

**Rationale:** testcontainers-go connects to `/var/run/docker.sock` automatically on ubuntu-latest runners. Avoiding docker:dind eliminates nested virtualization complexity and CI configuration overhead.
**Source:** 08-02-SUMMARY.md

---

### CHECK-02 and CHECK-03 fail tests use Container A as proxy
The tests for failing docker compose v2 and daemon checks connect to Container A (SSH-only, no Docker) rather than trying to break those checks directly on Container B.

**Rationale:** It is impossible to have compose v2 or a running daemon without the docker binary. CHECK-01 fails first on Container A, which is a valid proxy: the absence of docker binary implies absence of compose v2 and daemon.
**Source:** 08-04-SUMMARY.md

---

### busybox+exit1 for unhealthy container scenario, not CMD-SHELL HEALTHCHECK
The unhealthy compose test uses `busybox` with command `["sh", "-c", "exit 1"]` (container exits immediately) rather than a HEALTHCHECK that always fails.

**Rationale:** `poll.go` inspects `{{.State.Status}}` (running/exited/dead), not `{{.State.Health.Status}}`. A container with a failing HEALTHCHECK stays in "running" state — PollHealth would return nil. Only a container that exits triggers the error path.
**Source:** 08-06-SUMMARY.md

---

## Lessons

### openssh-server pre-generates SSH host keys during apt install
When `openssh-server` is installed via apt, it automatically creates `/etc/ssh/ssh_host_rsa_key`. A subsequent `RUN ssh-keygen -t rsa -f /etc/ssh/ssh_host_rsa_key -N ''` fails non-interactively because the file already exists.

**Context:** Dockerfile.sshd build failed until `rm -f /etc/ssh/ssh_host_rsa_key /etc/ssh/ssh_host_rsa_key.pub` was added before the keygen step.
**Source:** 08-01-SUMMARY.md

---

### useradd leaves accounts locked by default
`useradd` without `-p` creates an account with `!` in the shadow file (locked password). With `UsePAM no` in sshd_config, OpenSSH rejects even pubkey auth for locked accounts.

**Context:** sshuser and nosudouser SSH connections failed at authentication until `usermod -p '*'` was run to set an unlocked-but-passwordless state.
**Source:** 08-HUMAN-UAT.md

---

### DinD overlay2 fails on nested overlayfs (Colima/Lima)
Running Docker-in-Docker with the overlay2 storage driver fails when the host itself uses overlayfs (common with Colima and Lima on macOS).

**Context:** Adding `VOLUME /var/lib/docker` to Dockerfile.sshd causes Docker to use a host-managed ext4-backed volume for the inner daemon's storage, bypassing the nested overlayfs limitation.
**Source:** 08-HUMAN-UAT.md

---

### Test helpers writing to /opt require sudo even as sshuser
sshuser cannot write to `/opt` directly because `/opt` is owned by root with 755 permissions. Test pre-seed and cleanup steps that create or remove directories under `/opt` must use `sudo bash -c` or `sudo rm -rf`.

**Context:** Several integration test pre-seed steps failed at runtime because they used plain `mkdir` without sudo. Fixed by wrapping in `sshExecHelper` calls with `sudo bash -c "..."`.
**Source:** 08-HUMAN-UAT.md

---

### checkTargetDir returns false pass for existing restricted dirs
`mkdir -p` returns exit code 0 for an existing directory regardless of whether the calling user can write to it. The preflight `checkTargetDir` needed to combine `mkdir -p` with `test -w` to correctly detect unwritable target directories.

**Context:** CHECK-06 was incorrectly returning "pass" for a chmod 000 directory. The fix adds a `test -w` check alongside `mkdir -p` to distinguish truly writable from merely existing.
**Source:** 08-HUMAN-UAT.md

---

### plan.py YAML validation had a missing dependency
The CI YAML validation step using `python3 -c "import yaml; ..."` failed because pyyaml was not pre-installed. `pip3 install pyyaml` succeeded immediately.

**Context:** Minor CI tooling gap in Plan 02. No lasting impact — validation passed after pip install.
**Source:** 08-02-SUMMARY.md

---

## Patterns

### Shared container variables in TestMain for integration suites
Package-level `sshA *sshContainer` and `sshB *dinDContainer` are populated in `TestMain` once and reused by all test functions. Individual tests do not start their own containers.

**When to use:** Integration test suites where container startup is expensive (30+ seconds). Shared containers reduce total test time from minutes to seconds. Acceptable when tests are designed to be independent (using distinct remote paths, etc.).
**Source:** 08-01-SUMMARY.md, 08-03-SUMMARY.md

---

### knownhosts.New() in all SSH test dials
Every `gossh.Dial` call in the integration suite uses `knownhosts.New(khFile)` as the HostKeyCallback, where `khFile` is populated by `seedKnownHosts(t, host, port, hostKey)`.

**When to use:** Any integration test that opens an SSH connection. The pattern enforces the project's Rule 1 (no InsecureIgnoreHostKey) throughout the test suite and provides a concrete reference for how to wire host key verification correctly.
**Source:** 08-01-SUMMARY.md

---

### buildLargeLocalDir helper for mid-transfer cancellation
Creates N files of M bytes each to ensure an upload takes long enough for a timed goroutine cancel to fire before all files are transferred.

**When to use:** Testing context cancellation in file transfer code. Use 100 files × 1KB as baseline; adjust if CI hardware is significantly faster or slower. Always include a `t.Skip` guard in case the transfer completes before the cancel fires.
**Source:** 08-05-SUMMARY.md

---

### findResult helper for CheckResult slice lookups
A package-level `findResult(results []preflight.CheckResult, name string) *preflight.CheckResult` scans a slice by Name field and returns a pointer or nil.

**When to use:** Any integration test that calls `RunPreflightChecks` and needs to assert the status of a specific named check. Avoids repetitive for-loops in every test function.
**Source:** 08-04-SUMMARY.md

---

### Inline YAML constants in compose tests
Compose file content is defined as string constants in the test file rather than read from `testdata/` via `os.ReadFile`.

**When to use:** When `go test` binary working directory may differ from the source package directory (common with `./...` invocations). Inline constants are always available regardless of cwd. The `testdata/` files are still committed for documentation purposes.
**Source:** 08-06-SUMMARY.md

---

### t.Cleanup for DinD state teardown
Each compose test registers a `t.Cleanup` that runs `docker compose down --remove-orphans` to remove containers from the DinD daemon.

**When to use:** Any integration test that starts containers inside DinD. Without cleanup, containers from a failed test can interfere with subsequent tests that use the same project name or port binding.
**Source:** 08-06-SUMMARY.md

---

## Surprises

### poll.go inspects State.Status, not HEALTHCHECK health status
The plan specified writing a compose file with a failing HEALTHCHECK to test HEALTH-03. The actual `poll.go` implementation inspects container running state (`State.Status`) rather than health check status (`State.Health.Status`). A container with a failing HEALTHCHECK stays "running" and PollHealth returns nil.

**Impact:** Required changing both the test compose file (from CMD-SHELL HEALTHCHECK to busybox+exit1) and the error assertion (from "unhealthy" to "stopped unexpectedly"). The test correctly covers poll.go's actual behavior, but the original plan spec was misaligned with the implementation.
**Source:** 08-06-SUMMARY.md

---

### cobra stores RegisterFlagCompletionFunc results in a global map, not flag annotations
The test plan specified asserting `flag.Annotations` is non-nil after `Register()`. In cobra v1.10.2, `RegisterFlagCompletionFunc` stores functions in a package-level global map keyed by flag, not as flag annotations.

**Impact:** Test assertions had to use `cmd.GetFlagCompletionFunc("flagname")` instead of checking `flag.Annotations`. This was caught during the GREEN phase of TDD in Phase 10 Plan 02.
**Source:** 10-02-SUMMARY.md

---

### Makefile timeout needed to be 15m, not 5m
The plan specified `-timeout 5m` for the integration test target. DinD image builds from a cold cache plus container startup take longer than 5 minutes in practice.

**Impact:** The Makefile target was updated to `-timeout 15m` before the test suite was validated. Cold CI runs that have to pull and build the DinD image can exceed 5 minutes.
**Source:** 08-VERIFICATION.md

---

### plugin.Run intercepts bare "completion" args in Docker plugin context
When the docker-deploy binary is invoked as a Docker CLI plugin via `plugin.Run`, bare `completion` as the first argument is intercepted by Docker's own completion system, not routed to the hidden completion subcommand.

**Impact:** The `make completions` Makefile target had to use `./bin/docker-deploy deploy completion zsh` (with explicit `deploy` subcommand prefix) rather than `./bin/docker-deploy completion zsh`. The buildStandaloneRootForCompletion() helper was also required to produce scripts named `docker-deploy` instead of `docker`.
**Source:** 10-04-SUMMARY.md
