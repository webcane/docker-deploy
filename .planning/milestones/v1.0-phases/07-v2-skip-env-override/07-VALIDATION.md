---
phase: 07
slug: 07-v2-skip-env-override
status: verified
nyquist_compliant: true
wave_0_complete: false
created: 2026-05-23
---

# Phase 07 — Validation Strategy

> Per-phase validation contract for feedback sampling during execution.

---

## Test Infrastructure

| Property | Value |
|----------|-------|
| **Framework** | go test |
| **Config file** | none — built-in Go test runner |
| **Quick run command** | `go test ./internal/config/... ./cmd/docker-deploy/... -count=1` |
| **Full suite command** | `go test ./... -count=1` |
| **Estimated runtime** | ~15 seconds (unit), ~60 seconds (full with integration) |

---

## Sampling Rate

- **After every task commit:** Run `go test ./internal/config/... ./cmd/docker-deploy/... -count=1`
- **After every plan wave:** Run `go test ./... -count=1`
- **Before `/gsd-verify-work`:** Full suite must be green
- **Max feedback latency:** ~15 seconds

---

## Per-Task Verification Map

| Task ID | Plan | Wave | Requirement | Threat Ref | Secure Behavior | Test Type | Automated Command | File Exists | Status |
|---------|------|------|-------------|------------|-----------------|-----------|-------------------|-------------|--------|
| 07-01-01 | 01 | 1 | FILES-02 | T-07-01-01 | SkipEnv=true appends .env to excludes; .env appears exactly once via dedup | unit | `go test ./internal/config/... -run TestResolveSkipEnv -v` | ✅ | ✅ green |
| 07-01-02 | 01 | 1 | FILES-02 | T-07-01-02 | defaultExcludes has all 10 new dev-tooling entries; no prod artifact inadvertently excluded | unit | `go test ./internal/config/... -run TestResolveExpandedDefaults -v` | ✅ | ✅ green |
| 07-01-03 | 01 | 1 | FILES-02 | T-07-01-03 | Verbose config field set correctly via FlagOpts | unit | `go test ./internal/config/... -run TestResolveVerbose -v` | ✅ | ✅ green |
| 07-01-04 | 01 | 1 | FILES-02 | — | Resolve() accepts FlagOpts struct; all prior tests pass with new signature | unit | `go test ./internal/config/... -count=1` | ✅ | ✅ green |
| 07-02-01 | 02 | 2 | FILES-02 | — | Upload() verbose param: per-file lines to stderr only when verbose=true | unit | `go test ./internal/filetransfer/... -run TestUploadVerbose_PerFileStderr -v` | ✅ | ✅ green |
| 07-02-02 | 02 | 2 | FILES-02 | T-07-02-05 | Upload() verbose param: SSH commands logged with exit codes when verbose=true | unit | `go test ./internal/filetransfer/... -run TestUploadVerbose_SSHCommandLogging -v` | ✅ | ✅ green |
| 07-02-03 | 02 | 2 | FILES-02 | — | RunCompose() verbose param: compose SSH command + exit codes logged when verbose=true | unit | `go test ./internal/compose/... -run TestRunCompose_Verbose -v` | ✅ | ✅ green |
| 07-02-04 | 02 | 2 | FILES-02 | — | RunCompose() verbose=false: no SSH logging | unit | `go test ./internal/compose/... -run TestRunCompose_VerboseNoSSHLoggingWhenFalse -v` | ✅ | ✅ green |
| 07-02-05 | 02 | 2 | FILES-02 | T-07-02-01 | --skip-env flag registered as bool on deploy command | unit | `go test ./cmd/docker-deploy/... -run TestSkipEnvFlagRegistered -v` | ✅ | ✅ green |
| 07-02-06 | 02 | 2 | FILES-02 | — | --verbose flag registered as bool on deploy command | unit | `go test ./cmd/docker-deploy/... -run TestVerboseFlagRegistered -v` | ✅ | ✅ green |
| 07-02-07 | 02 | 2 | FILES-02 | T-07-02-02 | rollupMsg(verbose=true) omits hint; rollupMsg(verbose=false) includes --verbose hint | unit | `go test ./cmd/docker-deploy/... -run TestRollupMsg -v` | ✅ | ✅ green |
| 07-02-08 | 02 | 2 | FILES-02 | — | formatCheckResult produces correct `  [STATUS] name: message` format for pass/warn/fail | unit | `go test ./cmd/docker-deploy/... -run TestFormatCheckResult -v` | ✅ | ✅ green |
| 07-02-09 | 02 | 2 | FILES-02 | T-07-02-01 | skip-env preserves remote .env (end-to-end): remote .env unchanged after deploy with --skip-env | integration | `go test ./integration/... -run TestUpload_SkipEnv -v` | ✅ | ✅ green |

---

## Wave 0 Requirements

Existing infrastructure covers all phase requirements. Go test runner was already configured; no new packages or fixtures needed.

---

## Manual-Only Verifications

All phase behaviors have automated verification.

---

## Validation Sign-Off

- [x] All tasks have `<automated>` verify or Wave 0 dependencies
- [x] Sampling continuity: no 3 consecutive tasks without automated verify
- [x] Wave 0 covers all MISSING references
- [x] No watch-mode flags
- [x] Feedback latency < 15s
- [x] `nyquist_compliant: true` set in frontmatter

**Approval:** approved 2026-05-23
