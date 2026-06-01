---
phase: 10
slug: add-phase-autosuggestion
status: draft
nyquist_compliant: false
wave_0_complete: false
created: 2026-06-01
---

# Phase 10 — Validation Strategy

> Per-phase validation contract for feedback sampling during execution.

---

## Test Infrastructure

| Property | Value |
|----------|-------|
| **Framework** | Go standard `testing` package |
| **Config file** | none — `go test ./...` discovers all tests |
| **Quick run command** | `go test ./cmd/docker-deploy/... ./internal/sshconfig/...` |
| **Full suite command** | `go test ./...` |
| **Estimated runtime** | ~5 seconds |

---

## Sampling Rate

- **After every task commit:** Run `go test ./cmd/docker-deploy/... ./internal/sshconfig/...`
- **After every plan wave:** Run `go test ./...`
- **Before `/gsd-verify-work`:** Full suite must be green
- **Max feedback latency:** ~5 seconds

---

## Per-Task Verification Map

| Task ID | Plan | Wave | Requirement | Test Type | Automated Command | File Exists | Status |
|---------|------|------|-------------|-----------|-------------------|-------------|--------|
| ListHosts RED | 10-01 | 1 | D-02 — enumerate ~/.ssh/config aliases | unit | `go test ./internal/sshconfig/... -run TestListHosts -v` | ❌ Wave 1 | pending |
| ListHosts GREEN | 10-01 | 1 | D-02, D-03 — silent failure on missing file | unit | `go test ./internal/sshconfig/... -run TestListHosts -v` | ❌ Wave 1 | pending |
| buildCompletionCmd wiring | 10-02 | 2 | D-01, D-04, D-05, D-06, D-07 | compile | `go build ./cmd/docker-deploy/...` | ❌ Wave 2 | pending |
| Completion tests | 10-02 | 2 | D-01 bash/zsh only | unit | `go test ./cmd/docker-deploy/... -run TestCompletionCmd -v` | ❌ Wave 2 | pending |
| Completion tests | 10-02 | 2 | D-01 fish rejected | unit | `go test ./cmd/docker-deploy/... -run TestCompletionCmd_InvalidShell -v` | ❌ Wave 2 | pending |
| Completion tests | 10-02 | 2 | D-04 registered subcommand | unit | `go test ./cmd/docker-deploy/... -run TestCompletionCmd_Registered -v` | ❌ Wave 2 | pending |
| Completion tests | 10-02 | 2 | D-02, D-03 --host completion | unit | `go test ./cmd/docker-deploy/... -run TestHostCompletion -v` | ❌ Wave 2 | pending |
| Completion tests | 10-02 | 2 | D-06 --path completion | unit | `go test ./cmd/docker-deploy/... -run TestPathCompletion -v` | ❌ Wave 2 | pending |
| Completion tests | 10-02 | 2 | D-07 --compose-file completion | unit | `go test ./cmd/docker-deploy/... -run TestComposeFileCompletion -v` | ❌ Wave 2 | pending |

---

## Wave 0 Gaps

- [ ] `internal/sshconfig/sshconfig_test.go` — add `TestListHosts_*` tests (file exists, needs new cases)
- [ ] `cmd/docker-deploy/main_test.go` — add completion subcommand and flag completion tests (file exists, needs new cases)

---

## Phase Gate

Before marking Phase 10 complete:

```bash
go test ./...
```

All tests must pass with zero failures.
