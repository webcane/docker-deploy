---
phase: 14
slug: ssh-config-host-alias-resolution
status: compliant
nyquist_compliant: true
wave_0_complete: true
created: 2026-06-04
---

# Phase 14 — Validation Strategy

> Per-phase validation contract for SSH config host alias resolution.

---

## Test Infrastructure

| Property | Value |
|----------|-------|
| **Framework** | Go standard testing (`go test`) |
| **Config file** | `go.mod` |
| **Quick run command** | `go test ./internal/sshconfig/... ./internal/config/... -v` |
| **Full suite command** | `go test ./...` |
| **Estimated runtime** | ~3 seconds |

---

## Sampling Rate

- **After every task commit:** Run `go test ./internal/sshconfig/... ./internal/config/... -v`
- **After every plan wave:** Run `go test ./...`
- **Before `/gsd-verify-work`:** Full suite must be green
- **Max feedback latency:** 5 seconds

---

## Per-Task Verification Map

| Task ID | Plan | Wave | Requirement | Threat Ref | Secure Behavior | Test Type | Automated Command | File Exists | Status |
|---------|------|------|-------------|------------|-----------------|-----------|-------------------|-------------|--------|
| 14-01-01 | 01 | 1 | D-05/D-06: HostEntry struct exported | — | N/A | unit | `go test ./internal/sshconfig/... -run TestLookupHost_FoundWithAllDirectives` | ✅ | ✅ green |
| 14-01-02 | 01 | 1 | D-07: HostName fallback to alias label | — | N/A | unit | `go test ./internal/sshconfig/... -run TestLookupHost_FoundAliasOnly` | ✅ | ✅ green |
| 14-01-03 | 01 | 1 | D-04: Unmatched alias error before SSH dial | T-14-01-01 | Alias label never reaches dial or known_hosts check | unit | `go test ./internal/sshconfig/... -run TestLookupHost_NotFound` | ✅ | ✅ green |
| 14-01-04 | 01 | 1 | D-11: Include directives skipped silently | T-14-01-02 | Malformed Include cannot redirect to attacker host | unit | `go test ./internal/sshconfig/... -run TestLookupHost_IncludeSkipped` | ✅ | ✅ green |
| 14-01-05 | 01 | 1 | Wildcard Host block match | — | N/A | unit | `go test ./internal/sshconfig/... -run TestLookupHost_WildcardBlock` | ✅ | ✅ green |
| 14-01-06 | 01 | 1 | D-10: LoadSigners delegates to LookupHost | — | N/A | unit | `go test ./internal/sshconfig/... -run TestLoadSigners_DelegatesLookupHost` | ✅ | ✅ green |
| 14-01-07 | 01 | 1 | D-01: --host bare alias resolves via resolveHostString | T-14-01-01 | HostName (not alias) used for known_hosts | unit | `go test ./internal/config/... -run TestResolve_AliasResolved` | ✅ | ✅ green |
| 14-01-08 | 01 | 1 | D-04: Unmatched alias returns error | — | Error before SSH dial | unit | `go test ./internal/config/... -run TestResolve_AliasNotFound` | ✅ | ✅ green |
| 14-01-09 | 01 | 1 | Existing ssh:// URLs unchanged | — | N/A | unit | `go test ./internal/config/... -run TestResolve_SSHURLUnchanged` | ✅ | ✅ green |
| 14-01-10 | 01 | 1 | D-02: deploy.yaml target.host bare alias resolves via Resolve() | — | file.Target.Host alias path exercised | unit | `go test ./internal/config/... -run TestResolve_FileTargetAliasResolved` | ✅ | ✅ green |
| 14-02-01 | 02 | 1 | LoadFile bool signature: absent returns false | — | N/A | unit | `go test ./internal/config/... -run TestLoadFile_Absent` | ✅ | ✅ green |
| 14-02-02 | 02 | 1 | LoadFile bool signature: present returns true | — | N/A | unit | `go test ./internal/config/... -run TestLoadFile_Present` | ✅ | ✅ green |
| 14-02-03 | 02 | 1 | LoadFile bool signature: malformed returns true + error | — | N/A | unit | `go test ./internal/config/... -run TestLoadFile_Malformed` | ✅ | ✅ green |
| 14-02-04 | 02 | 1 | D-14a: No deploy.yaml + no --host error message | — | Error before SSH dial | unit | `go test ./internal/config/... -run TestNoHostError_FileAbsent` | ✅ | ✅ green |
| 14-02-05 | 02 | 1 | D-14b: deploy.yaml + no target.host error message | — | Error before SSH dial | unit | `go test ./internal/config/... -run TestNoHostError_FilePresent` | ✅ | ✅ green |

*Status: ⬜ pending · ✅ green · ❌ red · ⚠️ flaky*

---

## Wave 0 Requirements

Existing infrastructure covers all phase requirements. No Wave 0 stubs needed.

---

## Manual-Only Verifications

| Behavior | Requirement | Why Manual | Test Instructions |
|----------|-------------|------------|-------------------|
| `docker deploy --host minipc` connects to resolved HostName | D-01 | Requires a live machine with matching `~/.ssh/config` block | Run against a real VPS with `Host minipc` block; verify SSH connects to `HostName` value, not alias |
| `docker deploy --host ghost` with no matching block | D-04 | Requires real CLI invocation | Run without matching Host block; verify error contains `alias "ghost" not found in` |

---

## Validation Audit 2026-06-04

| Metric | Count |
|--------|-------|
| Gaps found | 1 |
| Resolved | 1 |
| Escalated | 0 |

Gap filled: `TestResolve_FileTargetAliasResolved` — D-02 path (`file.Target.Host` bare alias through `Resolve()`).

---

## Validation Sign-Off

- [x] All tasks have automated verify commands
- [x] Sampling continuity: no 3 consecutive tasks without automated verify
- [x] Wave 0 not needed — existing infrastructure covers all requirements
- [x] No watch-mode flags
- [x] Feedback latency < 5s
- [x] `nyquist_compliant: true` set in frontmatter

**Approval:** approved 2026-06-04
