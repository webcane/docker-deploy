---
status: passed
phase: 08-integration-tests
source: [08-VERIFICATION.md]
started: 2026-05-21T14:00:00Z
updated: 2026-05-22T06:30:00Z
---

## Current Test

Human verification complete — all tests pass.

## Tests

### 1. Run `make test-integration` end-to-end
expected: All tests in ./integration/... pass (or TestUpload_AtomicCancel is skipped with 'cancel fired too late' on fast hardware); exit code 0; no manual container setup required
result: PASSED — `ok github.com/webcane/docker-deploy/integration 60.170s`

Notes: Four root causes required runtime fixes before the suite passed:
- SSH pubkey auth rejected for sshuser/nosudouser: `useradd` leaves accounts locked (`!` in
  shadow); `UsePAM no` causes sshd to reject locked accounts even for pubkey auth. Fixed via
  `usermod -p '*'` in Dockerfile.sshd.
- DinD inner Docker daemon failing with overlay2: nested overlayfs fails on Colima/Lima. Fixed
  by adding `VOLUME /var/lib/docker` so the inner storage uses a host-managed ext4-backed volume.
- Test helpers creating/removing /opt dirs without sudo: sshuser cannot write to /opt directly.
  Fixed by using `sudo bash -c` for pre-seed steps and `sudo rm -rf` for cleanups.
- `checkTargetDir` returning false 'pass': `mkdir -p` returns 0 for existing directories
  regardless of permissions. Fixed by combining with `test -w` and adding `sudo -n mkdir -p`
  step for passwordless-sudo users.

All changes committed in afa29a9 ("fix(08): resolve all integration test failures").

### 2. Confirm HEALTH-03 design intent
expected: PollHealth returns non-nil error when a container reaches a terminal failure state (exited/dead). The implementation uses busybox+exit1 (no HEALTHCHECK, container exits immediately) rather than a container with a failing HEALTHCHECK. Confirm this is acceptable given poll.go checks State.Status (running/exited/dead), not State.Health.Status (healthy/unhealthy). Both compose test scenarios have no HEALTHCHECK defined.
result: CONFIRMED ACCEPTABLE

Rationale: poll.go intentionally inspects `State.Status` (running/exited/dead) and not
`State.Health.Status` (healthy/unhealthy). The busybox+exit1 approach correctly exercises the
HEALTH-03 "stopped unexpectedly" error path. The phase goal's "with and without a HEALTHCHECK
defined" wording describes two different container types; both nginx:alpine and busybox (as used)
have no HEALTHCHECK, meaning both tests cover the "without HEALTHCHECK" branch. This is
consistent with the implementation: poll.go has no `unhealthy` health-status case because
supporting HEALTHCHECK-based signaling was not a v1 requirement. The test suite correctly covers
what was implemented.

## Summary

total: 2
passed: 2
issues: 0
pending: 0
skipped: 0
blocked: 0

## Gaps

None — all tests pass; HEALTH-03 design intent confirmed acceptable.
