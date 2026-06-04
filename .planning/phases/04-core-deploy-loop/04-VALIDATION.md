---
phase: 4
slug: core-deploy-loop
status: complete
nyquist_compliant: true
wave_0_complete: true
created: 2026-06-04
---

# Phase 4 — Validation Strategy

> Per-phase validation contract. State B reconstruction from PLAN + SUMMARY artifacts (2026-06-04).

---

## Test Infrastructure

| Property | Value |
|----------|-------|
| **Framework** | `go test` (stdlib) |
| **Config file** | none — no config file needed |
| **Quick run command** | `go test ./internal/config/... ./internal/compose/... ./internal/filetransfer/... ./cmd/docker-deploy/...` |
| **Full suite command** | `go test ./...` |
| **Estimated runtime** | ~8 seconds |

---

## Sampling Rate

- **After every task commit:** Run `go test ./internal/config/... ./internal/compose/... ./internal/filetransfer/...`
- **After every plan wave:** Run `go test ./...`
- **Before `/gsd-verify-work`:** Full suite must be green
- **Max feedback latency:** ~8 seconds

---

## Per-Task Verification Map

| Task ID | Plan | Wave | Requirement | Threat Ref | Secure Behavior | Test Type | Automated Command | File | Status |
|---------|------|------|-------------|------------|-----------------|-----------|-------------------|------|--------|
| 04-01-01 | 01 | 1 | DEPLOY-01 | T-04-01-01 | ComposeFile is basename only; no path separators stored | unit | `go test ./internal/config/... -v -run TestResolveComposeFile` | internal/config/config_test.go | ✅ green |
| 04-02-01 | 02 | 1 | DEPLOY-04 | T-04-02-01 | ShellQuote(remotePath) prevents injection in exec command | unit | `go test ./internal/compose/... -v -run TestRunCompose_CommandConstruction` | internal/compose/run_test.go | ✅ green |
| 04-02-02 | 02 | 1 | DEPLOY-05 | T-04-02-05 | Non-zero exit code propagated via errors.As(ExitError) | unit | `go test ./internal/compose/... -v -run TestRunCompose_ExitCodeNonZero` | internal/compose/run_test.go | ✅ green |
| 04-02-03 | 02 | 1 | DEPLOY-06 | T-04-02-03 | PTY session.Stdin not connected to os.Stdin (output-only) | unit | `go test ./internal/compose/... -v -run TestRunCompose` | internal/compose/run_test.go | ✅ green |
| 04-03-01 | 03 | 2 | DEPLOY-01 | T-04-03-01 | --compose-file flag registered; basename validation rejects path traversal | unit | `go test ./cmd/docker-deploy/... -v -run TestComposeFileFlagRegistered\|TestComposeFile_BasenameInjectionGuard` | cmd/docker-deploy/main_test.go | ✅ green |
| 04-04-01 | 04 | 3 | DEPLOY-07 | — | Direct copy, passwordless sudo, interactive password, all-paths-exhausted | unit | `go test ./internal/filetransfer/... -v -run TestUploadAuthFallback\|TestSudoExec` | internal/filetransfer/upload_test.go | ✅ green |

*Status: ⬜ pending · ✅ green · ❌ red · ⚠️ flaky*

---

## Wave 0 Requirements

Existing infrastructure covers all phase requirements. No stubs needed.

---

## Manual-Only Verifications

| Behavior | Requirement | Why Manual | Verified Date |
|----------|-------------|------------|---------------|
| PTY output routing (docker compose colors, progress bars) | DEPLOY-06 | term.IsTerminal() returns false in CI; PTY path requires a real terminal | 2026-05-15 (human checkpoint, 04-03 Plan 03 Task 2) |
| Full end-to-end: copy files → compose up, streaming output | DEPLOY-01, DEPLOY-04, DEPLOY-06 | Requires real remote SSH host and docker compose | 2026-05-15 (6 scenarios verified, 04-03 Plan 03 Task 2) |
| Interactive sudo password prompt UX | DEPLOY-07 | term.ReadPassword() requires a real TTY; stdin mocking deferred | Not formally verified (behavior implemented, SudoExec unit tests cover retry logic) |

---

## Nyquist Audit Trail (2026-06-04)

| Metric | Count |
|--------|-------|
| Gaps found | 2 |
| Resolved (tests generated) | 2 |
| Escalated to manual-only | 0 |

**Gaps filled:**
1. `TestComposeFileFlagRegistered` — `cmd/docker-deploy/main_test.go` — verifies `--compose-file` flag registered on cobra command (DEPLOY-01)
2. `TestComposeFile_BasenameInjectionGuard` — `cmd/docker-deploy/main_test.go` — verifies runDeploy rejects `"../evil.yaml"` before SSH dial (T-04-03-01)

---

## Validation Sign-Off

- [x] All tasks have automated verify or are in Manual-Only
- [x] Sampling continuity: no 3 consecutive tasks without automated verify
- [x] Wave 0: existing infrastructure covers all phase requirements
- [x] No watch-mode flags in any test command
- [x] Feedback latency < 10s
- [x] `nyquist_compliant: true` set in frontmatter

**Approval:** approved 2026-06-04
