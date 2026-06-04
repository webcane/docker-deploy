---
phase: "09"
slug: documentation
status: compliant
nyquist_compliant: true
wave_0_complete: true
created: 2026-06-04
---

# Phase 09 — Validation Strategy

> Per-phase validation contract for the Distribution & Documentation phase.

---

## Test Infrastructure

| Property | Value |
|----------|-------|
| **Framework** | go test |
| **Config file** | none — stdlib only |
| **Quick run command** | `go test ./docs/...` |
| **Full suite command** | `go test ./...` |
| **Estimated runtime** | ~2 seconds |

---

## Sampling Rate

- **After every task commit:** Run `go test ./docs/...`
- **After every plan wave:** Run `go test ./...`
- **Before `/gsd-verify-work`:** Full suite must be green
- **Max feedback latency:** 5 seconds

---

## Per-Task Verification Map

| Task ID | Plan | Wave | Requirement | Threat Ref | Secure Behavior | Test Type | Automated Command | File Exists | Status |
|---------|------|------|-------------|------------|-----------------|-----------|-------------------|-------------|--------|
| 09-01-01 | 01 | 1 | SC-09-1 | T-09-01-01 | darwin builds produced; checksums signed | content | `go test ./docs/... -run TestSC091` | ✅ | ✅ green |
| 09-01-02 | 01 | 1 | SC-09-3 | T-09-01-02 | cosign keyless signs block present | content | `go test ./docs/... -run TestSC093` | ✅ | ✅ green |
| 09-02-01 | 02 | 1 | SC-09-2 | T-09-02-01 | SHA256 always verified; cosign fallback prints correct message | content + sh -n | `go test ./docs/... -run TestSC092` | ✅ | ✅ green |
| 09-03-01 | 03 | 1 | SC-09-5 | T-09-03-01 | README value proposition present | content | `go test ./docs/... -run TestSC095` | ✅ | ✅ green |
| 09-03-02 | 03 | 1 | SC-09-6 | T-09-03-01 | README has all 4 install methods | content | `go test ./docs/... -run TestSC096` | ✅ | ✅ green |
| 09-03-03 | 03 | 1 | SC-09-7 | T-09-03-01 | README covers 3 usage scenarios | content | `go test ./docs/... -run TestSC097` | ✅ | ✅ green |
| 09-03-04 | 03 | 1 | SC-09-11 | — | README links to all 4 supporting docs | content | `go test ./docs/... -run TestSC0911` | ✅ | ✅ green |
| 09-03-05 | 03 | 1 | SC-09-12 | — | README badges: CI, release, coverage | content | `go test ./docs/... -run TestSC0912` | ✅ | ✅ green |
| 09-04-01 | 04 | 1 | SC-09-8 | T-09-04-01 | COMPARISON.md has 8-tool × 9-dim table | content | `go test ./docs/... -run TestSC098` | ✅ | ✅ green |
| 09-04-02 | 04 | 1 | SC-09-9 | — | PREREQUISITES.md has ssh-keygen + sudo | content | `go test ./docs/... -run TestSC099` | ✅ | ✅ green |
| 09-04-03 | 04 | 1 | SC-09-10 | — | TROUBLESHOOTING.md has 5 failure scenarios | content | `go test ./docs/... -run TestSC0910` | ✅ | ✅ green |

*Status: ⬜ pending · ✅ green · ❌ red · ⚠️ flaky*

---

## Wave 0 Requirements

Existing infrastructure covers all phase requirements. Test files created by Nyquist audit:

- `docs/distribution_test.go` — SC-09-1, SC-09-2, SC-09-3
- `docs/documentation_test.go` — SC-09-5 through SC-09-12
- `docs/docs.go` — package stub

---

## Manual-Only Verifications

| Behavior | Requirement | Why Manual | Test Instructions |
|----------|-------------|------------|-------------------|
| All three install methods produce working `docker deploy --help` | SC-09-4 | Requires a real tagged release pushed to GitHub; Homebrew, curl\|sh, and GitHub Releases cannot be exercised without a live release artifact | After first `v*` tag push: (1) `brew tap webcane/docker-deploy && brew install docker-deploy && ln -sf $(brew --prefix)/bin/docker-deploy ~/.docker/cli-plugins/docker-deploy && docker deploy --help` (2) `curl https://raw.githubusercontent.com/webcane/docker-deploy/main/install.sh \| sh && docker deploy --help` (3) Manual binary download from GitHub Releases → chmod +x → move to ~/.docker/cli-plugins/ → `docker deploy --help`. All three must print Docker plugin help. |

---

## Validation Audit 2026-06-04

| Metric | Count |
|--------|-------|
| Gaps found | 11 automatable + 1 manual-only |
| Resolved (automated) | 11 |
| Escalated to manual-only | 1 (SC-09-4) |

---

## Validation Sign-Off

- [x] All tasks have `<automated>` verify or Wave 0 dependencies
- [x] Sampling continuity: no 3 consecutive tasks without automated verify
- [x] Wave 0 covers all MISSING references
- [x] No watch-mode flags
- [x] Feedback latency < 5s
- [x] `nyquist_compliant: true` set in frontmatter

**Approval:** approved 2026-06-04
