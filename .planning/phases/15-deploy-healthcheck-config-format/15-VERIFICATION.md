---
phase: 15-deploy-healthcheck-config-format
verified: 2026-05-30T00:00:00Z
status: passed
score: 14/14 must-haves verified
overrides_applied: 0
re_verification: false
gap_closure:
  plan: "15-03"
  verified: 2026-05-31T00:00:00Z
  status: passed
  score: 3/3 gap-closure must-haves verified
  gaps_closed:
    - "A typo in a YAML healthcheck key (e.g. 'retrise' instead of 'retries') returns a parse error, not silent zero"
    - "An unknown top-level key in deploy.yaml returns a parse error"
    - "--dry-run output always includes a Healthcheck row showing resolved interval/timeout/retries or 'disabled'"
  gaps_remaining: []
---

# Phase 15: Deploy Healthcheck Config Format — Verification Report

**Phase Goal:** Replace flat health_timeout/health_interval integer keys with a proper target.healthcheck sub-block using Docker-style duration strings, add retries field, wire CLI flags, and implement per-container retries semantics in the health poller.
**Verified:** 2026-05-30
**Status:** PASSED
**Re-verification:** No — initial verification

## Goal Achievement

### Observable Truths

| #  | Truth | Status | Evidence |
|----|-------|--------|---------|
| 1  | deploy.yaml accepts target.healthcheck.{interval,timeout,retries} with duration strings and integer retries | VERIFIED | `healthcheckYAML` struct with yaml tags in `TargetConfig.Healthcheck`; `HealthcheckConfig` exported runtime struct |
| 2  | Old flat keys health_timeout/health_interval fully removed from TargetConfig, Config, FlagOpts | VERIFIED | grep of `HealthTimeout\|HealthInterval\|health_timeout\|health_interval` in internal/ and cmd/ returns 0 matches |
| 3  | CLI flags --healthcheck-timeout, --healthcheck-interval (duration strings), --healthcheck-retries (int) registered | VERIFIED | Lines 82-84 of main.go; `StringVar`/`IntVar` with correct names; 6 FlagOpts field references |
| 4  | Resolution precedence: flag > local deploy.yaml > global config > zero (four-tier per D-06) | VERIFIED | Switch blocks in `Resolve()` for Interval, Timeout, and Retries each check opts → file → globalFile → leave zero |
| 5  | main.go loads ~/.docker/cli-plugins/deploy.yaml via loadGlobalConfig() before Resolve(); missing file = empty FileConfig | VERIFIED | `loadGlobalConfig()` helper lines 190-205; called at all three Resolve() sites (lines 174, 254, 338); uses `os.IsNotExist` tolerance |
| 6  | Invalid duration strings, negative parsed durations, negative retries rejected with source-naming errors | VERIFIED | 7 `time.ParseDuration` calls; 6 `duration must be >= 0` error strings; 5 `global config: healthcheck` error messages; source named in each |
| 7  | No hardcoded healthcheck default values in config.go (per D-04) | VERIFIED | No 60s/5s/10s/30s/3 literals in healthcheck resolution code; zero-value path is always the fallback |
| 8  | pollHealthWithRunner reads cfg.Healthcheck.Interval / cfg.Healthcheck.Timeout as time.Duration directly (no int→Duration multiplication) | VERIFIED | 2 references to `cfg.Healthcheck.Interval`, 4 to `cfg.Healthcheck.Timeout`; grep for `time.Duration(cfg` returns 0 |
| 9  | Per-container failCount map tracks consecutive unhealthy results; counter resets when status becomes healthy | VERIFIED | `failCount` declared in `pollHealthWithRunner`, 16 references in poll.go; reset `failCount[container] = 0` in healthy/no-healthcheck case |
| 10 | retries > 0 activates deferred-fail gate; fails when failCount >= retries | VERIFIED | `if retries > 0 { failCount[container]++; if failCount[container] >= retries { ... } }` block in pollContainers |
| 11 | retries == 0 preserves immediate-fail behaviour (backwards compat) | VERIFIED | `else` branch with direct `return false, fmt.Errorf("health: container %s is unhealthy", ...)` |
| 12 | Timeout error message prints configured duration via .String() | VERIFIED | `fmt.Fprintf(os.Stderr, "Health check timed out after %s: container %s ...", cfg.Healthcheck.Timeout, c)` — %s invokes Duration.String() |
| 13 | Tests cover: retries threshold reached, retries counter reset on healthy, retries=0 immediate-fail | VERIFIED | `TestPollHealth_RetriesThresholdReached`, `TestPollHealth_RetriesResetOnHealthy`, `TestPollHealth_NoRetries_ImmediateFail` — all present and pass |
| 14 | integration/compose_test.go migrated from removed Config{HealthTimeout/HealthInterval} literals to Config{Healthcheck: HealthcheckConfig{}} | VERIFIED | 0 old field references; 3 `config.HealthcheckConfig{` literals at lines 73, 114, 145 |

**Score:** 14/14 truths verified

### Required Artifacts

| Artifact | Expected | Status | Details |
|----------|----------|--------|---------|
| `internal/config/config.go` | HealthcheckConfig struct, healthcheckYAML, updated TargetConfig/Config/FlagOpts, Resolve() with globalFile | VERIFIED | All structs present; Resolve() signature `(opts FlagOpts, file FileConfig, globalFile FileConfig, projectName string, localDir string)` |
| `cmd/docker-deploy/main.go` | --healthcheck-{timeout,interval,retries} flag registration; loadGlobalConfig() helper; three Resolve() call sites with globalCfg | VERIFIED | Flags registered lines 82-84; helper lines 190-205; Resolve() at lines 180, 261, 344 all pass globalCfg |
| `internal/config/config_test.go` | Tests for new healthcheck block format, four-tier precedence, invalid/negative value rejection | VERIFIED | 14 healthcheck-specific test functions covering all required cases including global tier, negative durations, absent block |
| `internal/health/poll.go` | Updated pollHealthWithRunner with retries semantics and time.Duration cfg fields | VERIFIED | failCount map, per-container increment/reset, retries gate, Duration field access |
| `internal/health/poll_test.go` | Tests for retries threshold, reset-on-healthy, retries=0 immediate-fail | VERIFIED | 3 new retries tests; defaultCfg() uses HealthcheckConfig struct |
| `integration/compose_test.go` | Migrated from HealthTimeout/HealthInterval to HealthcheckConfig | VERIFIED | 0 old fields, 3 migrated sites |

### Key Link Verification

| From | To | Via | Status | Details |
|------|----|-----|--------|---------|
| cmd/docker-deploy/main.go | config.FlagOpts.HealthcheckTimeout/Interval/Retries | cmd.Flags().StringVar / IntVar | VERIFIED | Lines 82-84 with correct field bindings |
| cmd/docker-deploy/main.go | config.Resolve(opts, fileCfg, globalCfg, ...) | loadGlobalConfig() before each Resolve() call | VERIFIED | All 3 call sites: runValidate (line 180), runDryRun (line 261-272), runDeploy (line 344-355) |
| internal/config/config.go Resolve() | time.ParseDuration | duration string parsing for flag, local file, and global file values | VERIFIED | 7 time.ParseDuration calls covering interval and timeout across all three tiers |
| internal/health/poll.go pollHealthWithRunner | cfg.Healthcheck.Interval / cfg.Healthcheck.Timeout (time.Duration) | direct field access — no Duration() multiplication | VERIFIED | grep `time.Duration(cfg` returns 0; cfg.Healthcheck.Interval/Timeout used directly |
| internal/health/poll.go pollContainers | failCount map[string]int | increment on unhealthy, reset on healthy, gate fail on >= Retries | VERIFIED | pollContainers signature `(..., failCount map[string]int, retries int)`; logic correctly implemented |

### Data-Flow Trace (Level 4)

Not applicable — phase produces config parsing logic and CLI flags, not UI components or data-rendering artifacts.

### Behavioral Spot-Checks

| Behavior | Command | Result | Status |
|----------|---------|--------|--------|
| Whole repo builds | `go build ./...` | exit 0, no output | PASS |
| Config unit tests pass | `go test ./internal/config/...` | ok 1.433s | PASS |
| Health unit tests pass | `go test ./internal/health/...` | ok 8.222s | PASS |
| Old flat keys absent across codebase | `grep -rn "HealthTimeout\|HealthInterval\|health_timeout\|health_interval" internal/ cmd/` | no output (0 matches) | PASS |
| CLI flags registered | `grep -c '"healthcheck-timeout"' cmd/docker-deploy/main.go` | 1 | PASS |
| Global config wired at all 3 Resolve() sites | `grep -c 'globalCfg' cmd/docker-deploy/main.go` | 8 | PASS |

### Probe Execution

No probes declared or applicable for this phase.

### Requirements Coverage

No requirement IDs were explicitly mapped to this phase in the plan frontmatter. This phase is a config format evolution that supports HEALTH-01/HEALTH-02/HEALTH-03 (from Phase 5) and CFG-07 (global defaults layer), but does not claim new requirement satisfaction.

### Anti-Patterns Found

| File | Line | Pattern | Severity | Impact |
|------|------|---------|----------|--------|
| integration/compose_test.go | 22 | TODO comment: pin nginx:alpine to fixed digest | Info | Not introduced by this phase; pre-existing; no impact on phase goal |

No debt markers (TBD, FIXME, XXX) introduced by this phase. The TODO in compose_test.go is pre-existing and unrelated to healthcheck config format work.

### Human Verification Required

None. All truths are verifiable programmatically.

---

## Verification Summary

Phase 15 goal is fully achieved. All 14 must-have truths are VERIFIED with direct codebase evidence:

- The `target.healthcheck` sub-block (interval, timeout, retries) replaces the old flat integer keys in the YAML schema; old keys are completely removed from all structs.
- `Resolve()` now takes a `globalFile FileConfig` parameter and implements four-tier precedence (flag > local deploy.yaml > global config > zero) for all three healthcheck fields.
- Duration strings are parsed via `time.ParseDuration`; negative values are explicitly rejected with source-naming error messages for all three tiers.
- No hardcoded defaults exist in config.go code (D-04 honoured).
- `main.go` loads `~/.docker/cli-plugins/deploy.yaml` via `loadGlobalConfig()` (missing file tolerated, malformed file fatal) and passes the result to all three `config.Resolve()` call sites (runValidate, runDryRun, runDeploy).
- `--healthcheck-timeout`, `--healthcheck-interval`, `--healthcheck-retries` flags are registered.
- `poll.go` consumes `cfg.Healthcheck.{Interval,Timeout,Retries}` as `time.Duration` directly; per-container `failCount` map implements the retries gate with reset-on-healthy.
- All config and health unit tests pass; integration tests are migrated; whole repo builds.

---

## Gap Closure Verification (Plan 15-03)

**Verified:** 2026-05-31
**Status:** PASSED
**Score:** 3/3 gap-closure must-haves verified

The two UAT gaps from Phase 15 are fully closed.

### Gap-Closure Must-Haves

| # | Must-Have | Status | Evidence |
|---|-----------|--------|---------|
| 1 | A typo in a YAML healthcheck key (e.g. 'retrise' instead of 'retries') returns a parse error, not silent zero | VERIFIED | `TestLoadFile_UnknownHealthcheckKey` at config_test.go:1397 writes `retrise: 3`, asserts non-nil error containing "retrise"; `go test ./internal/config/... -run TestLoadFile_UnknownHealthcheckKey` exits 0 PASS |
| 2 | An unknown top-level key in deploy.yaml returns a parse error | VERIFIED | `TestLoadFile_UnknownTopLevelKey` at config_test.go:1418 writes `boguskey: true`, asserts non-nil error containing "boguskey"; `go test ./internal/config/... -run TestLoadFile_UnknownTopLevelKey` exits 0 PASS |
| 3 | --dry-run output always includes a Healthcheck row showing resolved interval/timeout/retries or 'disabled' | VERIFIED | `formatHealthcheckRow` helper at main.go:230; `runDryRun()` calls `fmt.Fprintln(os.Stdout, formatHealthcheckRow(resolved.Healthcheck))` at line 331 unconditionally; `TestFormatHealthcheckRow` at main_test.go:412 tests disabled and non-zero cases; all 3 sub-tests PASS |

### Gap-Closure Artifact Verification

| Artifact | Required Evidence | Status |
|----------|-------------------|--------|
| `internal/config/config.go` LoadFile() | Uses `yaml.NewDecoder(bytes.NewReader(data))` with `.KnownFields(true)` | VERIFIED — lines 195-197 |
| `internal/config/config_test.go` | Contains `TestLoadFile_UnknownHealthcheckKey` and `TestLoadFile_UnknownTopLevelKey` | VERIFIED — lines 1397 and 1418 |
| `cmd/docker-deploy/main.go` | Contains `formatHealthcheckRow` function and `runDryRun()` calls it unconditionally | VERIFIED — function at line 230; call at line 331 |
| `cmd/docker-deploy/main_test.go` | Contains `TestFormatHealthcheckRow` with disabled and non-zero sub-cases | VERIFIED — line 412; 3 sub-tests all pass |

### Gap-Closure Key Links

| From | To | Via | Status |
|------|----|-----|--------|
| `LoadFile()` | `yaml.Decoder.KnownFields(true)` | `yaml.NewDecoder(bytes.NewReader(data))` | VERIFIED |
| `runDryRun()` | `resolved.Healthcheck` | `fmt.Fprintln(os.Stdout, formatHealthcheckRow(resolved.Healthcheck))` after Status line | VERIFIED |

### Gap-Closure Behavioral Spot-Checks

| Behavior | Command | Result | Status |
|----------|---------|--------|--------|
| Strict YAML: typo key rejected | `go test ./internal/config/... -run TestLoadFile_UnknownHealthcheckKey -v` | PASS | PASS |
| Strict YAML: unknown top-level key rejected | `go test ./internal/config/... -run TestLoadFile_UnknownTopLevelKey -v` | PASS | PASS |
| formatHealthcheckRow helper tests | `go test ./cmd/docker-deploy/... -run TestFormatHealthcheckRow -v` | 3 sub-tests PASS | PASS |
| Full build clean | `go build ./...` | exit 0, no output | PASS |

### Gap-Closure Summary

Both UAT gaps are closed with substantive implementation and passing tests:

1. **[major] YAML healthcheck field typos silently ignored** — closed by replacing `yaml.Unmarshal` with `yaml.NewDecoder` + `KnownFields(true)` in `LoadFile()`. Any unknown key in deploy.yaml now returns a parse error naming the offending field. Two new tests (`TestLoadFile_UnknownHealthcheckKey`, `TestLoadFile_UnknownTopLevelKey`) confirm the behaviour; regression test `TestLoadFile_ValidHealthcheckParsed` confirms valid configs still parse correctly.

2. **[minor] --dry-run output omits resolved healthcheck config** — closed by extracting `formatHealthcheckRow(hc config.HealthcheckConfig) string` helper and calling it unconditionally in `runDryRun()` after the `Status: OK` line. The helper returns `"  Healthcheck:  disabled"` for zero config and `"  Healthcheck:  interval=Xs timeout=Ys retries=N"` otherwise. Three sub-tests in `TestFormatHealthcheckRow` cover disabled, fully non-zero, and partial (interval only) cases.

---

_Verified: 2026-05-30_
_Gap closure verified: 2026-05-31_
_Verifier: Claude (gsd-verifier)_
