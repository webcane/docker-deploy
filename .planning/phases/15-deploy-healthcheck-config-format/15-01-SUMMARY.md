---
phase: 15-deploy-healthcheck-config-format
plan: "01"
subsystem: config
tags: [healthcheck, config, duration-parsing, four-tier-precedence]
dependency_graph:
  requires: []
  provides: [HealthcheckConfig struct, four-tier Resolve() with globalFile, --healthcheck-* flags]
  affects: [internal/config/config.go, internal/config/config_test.go, cmd/docker-deploy/main.go, internal/health/poll.go, internal/health/poll_test.go]
tech_stack:
  added: [time.Duration parsing via time.ParseDuration for duration strings]
  patterns: [four-tier precedence flag>local>global>zero, no hardcoded defaults, Docker-style duration strings]
key_files:
  created: []
  modified:
    - internal/config/config.go
    - internal/config/config_test.go
    - cmd/docker-deploy/main.go
    - internal/health/poll.go
    - internal/health/poll_test.go
decisions:
  - "D-04 honoured: no hardcoded defaults in code; absent healthcheck block in all tiers produces zero-value HealthcheckConfig"
  - "D-05 honoured: old health_timeout/health_interval yaml keys hard-removed; yaml.v3 silently ignores unknown fields"
  - "D-06 honoured: four-tier precedence (flag > local deploy.yaml > global config > zero) implemented in Resolve()"
  - "healthcheckYAML unexported struct used for YAML parsing; HealthcheckConfig exported struct used for runtime form"
  - "loadGlobalConfig() helper in main.go reads ~/.docker/cli-plugins/deploy.yaml; missing file = empty FileConfig, malformed file = fatal"
metrics:
  duration: "~56 minutes"
  completed: "2026-05-30"
  tasks: 3
  files_modified: 5
---

# Phase 15 Plan 01: Deploy Healthcheck Config Format Summary

Replaced flat `health_timeout`/`health_interval` integer keys with a structured `target.healthcheck:` block using Docker-style duration strings (`interval`, `timeout`, `retries`), wired three matching CLI flags, extended `Resolve()` with a `globalFile` parameter for four-tier precedence, and loaded the global config in main.go at all three call sites.

## Tasks Completed

| # | Task | Commit | Files |
|---|------|--------|-------|
| 1 | Add HealthcheckConfig struct; update TargetConfig/Config/FlagOpts | 7e24bd4 | internal/config/config.go |
| 2 | Extend Resolve() with globalFile; four-tier healthcheck resolution; migrate config/health tests | 8cb3b4e | internal/config/config.go, internal/config/config_test.go, internal/health/poll.go, internal/health/poll_test.go |
| 3 | Register --healthcheck-{timeout,interval,retries} flags; load global config in main.go | bff63d5 | cmd/docker-deploy/main.go |

## What Was Built

- `HealthcheckConfig` struct with `Interval time.Duration`, `Timeout time.Duration`, `Retries int` (runtime form)
- `healthcheckYAML` unexported struct with string-typed fields for YAML parsing
- `Resolve()` signature extended to `func Resolve(opts FlagOpts, file FileConfig, globalFile FileConfig, projectName string, localDir string)`
- Four-tier healthcheck resolution: flag > local deploy.yaml > global config > zero value
- Duration string parsing via `time.ParseDuration` with negative-value rejection (T-15-01-02)
- Source-naming error messages: `--healthcheck-interval`, `deploy.yaml: healthcheck.interval`, `global config: healthcheck.interval`
- `loadGlobalConfig()` helper in main.go: reads `~/.docker/cli-plugins/deploy.yaml`; missing file = empty `FileConfig{}`, malformed file = fatal error
- Three new CLI flags: `--healthcheck-timeout` (string), `--healthcheck-interval` (string), `--healthcheck-retries` (int)
- 14 new config tests covering: valid durations, invalid durations, negative durations, negative retries, absent block (zero value), flag>local, local>global, global-only

## Deviations from Plan

### Auto-fixed Issues

**1. [Rule 3 - Blocking] Update internal/health/poll.go to use new Healthcheck fields**

- **Found during:** Task 2 (go build ./... after implementing Resolve())
- **Issue:** `poll.go` referenced `cfg.HealthTimeout` and `cfg.HealthInterval` (removed int fields); `go build ./...` failed with undefined field errors
- **Fix:** Updated `pollHealthWithRunner()` to use `cfg.Healthcheck.Interval` and `cfg.Healthcheck.Timeout` (time.Duration values directly, no cast needed); updated timeout error message to use `%s` format for duration
- **Files modified:** `internal/health/poll.go`
- **Commit:** 8cb3b4e

**2. [Rule 3 - Blocking] Update internal/health/poll_test.go defaultCfg() helper**

- **Found during:** Task 2 (go test ./internal/health/...)
- **Issue:** `poll_test.go` `defaultCfg()` used `HealthTimeout`/`HealthInterval` int fields in struct literal
- **Fix:** Rewrote `defaultCfg()` to use `config.HealthcheckConfig{Timeout: time.Duration(s)*time.Second, Interval: time.Duration(s)*time.Second}`
- **Files modified:** `internal/health/poll_test.go`
- **Commit:** 8cb3b4e

## Verification

- `go build ./...` exits 0
- `go test ./...` exits 0 (all packages)
- `grep -rn "HealthTimeout\|HealthInterval\|health_timeout\|health_interval" internal/config/ cmd/docker-deploy/` returns no matches
- All three `config.Resolve()` call sites in main.go pass `globalCfg` as third argument

## Known Stubs

None — all functionality is fully implemented.

## Threat Flags

No new network endpoints, auth paths, or trust boundaries introduced. The global config path (`~/.docker/cli-plugins/deploy.yaml`) was already identified in the plan's threat model (T-15-01-05); it is mitigated by same-trust-level file ownership and malformed-YAML fatal error.

## Self-Check: PASSED

- internal/config/config.go exists and contains `type HealthcheckConfig struct` ✓
- cmd/docker-deploy/main.go exists and contains `healthcheck-timeout` ✓
- Commits 7e24bd4, 8cb3b4e, bff63d5 exist ✓
- `go test ./...` all pass ✓
