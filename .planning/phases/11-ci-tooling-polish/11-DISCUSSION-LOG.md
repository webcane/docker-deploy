# Phase 11: CI & Tooling Polish - Discussion Log

> **Audit trail only.** Do not use as input to planning, research, or execution agents.
> Decisions are captured in CONTEXT.md — this log preserves the alternatives considered.

**Date:** 2026-05-23
**Phase:** 11-CI & Tooling Polish
**Areas discussed:** Linting & Formatting, Codecov setup, GitHub Actions hygiene, Brew symlink automation

---

## Linting & Formatting

| Option | Description | Selected |
|--------|-------------|----------|
| .golangci.yml file | Checked into repo; controls which linters run, thresholds, per-file exclusions | ✓ |
| Default (no config file) | Built-in defaults, zero config | |

**User's choice:** .golangci.yml config file

---

| Option | Description | Selected |
|--------|-------------|----------|
| Yes, lint job in CI | Gate that fails the build; separate job alongside test | ✓ |
| Makefile only | Local only, CI doesn't enforce | |

**User's choice:** Add lint job to CI as a gate

---

| Option | Description | Selected |
|--------|-------------|----------|
| github.com/webcane/docker-deploy prefix | Third import group; clean separation from stdlib and third-party | ✓ |
| No local prefix | Mixed import grouping | |

**User's choice:** github.com/webcane/docker-deploy local prefix for goimports

---

| Option | Description | Selected |
|--------|-------------|----------|
| Minimal — errcheck, govet, staticcheck, goimports | High-value checks, low noise for small CLI | ✓ |
| Broad — additional gosimple, ineffassign, unused, misspell | More coverage, more noise | |
| You decide | Claude picks for small Go CLI | |

**User's choice:** Minimal linter set (errcheck, govet, staticcheck, goimports)

---

## Codecov Setup

| Option | Description | Selected |
|--------|-------------|----------|
| Tokenless (public repo) | Works automatically, no secret to manage | ✓ |
| CODECOV_TOKEN secret | More reliable for large/private repos | |

**User's choice:** Tokenless upload

---

| Option | Description | Selected |
|--------|-------------|----------|
| Unit tests only | Fast, simple; integration tests separate | ✓ |
| Unit + integration merged | More complete but complex | |

**User's choice:** Unit tests only

---

| Option | Description | Selected |
|--------|-------------|----------|
| Yes — minimal codecov.yml | Controls PR diff coverage comments | ✓ |
| No config file | Verbose default PR comments | |

**User's choice:** Add minimal codecov.yml config file

---

## GitHub Actions Hygiene

| Option | Description | Selected |
|--------|-------------|----------|
| Remove FORCE_JAVASCRIPT_ACTIONS_TO_NODE24 after bumps | Clean up the workaround | ✓ |
| Keep as belt-and-suspenders | Harmless to leave in | |

**User's choice:** Remove workaround after real bumps

---

| Option | Description | Selected |
|--------|-------------|----------|
| Major version tags @vN | Conventional, auto-receives non-breaking updates | ✓ |
| Specific minor/patch @vN.N.N | Reproducible, manual updates needed | |

**User's choice:** Major version tags

---

| Option | Description | Selected |
|--------|-------------|----------|
| Yes — Dependabot for github-actions | Weekly PRs for action version updates | ✓ |
| No Dependabot | Manual bumps only | |

**User's choice:** Add Dependabot for github-actions (weekly schedule)

---

## Brew Symlink Automation

| Option | Description | Selected |
|--------|-------------|----------|
| post_install creates symlink | Auto-creates ~/.docker/cli-plugins/docker-deploy | ✓ |
| Keep caveats only | Manual instructions, user runs symlink command | |

**User's choice:** post_install creates symlink automatically

---

| Option | Description | Selected |
|--------|-------------|----------|
| def uninstall via custom_block | Removes symlink on brew uninstall | ✓ |
| No uninstall cleanup | Symlink stays after uninstall | |

**User's choice:** Inject def uninstall via custom_block

---

| Option | Description | Selected |
|--------|-------------|----------|
| Remove caveats entirely | post_install handles everything; caveats become misleading | ✓ |
| Update caveats to confirm location | Keep a short confirmation message | |

**User's choice:** Remove caveats block entirely

---

## Claude's Discretion

None — all areas had explicit user selections.

## Deferred Ideas

None.
