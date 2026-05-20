---
phase: 07-v2-skip-env-override
verified: 2026-05-20T00:00:00Z
status: human_needed
score: 5/6 must-haves verified
overrides_applied: 0
human_verification:
  - test: "Run docker deploy --verbose against a real SSH host and verify per-file transfer lines (  -> path), SSH command lines ([ssh] ...), exit-code lines (  → exit 0), and preflight checklist lines ([PASS]/[WARN]) all appear on stderr"
    expected: "All four output categories present in stderr; without --verbose none of these lines appear"
    why_human: "Cannot exercise the full Upload/RunCompose call chain without a live SSH daemon; TTY detection also affects which path runs in RunCompose (PTY vs non-TTY)"
  - test: "Run docker deploy --skip-env against a real SSH host and verify the warning appears and the remote .env is not modified"
    expected: "WARNING: .env not uploaded — remote .env left unchanged printed; remote .env content unchanged after deploy"
    why_human: "Requires a live SSH host to verify remote state; the warning routing (inline vs rollup) depends on the actual --verbose flag combination"
---

# Phase 7: v2 Skip-Env Override Verification Report

**Phase Goal:** Ship a wave of small v2 quality-of-life improvements: expand the built-in exclude list to cover common dev-tooling directories, add a `--skip-env` flag so operators can preserve remote secrets across deploys, and add a `--verbose` flag for detailed deploy output.
**Verified:** 2026-05-20
**Status:** human_needed
**Re-verification:** No — initial verification

## Goal Achievement

### Observable Truths

| # | Truth | Status | Evidence |
|---|-------|--------|---------|
| 1 | Passing `--skip-env` causes `.env` to be excluded from upload | VERIFIED | `mergeExcludes()` appends `.env` when `cfg.SkipEnv=true`; `config.go:197-203`; `TestResolveSkipEnv/flag_skip_env_appends_dot_env` passes |
| 2 | `skip_env: true` in deploy.yaml has same effect; CLI flag takes precedence | VERIFIED | `Resolve()` sets `cfg.SkipEnv = opts.SkipEnv \|\| file.Target.SkipEnv` (`config.go:263`); `TargetConfig.SkipEnv bool \`yaml:"skip_env"\`` present; `TestResolveSkipEnv/file_skip_env_appends_dot_env` and `flag_overrides_file_skip_env_false` pass |
| 3 | `--skip-env` is additive — appends `.env` without replacing other excludes | VERIFIED | `mergeExcludes()` uses a dedup-seen map; appends `.env` after all other entries; dedup tested in `TestResolveSkipEnv/skip_env_deduplicates_if_already_in_flag_excludes`; `TestResolveExcludes` confirms all 16 defaults remain present |
| 4 | When `.env` is skipped, a visible warning is printed | VERIFIED | `main.go:234-241`: when `resolved.SkipEnv`, prints `WARNING: .env not uploaded — remote .env left unchanged` inline (verbose) or accumulates for rollup (non-verbose); rollup fires at `main.go:333-335` |
| 5 | 10 new entries added to built-in default exclude list | VERIFIED | `defaultExcludes` in `config.go:18-22` contains all 16 entries: original 6 + `.claude/`, `.github/`, `.planning/`, `.idea/`, `.vscode/`, `*.swp`, `*.swo`, `coverage/`, `dist/`, `.terraform/`; `TestResolveExpandedDefaults` and `TestResolveExcludes` pass with `wantLen:16` |
| 6 | `--verbose` prints per-file transfers, SSH commands, exit codes; without it output is terse | UNCERTAIN | Code structure verified: `Upload()` routes per-file lines to stderr only when `verbose=true`; SSH commands logged via `sshExecVerbose()` and inline guards; `RunCompose()` logs `[ssh] <cmd>` before both TTY and non-TTY `session.Start()` and logs exit codes after `Wait()`; `go test ./...` passes; **end-to-end behavior requires live SSH host** |

**Score:** 5/6 truths verified (1 uncertain — requires human test)

### Required Artifacts

| Artifact | Expected | Status | Details |
|----------|----------|--------|---------|
| `internal/config/config.go` | FlagOpts struct, 16-entry defaultExcludes, SkipEnv/Verbose in Config+TargetConfig, updated Resolve() | VERIFIED | All present: `type FlagOpts struct` at line 68; `defaultExcludes` 16 entries; `Config.SkipEnv`, `Config.Verbose`, `TargetConfig.SkipEnv yaml:"skip_env"` |
| `internal/config/config_test.go` | TestResolveSkipEnv, TestResolveVerbose, TestResolveExpandedDefaults, updated FlagOpts call sites | VERIFIED | All three new tests present and passing; all existing tests use `FlagOpts{}` struct literals |
| `internal/filetransfer/upload.go` | Upload() verbose param, per-file stderr routing, warnedOnce conditioned on verbose, SSH command logging | VERIFIED | Signature `Upload(..., verbose bool)` at line 99; per-file guard `if verbose` at line 182; `sshExecVerbose()` helper at line 48; warnedOnce guard `if verbose \|\| !*warnedOnce` at line 243 |
| `internal/compose/run.go` | RunCompose() verbose param, SSH command + exit code logged when verbose | VERIFIED | Signature `RunCompose(..., verbose bool)` at line 51; `[ssh]` logging before `session.Start()` in both TTY (line 174) and non-TTY (line 141) paths; exit code logging after `Wait()` in both paths |
| `cmd/docker-deploy/main.go` | --skip-env and --verbose flags registered, FlagOpts wiring, warning rollup, verbose preflight rendering | VERIFIED | Flags registered at lines 73-74; `FlagOpts{SkipEnv: skipEnv, Verbose: verbose}` at both Resolve call sites; `var warnings []string` at line 229; rollup at line 333; preflight rendering at lines 251-262 |

### Key Link Verification

| From | To | Via | Status | Details |
|------|----|-----|--------|---------|
| `cmd/docker-deploy/main.go` | `config.FlagOpts{}` | Both `runDeploy()` and `runDryRun()` pass `SkipEnv: skipEnv, Verbose: verbose` | WIRED | `grep -c "FlagOpts{"` = 2 |
| `internal/filetransfer/upload.go` | `cfg.Verbose` | `verbose bool` param propagated from `runDeploy()` via `filetransfer.Upload(..., resolved.Verbose)` | WIRED | `main.go:312` passes `resolved.Verbose` |
| `cmd/docker-deploy/main.go` | `preflight.RunPreflightChecks()` | `results []preflight.CheckResult` rendered to stderr when `cfg.Verbose`; warn-status collected for rollup | WIRED | `main.go:244,251-262` |
| `cmd/docker-deploy/main.go` | `compose.RunCompose()` | `cfg.Verbose` passed as last argument | WIRED | `main.go:321` passes `resolved.Verbose` |

### Data-Flow Trace (Level 4)

| Artifact | Data Variable | Source | Produces Real Data | Status |
|----------|---------------|--------|--------------------|--------|
| `config.go` Resolve() | `cfg.SkipEnv` | `opts.SkipEnv \|\| file.Target.SkipEnv` | Yes — reads from FlagOpts and FileConfig | FLOWING |
| `upload.go` Upload() | `verbose` | Passed from `resolved.Verbose` in `main.go` | Yes — live config value | FLOWING |
| `run.go` RunCompose() | `verbose` | Passed from `resolved.Verbose` in `main.go` | Yes — live config value | FLOWING |
| `main.go` warnings | `[]string` accumulator | Populated on `resolved.SkipEnv` and preflight `warn` results | Yes — reflects runtime state | FLOWING |

### Behavioral Spot-Checks

| Behavior | Command | Result | Status |
|----------|---------|--------|--------|
| `go test ./internal/config/... -run TestResolveSkipEnv` | `go test ./internal/config/... -run TestResolveSkipEnv` | 5 subtests, all PASS | PASS |
| `go test ./internal/config/... -run TestResolveExpandedDefaults` | `go test ./internal/config/... -run TestResolveExpandedDefaults` | PASS | PASS |
| `go test ./internal/config/... -run TestResolveVerbose` | `go test ./internal/config/... -run TestResolveVerbose` | 2 subtests, all PASS | PASS |
| `go build ./...` | `go build ./...` | No output — success | PASS |
| `go test ./...` | `go test ./...` | All 6 packages PASS | PASS |
| All 10 new excludes present in config.go | `python3` membership check | All 10 PRESENT | PASS |
| --skip-env flag registered | `main_test.go` TestSkipEnvFlagRegistered | PASS | PASS |
| --verbose flag registered | `main_test.go` TestVerboseFlagRegistered | PASS | PASS |

### Probe Execution

Step 7c: No `probe-*.sh` files declared or found for this phase.

### Requirements Coverage

| Requirement | Source Plan | Description | Status | Evidence |
|-------------|------------|-------------|--------|----------|
| FILES-02 | 07-01, 07-02 | Default exclude list: .git/, node_modules/, vendor/, *.log, .DS_Store, \_\_pycache\_\_/ | SATISFIED | `defaultExcludes` retains all 6 original entries and adds 10 more; `TestResolveExcludes` passes with `wantLen:16` |

Note: The traceability table in REQUIREMENTS.md also maps `CFG-06` to Phase 7 (`--config <path>` flag for custom config file path). This requirement is NOT implemented in this phase — Phase 7 scope is limited to skip-env, verbose, and expanded excludes per the ROADMAP. CFG-06 is an orphaned mapping in REQUIREMENTS.md that was not addressed by either plan; it is a v2 requirement marked "Pending" and is not part of the phase goal.

### Anti-Patterns Found

| File | Line | Pattern | Severity | Impact |
|------|------|---------|----------|--------|
| — | — | None found | — | No debt markers (TBD/FIXME/XXX) in any phase-modified file |

### Human Verification Required

#### 1. Verbose flag end-to-end output verification

**Test:** Run `docker deploy --host ssh://user@yourhost:22 --verbose 2>/tmp/verbose.txt` against a real SSH host, then inspect `/tmp/verbose.txt`.
**Expected:** Output contains ALL of:
- Lines starting with `  ->` (per-file transfer lines from upload.go)
- Lines starting with `[ssh] ` (SSH command lines: mkdir, mv, rm, docker compose up -d)
- Lines starting with `  → exit ` (exit codes after each SSH command)
- Lines starting with `  [PASS]` or `  [WARN]` (preflight checklist from main.go)

Then run the same deploy WITHOUT `--verbose` and verify none of those lines appear; only a possible `WARN: there are some warnings...` rollup.
**Why human:** Cannot exercise the Upload + RunCompose full call chain without a live SSH daemon. TTY detection in `RunCompose()` also governs which execution path runs (PTY vs non-TTY pipe drain), and the PTY path is only exercisable in an interactive terminal.

#### 2. Skip-env flag with remote state verification

**Test:** On a remote host that has an existing `.env` file, run `docker deploy --host ssh://user@host:22 --skip-env`. After deploy, SSH into the remote and verify the `.env` content was not changed.
**Expected:** The deploy completes; the `.env` file on the remote is untouched; the warning `WARNING: .env not uploaded — remote .env left unchanged` appears in stderr output (inline if `--verbose`, otherwise as part of the `WARN: there are some warnings...` rollup line).
**Why human:** Verifying that the remote file was not modified requires actual SSH access to a remote host with a pre-existing `.env` file. The warning routing (inline vs rollup) also involves the interaction of --skip-env and --verbose flags in a live deploy context.

### Gaps Summary

No blocking gaps. All six observable truths are either VERIFIED (5) or UNCERTAIN (1 — SC-6 verbose output verified at code level, pending human end-to-end test). The code structure for SC-6 is fully implemented and tested with unit tests; the uncertainty is behavioral (requires a live SSH host to exercise the full call path).

---

_Verified: 2026-05-20_
_Verifier: Claude (gsd-verifier)_
