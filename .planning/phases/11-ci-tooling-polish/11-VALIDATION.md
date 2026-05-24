---
phase: 11
slug: ci-tooling-polish
status: complete
nyquist_compliant: true
wave_0_complete: false
created: 2026-05-24
audited: 2026-05-24
---

# Phase 11 — Validation Strategy

> Nyquist validation audit reconstructed from plan artifacts and live codebase verification.
> All requirements verified as COVERED. No test gaps found.

---

## Test Infrastructure

| Property | Value |
|----------|-------|
| **Framework** | Shell assertions (grep, test -f) + `go test` + `golangci-lint` |
| **Config file** | `.golangci.yml` (linter gate); `codecov.yml` (coverage reporting) |
| **Quick run command** | `make lint` |
| **Full suite command** | `go test ./... && make lint` |
| **Estimated runtime** | ~15 seconds |

> Note: Phase 11 is a CI/tooling-only phase — no new Go business logic. Automated verification
> is shell-based file/content assertions plus `make lint`. This is the correct approach for
> infrastructure configuration changes.

---

## Sampling Rate

- **After every task commit:** Run `make lint`
- **After every plan wave:** Run `go test ./... && make lint`
- **Before `/gsd-verify-work`:** Full suite must be green
- **Max feedback latency:** ~15 seconds

---

## Per-Task Verification Map

| Task ID | Plan | Wave | Requirement | Threat Ref | Secure Behavior | Test Type | Automated Command | Status |
|---------|------|------|-------------|------------|-----------------|-----------|-------------------|--------|
| 11-01-T1 | 01 | 1 | codecov.yml exists with comment.require_changes | T-11-01-01 | N/A | shell | `test -f codecov.yml && grep -c "require_changes" codecov.yml \| grep -q "1"` | ✅ green |
| 11-01-T2a | 01 | 1 | CI test job generates and uploads coverage | T-11-01-02 | N/A | shell | `grep "codecov/codecov-action" .github/workflows/ci.yml && grep "coverprofile=coverage.out" .github/workflows/ci.yml` | ✅ green |
| 11-01-T2b | 01 | 1 | README Codecov badge references master branch | — | N/A | shell | `grep "codecov.io/gh/webcane/docker-deploy/branch/master/graph/badge" README.md` | ✅ green |
| 11-02-T1 | 02 | 1 | FORCE_JAVASCRIPT_ACTIONS_TO_NODE24 absent from all workflows | T-11-02-02 | N/A | shell | `grep -r "FORCE_JAVASCRIPT_ACTIONS_TO_NODE24" .github/workflows/ \| wc -l \| tr -d ' ' \| grep -q "^0$"` | ✅ green |
| 11-02-T2 | 02 | 1 | dependabot.yml exists with weekly github-actions schedule | T-11-02-01 | N/A | shell | `test -f .github/dependabot.yml && grep -c "github-actions" .github/dependabot.yml \| grep -q "1" && grep "weekly" .github/dependabot.yml` | ✅ green |
| 11-03-T1a | 03 | 1 | .goreleaser.yaml post_install creates symlink | T-11-03-01 | N/A | shell | `grep "post_install" .goreleaser.yaml` | ✅ green |
| 11-03-T1b | 03 | 1 | README Homebrew section has no manual ln-sf | — | N/A | shell | `! grep -q "ln -sf \$(brew --prefix)" README.md` | ✅ green |
| 11-04-T1 | 04 | 1 | .golangci.yml exists with 4 linters + local prefix | T-11-04-02 | N/A | shell | `test -f .golangci.yml && grep "errcheck" .golangci.yml && grep "staticcheck" .golangci.yml && grep "goimports" .golangci.yml && grep "webcane/docker-deploy" .golangci.yml` | ✅ green |
| 11-04-T1b | 04 | 1 | Makefile has lint and fmt targets | — | N/A | shell | `grep "golangci-lint" Makefile && grep "goimports" Makefile` | ✅ green |
| 11-04-T2 | 04 | 1 | CI lint job runs make lint | T-11-04-01 | N/A | shell | `grep -A15 "^  lint:" .github/workflows/ci.yml \| grep "make lint"` | ✅ green |
| 11-04-functional | 04 | 1 | golangci-lint passes on codebase with 0 issues | — | errcheck/govet/staticcheck all pass | linter | `make lint` | ✅ green |

*Status: ⬜ pending · ✅ green · ❌ red · ⚠️ flaky*

---

## Wave 0 Requirements

Not applicable — Phase 11 is CI/tooling-only. No new test stubs required. All verification is
shell-based config assertion plus the existing `make lint` gate.

---

## Manual-Only Verifications

| Behavior | Requirement | Why Manual | Test Instructions |
|----------|-------------|------------|-------------------|
| brew install creates ~/.docker/cli-plugins/docker-deploy symlink | D-12 (post_install) | Requires a Homebrew environment and a live release; cannot be tested locally against unreleased builds | `brew install webcane/docker-deploy/docker-deploy` and verify `ls ~/.docker/cli-plugins/docker-deploy` |
| brew uninstall removes symlink | D-13 (descoped) | `def uninstall` is not valid Homebrew Formula DSL (only Cask DSL). Intentionally replaced with a caveats block. See VERIFICATION.md override. | Manual: `rm -f ~/.docker/cli-plugins/docker-deploy` after `brew uninstall` |
| CI pipeline runs green end-to-end | Phase goal | Requires GitHub Actions runtime; cannot be replicated locally | Check CI badge or `gh run list --limit 5` |

---

## Validation Audit 2026-05-24

| Metric | Count |
|--------|-------|
| Requirements analyzed | 10 |
| COVERED | 10 |
| PARTIAL | 0 |
| MISSING | 0 |
| Manual-only | 3 (2 infeasible to automate, 1 intentionally descoped) |
| Gaps found | 0 |
| Tests generated | 0 (no gaps to fill) |

---

## Validation Sign-Off

- [x] All tasks have `<automated>` verify or are documented as manual-only
- [x] Sampling continuity: no 3 consecutive tasks without automated verify
- [x] Wave 0 not needed (CI/tooling phase)
- [x] No watch-mode flags
- [x] Feedback latency < 15s (`make lint` runs in ~3s)
- [x] `nyquist_compliant: true` set in frontmatter

**Approval:** approved 2026-05-24

---

## Validation Audit 2026-05-24 (re-verify)

| Metric | Count |
|--------|-------|
| Gaps found | 0 |
| Resolved | 0 |
| Escalated | 0 |
| Automated checks re-run | 10/10 PASS |
| `make lint` result | 0 issues |
