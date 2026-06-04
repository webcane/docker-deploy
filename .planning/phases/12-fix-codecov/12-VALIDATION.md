---
phase: "12"
slug: fix-codecov
status: partial
nyquist_compliant: false
wave_0_complete: false
created: 2026-06-04
---

# Phase 12 — Validation Strategy

> Per-phase validation contract: Phase 12 (Docs Polish) — documentation-only changes to plugin description, README, INSTALL.md, and COMPARISON.md.

---

## Test Infrastructure

| Property | Value |
|----------|-------|
| **Framework** | go test |
| **Config file** | none — standard `go test ./...` |
| **Quick run command** | `go test ./cmd/docker-deploy/... ./docs/...` |
| **Full suite command** | `go test ./...` |
| **Estimated runtime** | ~5 seconds |

---

## Sampling Rate

- **After every task commit:** Run `go test ./cmd/docker-deploy/... ./docs/...`
- **After every plan wave:** Run `go test ./...`
- **Before `/gsd-verify-work`:** Full suite must be green
- **Max feedback latency:** 10 seconds

---

## Per-Task Verification Map

| Task ID | Plan | Wave | Requirement | Threat Ref | Secure Behavior | Test Type | Automated Command | File Exists | Status |
|---------|------|------|-------------|------------|-----------------|-----------|-------------------|-------------|--------|
| 12-01-01 | 01 | 1 | D-03 | T-12-01-01 / — | No "remote VPS" in plugin description | grep | `grep -c "remote VPS" cmd/docker-deploy/main.go` → 0 | ✅ | ✅ green |
| 12-02-01 | 02 | 1 | D-04/D-05 | T-12-02-01 / — | Value prop ≤100 words; INSTALL.md link present | grep | `grep -c "INSTALL.md" README.md` → ≥1 | ✅ | ✅ green |
| 12-03-01 | 03 | 1 | D-06/D-07 | T-12-03-01 / — | INSTALL.md exists with 4 sections | grep | `grep -c "^## " INSTALL.md` → 4 | ✅ | ✅ green |
| 12-04-01 | 04 | 1 | D-08/D-09 | T-12-04-01 / — | "Missing a tool?" section at bottom of COMPARISON.md | grep | `grep -c "## Missing a tool?" COMPARISON.md` → 1 | ✅ | ✅ green |

*Status: ⬜ pending · ✅ green · ❌ red · ⚠️ flaky*

---

## Wave 0 Requirements

Existing infrastructure covers all phase requirements. All four tasks modify documentation files only; go test handles doc-content verification via `docs/documentation_test.go`.

---

## Manual-Only Verifications

| Behavior | Requirement | Why Manual | Test Instructions |
|----------|-------------|------------|-------------------|
| `ShortDescription` + `Short` both say "Deploy a docker-compose project to a remote host" (not "remote VPS") | D-03 | Existing tests only verify fields are non-empty, not the specific wording | Run `grep -c "remote VPS" cmd/docker-deploy/main.go` (expect 0) and `grep "remote host" cmd/docker-deploy/main.go` (expect 2 occurrences in ShortDescription and Short) |
| README install section contains ONLY install script; Homebrew/Manual binary/go install blocks are absent | D-04/D-05 | No automated test checks the absence of the three removed methods from README | Run `grep -c "brew tap\|brew install\|go install\|tar.gz" README.md` (expect 0) and confirm only `### Install script` section exists |
| README install section has no "Option N:" prefixes | D-06 | No automated test for prefix removal | Run `grep -c "Option 1\|Option 2\|Option 3\|Option 4" README.md` (expect 0) |
| INSTALL.md has exactly 4 `##` sections with simplified headers (no Option N: prefix) | D-06/D-07 | Existing TestSC096 checks methods are accessible but not INSTALL.md section structure | Run `grep "^## " INSTALL.md` and verify exactly: `## Install script`, `## Homebrew`, `## Manual binary`, `## go install` |
| COMPARISON.md ends with `## Missing a tool?` section containing GitHub Issues link | D-08/D-09 | No automated test covers the new feedback section | Run `grep -c "## Missing a tool?" COMPARISON.md` (expect 1) and `grep -c "github.com/webcane/docker-deploy/issues" COMPARISON.md` (expect 1) |

---

## Validation Audit 2026-06-04

| Metric | Count |
|--------|-------|
| Gaps found | 5 |
| Resolved | 0 |
| Escalated (manual-only) | 5 |

---

## Validation Sign-Off

- [x] All tasks have `<automated>` verify commands (grep-based in-plan verification)
- [x] Sampling continuity: all four Wave 1 tasks verified at commit time
- [x] Wave 0: no new test infrastructure required — docs/ framework already in place
- [x] No watch-mode flags
- [x] Feedback latency < 10s
- [ ] `nyquist_compliant: true` — NOT set; 5 behaviors are manual-only (documentation content assertions)

**Approval:** pending — 5 manual-only items remain; automated wording tests not generated per user decision 2026-06-04
