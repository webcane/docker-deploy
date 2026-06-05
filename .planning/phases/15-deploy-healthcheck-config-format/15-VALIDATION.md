---
phase: 15
slug: deploy-healthcheck-config-format
status: compliant
nyquist_compliant: true
wave_0_complete: false
created: 2026-06-05
---

# Phase 15 — Validation Strategy

> Per-phase validation contract for feedback sampling during execution.

---

## Test Infrastructure

| Property | Value |
|----------|-------|
| **Framework** | go test |
| **Config file** | go.mod |
| **Quick run command** | `go test ./internal/config/... ./internal/health/... ./cmd/docker-deploy/...` |
| **Full suite command** | `go test ./...` |
| **Estimated runtime** | ~12 seconds |

---

## Sampling Rate

- **After every task commit:** Run `go test ./internal/config/... ./internal/health/... ./cmd/docker-deploy/...`
- **After every plan wave:** Run `go test ./...`
- **Before `/gsd-verify-work`:** Full suite must be green
- **Max feedback latency:** 12 seconds

---

## Per-Task Verification Map

| Task ID | Plan | Wave | Requirement | Threat Ref | Secure Behavior | Test Type | Automated Command | File Exists | Status |
|---------|------|------|-------------|------------|-----------------|-----------|-------------------|-------------|--------|
| 15-01-01 | 01 | 1 | CFG-07 | — | HealthcheckConfig struct with Interval/Timeout/Retries; old flat keys removed | unit | `go test ./internal/config/... -run TestResolveHealthcheck_ValidDurationStrings` | ✅ | ✅ green |
| 15-01-02 | 01 | 1 | CFG-07 | T-15-01-05 | Four-tier precedence (flag>local>global>zero); invalid/negative durations rejected with source-naming errors | unit | `go test ./internal/config/... -run TestResolveHealthcheck` | ✅ | ✅ green |
| 15-01-03 | 01 | 1 | CFG-07 | T-15-01-05 | CLI flags --healthcheck-{timeout,interval,retries} registered; loadGlobalConfig() loads ~/.docker/cli-plugins/deploy.yaml; missing file tolerated | unit | `go test ./internal/config/... -run TestResolveHealthcheck_GlobalFileUsedWhenFlagAndLocalEmpty` | ✅ | ✅ green |
| 15-02-01 | 02 | 2 | HEALTH-03 | T-15-02-01 | Per-container failCount map; retries>0 defers fail; retries==0 preserves immediate-fail; timeout prints Duration.String() | unit | `go test ./internal/health/... -run TestPollHealth_Retries` | ✅ | ✅ green |
| 15-02-02 | 02 | 2 | HEALTH-01 | — | integration/compose_test.go migrated from removed flat fields to HealthcheckConfig struct; full repo compiles | build | `go build ./...` | ✅ | ✅ green |
| 15-03-01 | 03 | 3 | CFG-07 | — | KnownFields(true) in LoadFile() rejects unknown top-level and healthcheck sub-keys with diagnostic error | unit | `go test ./internal/config/... -run TestLoadFile_Unknown` | ✅ | ✅ green |
| 15-03-02 | 03 | 3 | CFG-07 | — | formatHealthcheckRow renders disabled/enabled; runDryRun() always includes Healthcheck row | unit | `go test ./cmd/docker-deploy/... -run TestFormatHealthcheckRow` | ✅ | ✅ green |

*Status: ⬜ pending · ✅ green · ❌ red · ⚠️ flaky*

---

## Wave 0 Requirements

Existing infrastructure covers all phase requirements.

---

## Manual-Only Verifications

All phase behaviors have automated verification.

---

## Validation Audit 2026-06-05

| Metric | Count |
|--------|-------|
| Gaps found | 0 |
| Resolved | 0 |
| Escalated | 0 |

All 7 tasks (Plans 01–03) have automated test coverage. 22 new tests added across:
- `internal/config/config_test.go`: 18 healthcheck-specific tests (four-tier precedence, invalid/negative values, strict YAML parsing)
- `internal/health/poll_test.go`: 3 retries-semantics tests (threshold reached, reset-on-healthy, retries=0 compat)
- `cmd/docker-deploy/main_test.go`: 1 `TestFormatHealthcheckRow` with 3 sub-tests

---

## Validation Sign-Off

- [x] All tasks have `<automated>` verify or Wave 0 dependencies
- [x] Sampling continuity: no 3 consecutive tasks without automated verify
- [x] Wave 0 covers all MISSING references
- [x] No watch-mode flags
- [x] Feedback latency < 12s
- [x] `nyquist_compliant: true` set in frontmatter

**Approval:** approved 2026-06-05
