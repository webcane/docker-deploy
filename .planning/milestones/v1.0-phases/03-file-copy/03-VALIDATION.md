---
phase: 03
slug: file-copy
status: complete
nyquist_compliant: true
wave_0_complete: false
created: 2026-06-04
---

# Phase 3 — Validation Strategy (File Copy)

> Retroactively reconstructed from PLAN/SUMMARY artifacts (State B).

---

## Test Infrastructure

| Property | Value |
|----------|-------|
| **Framework** | go test |
| **Config file** | none (standard Go test toolchain) |
| **Quick run command** | `go test ./internal/config/... ./internal/filetransfer/... -count=1` |
| **Full suite command** | `go test ./... -count=1` |
| **Estimated runtime** | ~5 seconds |

---

## Sampling Rate

- **After every task commit:** `go test ./internal/config/... ./internal/filetransfer/... -count=1`
- **After every plan wave:** `go test ./... -count=1`
- **Before `/gsd-verify-work`:** Full suite must be green

---

## Per-Task Verification Map

| Task ID | Plan | Wave | Requirement | Threat Ref | Secure Behavior | Test Type | Automated Command | File Exists | Status |
|---------|------|------|-------------|------------|-----------------|-----------|-------------------|-------------|--------|
| 03-01-T1 | 01 | 1 | FILES-03 | T-03-01 | Built-in excludes cannot be removed by user input | unit | `go test ./internal/config/... -v -run TestResolveExcludes` | ✅ | ✅ green |
| 03-01-T2 | 01 | 1 | FILES-02 | T-03-01 | Config.Force is flag > file, no secret exposure | unit | `go test ./internal/config/... -v -run TestResolveForce` | ✅ | ✅ green |
| 03-02-T1 | 02 | 1 | FILES-01 | — | Makefile/.env/compose.yaml not in default excludes | unit | `go test ./internal/filetransfer/... -v -run TestShouldExclude` | ✅ | ✅ green |
| 03-02-T2 | 02 | 1 | DEPLOY-02 | — | SFTP wraps existing SSH client (no 2nd TCP dial) | unit | `go test ./internal/filetransfer/... -v -run TestUploadAuthFallback` | ✅ | ✅ green |
| 03-02-T3 | 02 | 1 | DEPLOY-03 | — | Files staged to /tmp before atomic mv; no partial state | unit | `go test ./internal/filetransfer/... -v -run "TestUploadFirstDeploy_RmBeforeMv\|TestUploadRepeatDeploy_ThreeStepSwapUnchanged"` | ✅ | ✅ green |
| 03-03-T1 | 03 | 2 | FILES-03 | — | --exclude flag extends, not replaces, built-in list | integration | `go test ./internal/config/... -v -run TestResolveExcludes/flag_extends` | ✅ | ✅ green |
| 03-04-T1 | 04 | 3 | DEPLOY-03 | — | Atomic swap rolls back on step-2 failure | unit | `go test ./internal/filetransfer/... -v -run TestUploadRepeatDeploy_ThreeStepSwapUnchanged` | ✅ | ✅ green |
| 03-05-T1 | 05 | 1 | DEPLOY-03 | — | First deploy places files directly under remoteBase | unit | `go test ./internal/filetransfer/... -v -run TestUploadFirstDeploy_RmBeforeMv` | ✅ | ✅ green |

*Status: ⬜ pending · ✅ green · ❌ red · ⚠️ flaky*

---

## Wave 0 Requirements

Existing infrastructure covers all phase requirements.

---

## Manual-Only Verifications

All phase behaviors have automated verification.

---

## Validation Audit 2026-06-04

| Metric | Count |
|--------|-------|
| Gaps found | 1 |
| Resolved | 1 |
| Escalated | 0 |

**Gap resolved:** FILES-01 PARTIAL — added `TestShouldExclude/Makefile_not_in_built-in_excludes` to `internal/filetransfer/filter_test.go` confirming Makefile is not excluded by any default pattern.

---

## Validation Sign-Off

- [x] All tasks have automated verify commands
- [x] No 3 consecutive tasks without automated verify
- [x] Wave 0 N/A (existing infrastructure covers all requirements)
- [x] No watch-mode flags
- [x] Feedback latency < 10s
- [x] `nyquist_compliant: true` set in frontmatter

**Approval:** approved 2026-06-04
