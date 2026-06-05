---
phase: 08-integration-tests
plan: 01
subsystem: testing
tags: [testcontainers, openssh, docker-in-docker, integration-tests, ssh, knownhosts]

# Dependency graph
requires:
  - phase: 07-leftovers
    provides: all internal package APIs (Upload, RunCompose, PollHealth, RunPreflightChecks) under test
provides:
  - integration/testdata/Dockerfile.sshd — DinD+SSH image with 4 users and build-time RSA key pairs
  - integration/helpers_test.go — TestMain lifecycle, container structs, all shared SSH helper functions
affects:
  - 08-02-dial
  - 08-03-preflight
  - 08-04-filetransfer
  - 08-05-compose
  - 08-06-ci

# Tech tracking
tech-stack:
  added: []
  patterns:
    - "testcontainers.FromDockerfile for building custom images in tests"
    - "container.Exec for extracting keys from containers without SSH dial"
    - "captureHostKey + seedKnownHosts + knownhosts.New() for compliant HostKeyCallback"
    - "TestMain with package-level container vars for shared container state across tests"
    - "dinDContainer.signers map[string]gossh.Signer keyed by username"

key-files:
  created:
    - integration/testdata/Dockerfile.sshd
    - integration/testdata/entrypoint.sh
    - integration/helpers_test.go
  modified: []

key-decisions:
  - "ubuntu:22.04 base for DinD+SSH image — maximum control over user setup vs docker:dind"
  - "Per-user RSA key pairs generated at Docker build time via RUN ssh-keygen; private keys at /etc/ssh/test_keys/<user>_rsa"
  - "container.Exec used to extract private key bytes — no SSH dial inside startDinDContainer (avoids chicken-and-egg with known_hosts)"
  - "gossh.Dial() directly in dialContainer, not internalssh.Dial() — bypasses ssh-agent/~/.ssh/config which are absent in CI"
  - "Two unexported helpers newSSHContainer/newDinDContainer for TestMain; t-bearing wrappers startSSHContainer/startDinDContainer for per-test use"
  - "Privileged: true on ContainerRequest for DinD — required for Docker daemon inside container"

patterns-established:
  - "Integration build tag: //go:build integration on line 1, blank line 2, package integration_test on line 3"
  - "Every integration/ file uses package integration_test (external test package)"
  - "dialContainer() always uses knownhosts.New() — never InsecureIgnoreHostKey"
  - "sshExecHelper/sshExecOutputHelper each call NewSession() — never reuse a session"

requirements-completed:
  - SC-1
  - SC-2
  - SC-3
  - SC-4
  - SC-5
  - SC-6
  - SC-7

# Metrics
duration: 35min
completed: 2026-05-21
---

# Phase 8 Plan 01: Integration Test Foundation Summary

**testcontainers-based DinD+SSH image with 4 users and build-time RSA key pairs, plus TestMain lifecycle with all shared SSH helpers for the integration test suite**

## Performance

- **Duration:** ~35 min
- **Started:** 2026-05-21T11:00:00Z
- **Completed:** 2026-05-21T11:35:00Z
- **Tasks:** 2
- **Files modified:** 3 created (Dockerfile.sshd, entrypoint.sh, helpers_test.go)

## Accomplishments

- Custom DinD+SSH Docker image built successfully: ubuntu:22.04 with Docker Engine CE, openssh-server, 4 users (root, sshuser, nosudouser, sudopassuser) with per-user RSA key pairs at /etc/ssh/test_keys/
- TestMain starts Container A (lscr.io/linuxserver/openssh-server) and Container B (custom Dockerfile.sshd) with proper lifecycle and teardown
- All shared helper functions defined: captureHostKey, seedKnownHosts, emptyKnownHosts, generateTestKeyFile, dialContainer, sshExecHelper, sshExecOutputHelper, captureStderr
- No InsecureIgnoreHostKey anywhere; private key extraction via container.Exec not SSH dial

## Task Commits

Each task was committed atomically:

1. **Task 1: Create integration/testdata/Dockerfile.sshd** - `fb20cac` (feat)
2. **Task 2: Create integration/helpers_test.go** - `c0ed721` (feat)

## Files Created/Modified

- `integration/testdata/Dockerfile.sshd` — DinD+SSH image with Docker Engine CE, openssh-server, 4 users with authorized_keys, per-user RSA key pairs at /etc/ssh/test_keys/
- `integration/testdata/entrypoint.sh` — Starts dockerd in background, waits for socket, then runs sshd -D in foreground
- `integration/helpers_test.go` — TestMain lifecycle; sshContainer + dinDContainer structs; newSSHContainer + newDinDContainer startup helpers; captureHostKey, seedKnownHosts, emptyKnownHosts, generateTestKeyFile, dialContainer, sshExecHelper, sshExecOutputHelper, captureStderr

## Decisions Made

- Used ubuntu:22.04 as base for Dockerfile.sshd (not docker:dind) — gives full control over user setup and SSH configuration
- Built-time key generation via `RUN ssh-keygen` in Dockerfile — keys are baked in, readable via `container.Exec`, no runtime key injection needed
- `container.Exec` for private key extraction instead of SSH dial — avoids bootstrapping problem of needing known_hosts before key extraction
- Two-layer helper design: `newSSHContainer`/`newDinDContainer` for TestMain (return error), `startSSHContainer`/`startDinDContainer` wrappers for per-test use (call t.Fatal)
- `dialContainer` uses raw `gossh.Dial` not `internalssh.Dial` — CI has no ssh-agent or ~/.ssh/config; direct auth method injection is required

## Deviations from Plan

### Auto-fixed Issues

**1. [Rule 1 - Bug] Removed pre-existing SSH host key before regenerating**
- **Found during:** Task 1 (Dockerfile.sshd build)
- **Issue:** openssh-server package pre-generates /etc/ssh/ssh_host_rsa_key during installation; subsequent `ssh-keygen` command failed with "file already exists" prompt
- **Fix:** Added `rm -f /etc/ssh/ssh_host_rsa_key /etc/ssh/ssh_host_rsa_key.pub` before `ssh-keygen -t rsa -f /etc/ssh/ssh_host_rsa_key -N ''`
- **Files modified:** integration/testdata/Dockerfile.sshd
- **Verification:** `docker build` exits 0; image verified with `docker run --rm <image> ls /etc/ssh/`
- **Committed in:** fb20cac (Task 1 commit)

---

**Total deviations:** 1 auto-fixed (Rule 1 - bug during Docker build)
**Impact on plan:** Necessary fix to make the image build. No scope creep.

## Issues Encountered

- openssh-server package installs SSH host keys automatically during apt-get install. The Dockerfile instruction `RUN ssh-keygen -t rsa -f /etc/ssh/ssh_host_rsa_key -N ''` fails non-interactively when the key already exists. Fixed by removing the pre-generated key first.

## Known Stubs

None — all code is functional infrastructure. No placeholder data or TODO stubs.

## Threat Flags

None — no new network endpoints or auth paths beyond what the plan's threat model covers. The mitigations for T-08-01-01 (knownhosts.New in dialContainer) are implemented. T-08-01-02 through T-08-01-05 are accepted risks per the plan's threat register.

## Next Phase Readiness

- Wave 2 plans (08-02-dial, 08-03-preflight, 08-04-filetransfer, 08-05-compose) can now proceed in parallel
- `sshA` (Container A) is available for dial tests
- `sshB` (Container B) with all 4 users is available for preflight, filetransfer, and compose tests
- `dialContainer(t, "sshuser")` provides a ready *gossh.Client for any test needing Container B access

---
*Phase: 08-integration-tests*
*Completed: 2026-05-21*
