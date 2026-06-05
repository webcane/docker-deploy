---
plan: 11-02
phase: 11-ci-tooling-polish
status: complete
completed: 2026-05-23
commit: 85f1a93
---

# Plan 11-02: Remove FORCE Workaround + Add Dependabot

## What Was Built

Removed the `FORCE_JAVASCRIPT_ACTIONS_TO_NODE24` workaround from all workflow files and created Dependabot config for automated weekly GitHub Actions version updates.

## Key Files Modified/Created

- `.github/workflows/ci.yml` — removed `env: FORCE_JAVASCRIPT_ACTIONS_TO_NODE24: "true"` from both `test` and `integration` jobs; `TESTCONTAINERS_RYUK_DISABLED` step-level env preserved
- `.github/workflows/release.yml` — removed `env: FORCE_JAVASCRIPT_ACTIONS_TO_NODE24: "true"` from `release` job; `permissions`, `GITHUB_TOKEN`, and `HOMEBREW_TAP_TOKEN` preserved
- `.github/dependabot.yml` — new file; `package-ecosystem: github-actions`, `directory: "/"`, `schedule: weekly`, `open-pull-requests-limit: 5`

## Deviations

None. Action versions already at current major versions (checkout@v4, setup-go@v5, goreleaser-action@v6, cosign-installer@v3), satisfying the D-09 precondition.

## Self-Check: PASSED

- `FORCE_JAVASCRIPT_ACTIONS_TO_NODE24` absent from all workflow files ✓
- `.github/dependabot.yml` exists with `github-actions` ecosystem and weekly schedule ✓
- All other workflow content (permissions, action versions, step-level env vars) unchanged ✓
