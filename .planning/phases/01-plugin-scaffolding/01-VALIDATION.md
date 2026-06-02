---
phase: 1
slug: plugin-scaffolding
status: complete
nyquist_compliant: true
wave_0_complete: true
created: 2026-06-02
audited: 2026-06-02
---

# Phase 1 — Validation Strategy

> Per-phase validation contract. Reconstructed from PLAN and SUMMARY artifacts (State B).

---

## Test Infrastructure

| Property | Value |
|----------|-------|
| **Framework** | Go standard testing (`testing` package) |
| **Config file** | `go.mod` (root) |
| **Quick run command** | `go test ./cmd/docker-deploy/` |
| **Full suite command** | `go test ./...` |
| **Estimated runtime** | ~2 seconds |

---

## Sampling Rate

- **After every task commit:** Run `go test ./cmd/docker-deploy/`
- **After every plan wave:** Run `go test ./...`
- **Before `/gsd-verify-work`:** Full suite must be green
- **Max feedback latency:** ~2 seconds

---

## Per-Task Verification Map

| Task ID | Plan | Wave | Requirement | Threat Ref | Secure Behavior | Test Type | Automated Command | File Exists | Status |
|---------|------|------|-------------|------------|-----------------|-----------|-------------------|-------------|--------|
| 01-01-T1 | 01 | 1 | PLUG-01 / PLUG-02 / PLUG-03 | T-01-01 | `go.mod` module path correct; docker/cli dep pinned first | structural | `grep -c 'module github.com/mniedre/docker-deploy' go.mod && grep -c 'github.com/docker/cli' go.mod` | ✅ | ✅ green |
| 01-01-T2 | 01 | 1 | PLUG-02 | — | `buildDeployCmd().Use == "deploy"` and Short non-empty | unit | `go test ./cmd/docker-deploy/ -run TestDeployCmd_UseAndShortDescription` | ✅ | ✅ green |
| 01-01-T2 | 01 | 1 | PLUG-03 | T-01-03 | `pluginMetadata.SchemaVersion == "0.1.0"`, Vendor/Short non-empty, Version references ldflags var | unit | `go test ./cmd/docker-deploy/ -run TestPluginMetadata_ContractFields` | ✅ | ✅ green |
| 01-01-T3 | 01 | 1 | PLUG-01 / PLUG-02 | T-01-04 | `make build` produces binary; `make test` exits 0 | build | `make build && make test` | ✅ | ✅ green |
| 01-02-T1 | 02 | 2 | PLUG-03 | T-02-01 | `.goreleaser.yaml` targets linux only; ldflags inject `main.version` | structural | `grep -c 'linux' .goreleaser.yaml && grep -c 'main.version' .goreleaser.yaml` | ✅ | ✅ green |
| 01-02-T2 | 02 | 2 | PLUG-01 / PLUG-02 | T-02-03 | CI workflow triggers on push+PR; release workflow scoped to v* tags; `permissions: contents: write` | structural | `grep -c 'go test' .github/workflows/ci.yml && grep -c 'goreleaser' .github/workflows/release.yml` | ✅ | ✅ green |

*Status: ⬜ pending · ✅ green · ❌ red · ⚠️ flaky*

---

## Wave 0 Requirements

Existing infrastructure covers all phase requirements. No new test files were needed — tests were added to the existing `cmd/docker-deploy/main_test.go`.

---

## Manual-Only Verifications

| Behavior | Requirement | Why Manual | Test Instructions |
|----------|-------------|------------|-------------------|
| `docker deploy --help` is discoverable in Docker CLI | PLUG-01 | Requires live Docker CLI binary + binary installed in `~/.docker/cli-plugins/` — no Docker API in CI unit test environment | Run `make install` then `docker deploy --help`; verify "Deploy a docker-compose project" appears in output |
| GoReleaser produces correctly named archives on v* tag push | PLUG-01 (release artifact) | Requires GitHub Actions runner + tag push — not reproducible in local unit tests | Push a `v*` tag and verify GitHub Release contains `docker-deploy_linux_amd64.tar.gz`, `docker-deploy_linux_arm64.tar.gz`, and `checksums.txt` |

---

## Validation Audit 2026-06-02

| Metric | Count |
|--------|-------|
| Gaps found | 3 |
| Resolved (automated) | 2 |
| Escalated to manual-only | 1 |

**Gaps resolved:**
- PLUG-02: `TestDeployCmd_UseAndShortDescription` — verifies `buildDeployCmd().Use == "deploy"` and `Short` non-empty
- PLUG-03: `TestPluginMetadata_ContractFields` — verifies `pluginMetadata` struct fields; catches silent ldflags injection failure

**Escalated to manual-only:**
- PLUG-01 (Docker CLI runtime discovery): Cannot be automated without a live Docker daemon and installed binary — verified manually during original phase execution (2026-05-13)

---

## Validation Sign-Off

- [x] All tasks have automated verify or documented manual-only justification
- [x] Sampling continuity: no 3 consecutive tasks without automated verify
- [x] Wave 0: no new test files needed; infrastructure pre-existing
- [x] No watch-mode flags
- [x] Feedback latency < 5s
- [x] `nyquist_compliant: true` set in frontmatter

**Approval:** approved 2026-06-02
