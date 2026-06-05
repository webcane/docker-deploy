---
phase: 15-deploy-healthcheck-config-format
plan: "03"
subsystem: config, cmd
tags: [strict-yaml, healthcheck, dry-run, KnownFields, formatHealthcheckRow]
dependency_graph:
  requires: [15-01, 15-02]
  provides: [strict YAML parsing, formatHealthcheckRow, dry-run healthcheck output]
  affects: [internal/config/config.go, cmd/docker-deploy/main.go]
tech_stack:
  added: []
  patterns: [KnownFields(true) strict YAML decoder, zero-value disabled sentinel, TDD RED/GREEN]
key_files:
  created: []
  modified:
    - internal/config/config.go
    - cmd/docker-deploy/main.go
    - cmd/docker-deploy/main_test.go
decisions:
  - "KnownFields(true) on yaml.Decoder rejects unknown keys in deploy.yaml; fails fast with informative error (D-05/D-11)"
  - "formatHealthcheckRow treats all-zero HealthcheckConfig as disabled (D-04 zero-value convention)"
  - "runDryRun prints Healthcheck row after Status line unconditionally; shows 'disabled' when no hc block present"
metrics:
  duration: "~15 minutes"
  completed: "2026-05-31"
  tasks: 2
  files_modified: 3
---

# Phase 15 Plan 03: Strict YAML Parsing and Dry-Run Healthcheck Output Summary

Strict `KnownFields(true)` YAML parsing added to `LoadFile()` to reject typos/unknown keys in `deploy.yaml`; `formatHealthcheckRow` helper added to `main.go` to render the healthcheck row in `--dry-run` output.

## Tasks Completed

| # | Task | Commit | Files |
|---|------|--------|-------|
| RED | Add failing tests for strict YAML parsing | 1e1fccf | internal/config/config_test.go |
| 1 | Implement KnownFields(true) in LoadFile() | d6831cc | internal/config/config.go |
| RED | Add failing test for formatHealthcheckRow | 9556e03 | cmd/docker-deploy/main_test.go |
| 2 | Implement formatHealthcheckRow + dry-run row | 1bf99a7 | cmd/docker-deploy/main.go |

## What Was Built

- `LoadFile()` in `internal/config/config.go` now uses `yaml.NewDecoder(f)` with `.KnownFields(true)` instead of `yaml.Unmarshal`; unknown top-level keys and unknown `healthcheck` sub-keys now return an error with the yaml decoder's diagnostic message.
- `formatHealthcheckRow(hc config.HealthcheckConfig) string` helper in `cmd/docker-deploy/main.go`:
  - Returns `"  Healthcheck:  disabled"` when `hc.Interval == 0 && hc.Timeout == 0 && hc.Retries == 0`
  - Returns `"  Healthcheck:  interval=Xs timeout=Ys retries=N"` otherwise (using `Duration.String()`)
- `runDryRun()` now calls `fmt.Fprintln(os.Stdout, formatHealthcheckRow(resolved.Healthcheck))` after the `Status: OK` line
- Three new config tests: `TestLoadFile_UnknownHealthcheckKey`, `TestLoadFile_UnknownTopLevelKey`, `TestLoadFile_ValidHealthcheckParsed`
- Three new `TestFormatHealthcheckRow` sub-tests: disabled (zero), enabled (non-zero), partial (interval only)

## Deviations from Plan

None — plan executed exactly as specified. All test expectations matched first-try with `Duration.String()` formatting (`30s`, `1m0s`, `0s`).

## Verification

- `go test ./internal/config/... -run TestLoadFile` all pass (7 test cases)
- `go test ./cmd/docker-deploy/... -run TestFormatHealthcheckRow` all pass (3 sub-tests)
- `go build ./...` exits 0
- `go vet ./...` exits 0

## Known Stubs

None.

## Threat Flags

No new network endpoints, auth paths, or trust boundaries introduced.
- Strict YAML parsing closes a configuration injection surface: previously unknown keys were silently ignored, allowing typos to cause silent misconfiguration.

## Self-Check: PASSED

- `internal/config/config.go` uses `KnownFields(true)` ✓
- `cmd/docker-deploy/main.go` contains `formatHealthcheckRow` ✓
- `runDryRun()` calls `formatHealthcheckRow(resolved.Healthcheck)` after Status line ✓
- Commits d6831cc, 9556e03, 1bf99a7 exist ✓
- `go test ./...` all pass ✓
