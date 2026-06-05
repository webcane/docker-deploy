---
phase: 10
slug: add-phase-autosuggestion
status: complete
nyquist_compliant: true
wave_0_complete: true
created: 2026-06-01
audited: 2026-06-02
---

# Phase 10 — Validation Strategy

> Per-phase validation contract for feedback sampling during execution.

---

## Test Infrastructure

| Property | Value |
|----------|-------|
| **Framework** | Go standard `testing` package |
| **Config file** | none — `go test ./...` discovers all tests |
| **Quick run command** | `go test ./cmd/docker-deploy/... ./internal/sshconfig/... ./internal/completion/...` |
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

| Task ID | Plan | Wave | Requirement | Test Type | Automated Command | File | Status |
|---------|------|------|-------------|-----------|-------------------|------|--------|
| ListHosts RED | 10-01 | 1 | D-02 — enumerate ~/.ssh/config aliases | unit | `go test ./internal/sshconfig/... -run TestListHosts -v` | `internal/sshconfig/sshconfig_test.go` | ✅ COVERED |
| ListHosts GREEN | 10-01 | 1 | D-02, D-03 — silent failure on missing file | unit | `go test ./internal/sshconfig/... -run TestListHosts -v` | `internal/sshconfig/sshconfig_test.go` | ✅ COVERED |
| buildCompletionCmd wiring | 10-02 | 2 | D-01, D-04 | compile | `go build ./cmd/docker-deploy/...` | — | ✅ COVERED |
| Completion - bash/zsh registered | 10-02 | 2 | D-01, D-04 — subcommand registered, hidden | unit | `go test ./cmd/docker-deploy/... -run TestCompletionCmd_Registered -v` | `cmd/docker-deploy/main_test.go` | ✅ COVERED |
| Completion - fish rejected | 10-02 | 2 | D-01 — unsupported shells rejected | unit | `go test ./cmd/docker-deploy/... -run TestCompletionCmd_InvalidShell -v` | `cmd/docker-deploy/main_test.go` | ✅ COVERED |
| Bash script generates | 10-02/04 | 2 | D-01 — bash completion output | unit | `go test ./internal/completion/... -run TestGenerateBash -v` | `internal/completion/bash_test.go` | ✅ COVERED |
| Zsh script generates | 10-02/04 | 2 | D-01 — zsh completion output | unit | `go test ./internal/completion/... -run TestGenerateZsh -v` | `internal/completion/zsh_test.go` | ✅ COVERED |
| Dynamic completion hooks | 10-03 | 3 | D-03 — hooks removed | n/a | Deleted in Plan 03 — D-06/D-07 tests N/A | — | 🗑 REMOVED |

---

## Manual-Only Requirements

| Requirement | Reason | Verification |
|-------------|--------|--------------|
| Makefile `completions` target idempotency | Requires `make` + built binary; not a unit test | `make completions && git diff --exit-code contrib/` exits 0 |
| `contrib/install-completions.sh` shell execution | Requires interactive shell environment | `sh contrib/install-completions.sh --dry-run` (or `sh -n contrib/install-completions.sh`) |
| goreleaser archives `files:` entry | CI/release-time validation | `grep -A5 'files:' .goreleaser.yaml` |
| INSTALL.md Shell Completions section | Documentation review | `grep '## Shell Completions' INSTALL.md` |

---

## Phase Gate

```bash
go test ./...
```

All tests pass with zero failures. ✅ Verified 2026-06-02.

---

## Validation Audit 2026-06-02

| Metric | Count |
|--------|-------|
| Gaps found | 1 |
| Resolved | 1 |
| Escalated | 0 |
| Manual-only | 4 |

**Gap resolved:** `TestCompletionCmd_InvalidShell` added to `cmd/docker-deploy/main_test.go`. Verifies that `cobra.MatchAll(cobra.ExactArgs(1), cobra.OnlyValidArgs)` with `ValidArgs=["bash","zsh"]` rejects unsupported shells (e.g. `fish`) before `RunE` fires.

**Removed requirements (D-06, D-07, dynamic --host/--path/--compose-file completion):** Plan 03 deleted `internal/completion/completion.go` and all dynamic flag completion hooks per the static-completion rework. Those test rows are no longer applicable.
