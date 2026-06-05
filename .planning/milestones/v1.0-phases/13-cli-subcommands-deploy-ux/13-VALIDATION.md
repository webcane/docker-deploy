---
phase: 13
slug: cli-subcommands-deploy-ux
status: draft
nyquist_compliant: false
wave_0_complete: false
created: 2026-05-26
---

# Phase 13 — Validation Strategy

> Per-phase validation contract for feedback sampling during execution.

---

## Test Infrastructure

| Property | Value |
|----------|-------|
| **Framework** | Go testing stdlib |
| **Config file** | none (`go test ./...`) |
| **Quick run command** | `go test ./cmd/docker-deploy/... ./internal/filetransfer/... ./internal/preflight/... ./internal/config/...` |
| **Full suite command** | `go test ./...` |
| **Estimated runtime** | ~10 seconds |

---

## Sampling Rate

- **After every task commit:** Run `go test ./...`
- **After every plan wave:** Run `go test ./...`
- **Before `/gsd-verify-work`:** Full suite must be green
- **Max feedback latency:** ~10 seconds

---

## Per-Task Verification Map

| Task ID | Plan | Wave | Requirement | Threat Ref | Secure Behavior | Test Type | Automated Command | File Exists | Status |
|---------|------|------|-------------|------------|-----------------|-----------|-------------------|-------------|--------|
| 13-01-01 | 01 | 1 | SC-1 | — | N/A | unit | `go test ./internal/config/... -run TestLoadFile` | ✅ config_test.go | ⬜ pending |
| 13-02-01 | 02 | 1 | SC-2,3,4 | — | N/A | unit | `go test ./cmd/docker-deploy/... -run TestVersionCmd` | ❌ Wave 0 | ⬜ pending |
| 13-02-02 | 02 | 1 | SC-2,3,4 | — | N/A | build | `go build ./...` | n/a | ⬜ pending |
| 13-03-01 | 03 | 1 | SC-5 | — | No SSH connection on validate | unit | `go test ./cmd/docker-deploy/... -run TestValidateCmd` | ❌ Wave 0 | ⬜ pending |
| 13-04-01 | 04 | 1 | SC-6 | T-13-01 | SudoCreds.Zero() zeroes pw bytes; sshRun uses stdin pipe not env var | unit | `go test ./internal/filetransfer/... -run TestSudoExec` | ❌ Wave 0 | ⬜ pending |
| 13-04-02 | 04 | 1 | SC-6 | — | N/A | compilation | `go build ./...` | n/a | ⬜ pending |
| 13-04-03 | 04 | 1 | SC-6 | — | Exactly one password prompt per deploy (warnedOnce) | unit | `go test ./internal/filetransfer/... -run TestSudoExec_SinglePromptMultiFile` | ❌ Wave 0 | ⬜ pending |
| 13-05-01 | 05 | 2 | SC-7 | — | N/A | unit | `go test ./internal/filetransfer/... -run TestUploadVerbose_PreConfirmDiff` | ❌ Wave 0 | ⬜ pending |
| 13-06-01 | 06 | 2 | SC-8 | — | Writable path bypasses SudoExec entirely | unit | `go test ./internal/filetransfer/... -run TestUpload_PathAwareSudo` | ❌ Wave 0 | ⬜ pending |
| 13-07-01 | 07 | 1 | D-26..29 | T-13-02 | sudo -l output to stderr only; never blocks deploy | unit | `go test ./internal/preflight/... -run TestCheckDockerGroup_SudoL` | ❌ Wave 0 | ⬜ pending |

*Status: ⬜ pending · ✅ green · ❌ red · ⚠️ flaky*

---

## Wave 0 Requirements

- [ ] `cmd/docker-deploy/main_test.go` — add `TestVersionCmd*` tests (subcommand registered, output format with D-01 Docker CLI style)
- [ ] `cmd/docker-deploy/main_test.go` — add `TestValidateCmd*` tests (valid config exits 0, missing file exits non-zero, bad YAML exits non-zero, no SSH connection made)
- [ ] `internal/filetransfer/upload_test.go` — add `TestSudoExec*` tests (direct success, cached creds step 2, passwordless sudo step 3, interactive step 4)
- [ ] `internal/filetransfer/upload_test.go` — add `TestSudoExec_SinglePromptMultiFile` (8 SudoExec calls, interactive prompt fires exactly once via warnedOnce)
- [ ] `internal/filetransfer/upload_test.go` — add `TestUploadVerbose_PreConfirmDiff` (remote ReadDir + local WalkFiles lists appear before confirm prompt)
- [ ] `internal/filetransfer/upload_test.go` — add `TestUpload_PathAwareSudo` (writable probe skips SudoExec; non-writable triggers SudoExec)
- [ ] `internal/preflight/checks_test.go` — add `TestCheckDockerGroup_SudoL*` (verbose=true prints `[sudo -l]` output; sudo -l failure is silently skipped)

---

## Manual-Only Verifications

| Behavior | Requirement | Why Manual | Test Instructions |
|----------|-------------|------------|-------------------|
| `docker deploy version` output format matches D-01 exactly | SC-2,3,4 | Visual format check | Run `docker deploy version` on a tagged build; verify 4-line output with version, git commit, built, OS/arch |
| `docker deploy validate` makes no SSH connection | SC-5 | Requires network isolation check | Run with `--host 192.0.2.1` (unreachable IP, 3s timeout); validate must return before timeout |

---

## Validation Sign-Off

- [ ] All tasks have `<automated>` verify or Wave 0 dependencies
- [ ] Sampling continuity: no 3 consecutive tasks without automated verify
- [ ] Wave 0 covers all MISSING references
- [ ] No watch-mode flags
- [ ] Feedback latency < 30s
- [ ] `nyquist_compliant: true` set in frontmatter

**Approval:** pending
