---
phase: 02
slug: ssh-transport-config
status: validated
nyquist_compliant: true
wave_0_complete: true
created: 2026-06-02
audited: 2026-06-02
---

# Phase 2 — Validation Strategy

> Per-phase validation contract. State B reconstruction from 02-01/02/03 SUMMARY files.

---

## Test Infrastructure

| Property | Value |
|----------|-------|
| **Framework** | Go standard `testing` package |
| **Config file** | `go.mod` (Go modules) |
| **Quick run command** | `go test ./internal/config/... ./internal/ssh/... ./cmd/docker-deploy/...` |
| **Full suite command** | `go test ./...` |
| **Integration tests** | `go test -tags=integration ./internal/ssh/...` (requires Docker daemon) |
| **Estimated runtime** | ~2 seconds (unit); ~2 min (integration) |

---

## Sampling Rate

- **After every task commit:** Run `go test ./internal/config/... ./internal/ssh/... ./cmd/docker-deploy/...`
- **After every plan wave:** Run `go test ./...`
- **Before `/gsd-verify-work`:** Full suite must be green
- **Max feedback latency:** ~2 seconds

---

## Per-Task Verification Map

| Task ID | Plan | Wave | Requirement | Threat Ref | Secure Behavior | Test Type | Automated Command | File Exists | Status |
|---------|------|------|-------------|------------|-----------------|-----------|-------------------|-------------|--------|
| 02-01-01 | 01 | 1 | CFG-03/04/05 | T-02-01 | deploy.yaml malformed YAML → error, no panic | unit | `go test ./internal/config/... -run TestLoadFile` | ✅ | ✅ green |
| 02-01-02 | 01 | 1 | CFG-03/04/05 | T-02-02 | ParseHost rejects non-ssh:// schemes; requires hostname | unit | `go test ./internal/config/... -run TestParseHost` | ✅ | ✅ green |
| 02-01-02 | 01 | 1 | CFG-04 | — | flag > file > default precedence (host) | unit | `go test ./internal/config/... -run TestResolveHostPrecedence` | ✅ | ✅ green |
| 02-01-02 | 01 | 1 | CFG-04 | — | flag > file > default precedence (path) | unit | `go test ./internal/config/... -run TestResolvePathPrecedence` | ✅ | ✅ green |
| 02-01-03 | 01 | 1 | CFG-02 | — | --host flag registered as string on deploy cmd | unit | `go test ./cmd/docker-deploy/... -run TestHostFlagRegistered` | ✅ | ✅ green |
| 02-01-03 | 01 | 1 | CFG-02 | — | --path flag registered as string on deploy cmd | unit | `go test ./cmd/docker-deploy/... -run TestPathFlagRegistered` | ✅ | ✅ green |
| 02-01-03 | 01 | 1 | CFG-02 | — | --dry-run flag registered as bool on deploy cmd | unit | `go test ./cmd/docker-deploy/... -run TestDryRunFlagRegistered` | ✅ | ✅ green |
| 02-02-01 | 02 | 1 | CFG-01 | T-02-04 | known host key accepted (nil returned by callback) | unit | `go test ./internal/ssh/... -run TestBuildHostKeyCallback_KnownHostAccepted` | ✅ | ✅ green |
| 02-02-01 | 02 | 1 | CFG-01 | T-02-04 | unknown host → *UnknownHostError (TOFU signal) | unit | `go test ./internal/ssh/... -run TestBuildHostKeyCallback_UnknownHost` | ✅ | ✅ green |
| 02-02-01 | 02 | 1 | CFG-01 | T-02-04 | changed key → *KeyMismatchError (hard-fail signal) | unit | `go test ./internal/ssh/... -run TestBuildHostKeyCallback_KeyMismatch` | ✅ | ✅ green |
| 02-02-02 | 02 | 1 | CFG-01 | T-02-05/06/07 | no password/keyboard-interactive auth methods added | unit | `go test ./internal/ssh/... -run TestBuildAuthMethods_NoPasswordOrKeyboardInteractive` | ✅ | ✅ green |
| 02-02-02 | 02 | 1 | CFG-01 | T-02-07 | Dial() times out on unreachable host within cfg.Timeout | integration | `go test -tags=integration ./internal/ssh/... -run TestDial_Timeout` | ✅ | ✅ green |
| 02-02-02 | 02 | 1 | CFG-01 | T-02-04/09 | unknown host + user says "no" → non-nil error | integration | `go test -tags=integration ./internal/ssh/... -run TestDial_UnknownHost` | ✅ | ✅ green |
| 02-02-02 | 02 | 1 | CFG-01 | T-02-09 | TOFU accepted → known_hosts populated | integration | `go test -tags=integration ./internal/ssh/... -run TestDial_UnknownHostAccepted` | ✅ | ✅ green |
| 02-02-02 | 02 | 1 | CFG-01 | T-02-04 | Dial() success with seeded known_hosts | integration | `go test -tags=integration ./internal/ssh/... -run TestDial_Success` | ✅ | ✅ green |
| 02-03-01 | 03 | 2 | CFG-01/04 | T-02-10 | --dry-run wires config.Resolve + sshpkg.Dial (build check) | build | `go build ./cmd/docker-deploy/` | ✅ | ✅ green |

*Status: ⬜ pending · ✅ green · ❌ red · ⚠️ flaky*

---

## Wave 0 Requirements

Existing infrastructure covered all phase requirements. New unit tests added during Nyquist audit:

- `internal/ssh/knownhosts_test.go` — knownhosts callback mechanics (4 tests)
- `cmd/docker-deploy/main_test.go` — flag registration tests (3 tests added)

---

## Manual-Only Verifications

| Behavior | Requirement | Why Manual | Test Instructions |
|----------|-------------|------------|-------------------|
| `--dry-run` end-to-end with real SSH host | CFG-01 | Requires live SSH-accessible host; human UAT gate in 02-03-PLAN.md | `docker deploy --host ssh://user@host:22 --dry-run` — expect connectivity summary + Status: OK |
| Changed fingerprint loud warning + ssh-keygen hint printed | CFG-01 / T-02-04 | Requires two different keys for same host; impractical to automate in unit tests | Manually test: add a host entry with key A to known_hosts, then connect with key B; expect stderr warning and remediation command |

---

## Validation Sign-Off

- [x] All tasks have automated verify or manual-only documented
- [x] Sampling continuity: no 3 consecutive tasks without automated verify
- [x] No Wave 0 MISSING references remain
- [x] No watch-mode flags
- [x] Feedback latency < 2s (unit suite)
- [x] `nyquist_compliant: true` set in frontmatter

**Approval:** approved 2026-06-02

---

## Validation Audit 2026-06-02

| Metric | Count |
|--------|-------|
| Gaps found | 6 |
| Resolved | 6 |
| Escalated | 0 |
| Partial (integration-only, pre-existing) | 4 |
