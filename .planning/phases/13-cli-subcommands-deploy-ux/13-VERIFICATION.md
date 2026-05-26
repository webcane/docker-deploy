---
phase: 13-cli-subcommands-deploy-ux
verified: 2026-05-26T15:30:00Z
status: passed
score: 8/8 must-haves verified
overrides_applied: 0
re_verification:
  previous_status: human_needed
  previous_score: 7/8
  gaps_closed:
    - "SC-3: dev/untagged builds omit Built: line — version != 'dev' gate added and tested (13-08)"
    - "SC-5/UAT-5: verbose mode no longer double-lists files — per-file arrows replaced with summary line (13-09)"
    - "SC-8/UAT-6: needsSudo probe now checks parent-only so user-owned target with root-owned parent correctly triggers sudo (13-10)"
  gaps_remaining: []
  regressions: []
---

# Phase 13: CLI Subcommands & Deploy UX Verification Report

**Phase Goal:** Fix six self-contained Go issues: resolve `deploy.yaml` relative to cwd, add `docker deploy version` and `docker deploy validate` subcommands, consolidate remote sudo calls into a single SSH session, add a verbose pre-confirm file diff, and add path-aware sudo detection to skip guaranteed-to-fail direct copy attempts on elevated paths.
**Verified:** 2026-05-26T15:30:00Z
**Status:** passed
**Re-verification:** Yes — after gap closure (plans 13-08, 13-09, 13-10)

## Goal Achievement

### Observable Truths

| #  | Truth | Status | Evidence |
|----|-------|--------|----------|
| 1  | `deploy.yaml` is resolved relative to cwd; no hardcoded absolute paths | VERIFIED | `config.LoadFile(cwd)` calls `filepath.Join(dir, "deploy.yaml")` with no internal `os.Getwd()` call; `TestLoadFile_CwdRelative` present in config_test.go |
| 2  | `docker deploy version` is a standalone subcommand that prints version and exits 0 | VERIFIED | `buildVersionCmd()` at line 88 of main.go; `cmd.AddCommand(buildVersionCmd())` at line 80; 5 TestVersionCmd_* tests all pass |
| 3  | When built from tagged commit, version string is semver tag; untagged (dev) builds omit Built: line and show commit hash on separate Git commit: line | VERIFIED | `version != "dev" && buildTime != "unknown"` gate at line 109 of main.go; `TestVersionCmd_DevBuildWithInjectedTime` passes: version="dev" + injected buildTime → no Built: line; `TestVersionCmd_TaggedOutput` passes: version="v0.6.3" → Built: line present |
| 4  | Version values injected at build time via ldflags; no runtime git invocation | VERIFIED | `var gitCommit = "unknown"` and `var buildTime = "unknown"` at lines 29-30 of main.go; Makefile and `.goreleaser.yaml` both inject via `-X main.gitCommit` and `-X main.buildTime`; no `exec.Command("git"...)` in source |
| 5  | `docker deploy validate` exits 0 with "✓ deploy.yaml is valid" on good config; exits non-zero on bad config; no SSH connection made | VERIFIED | `buildValidateCmd()` and `runValidate()` at lines 125-175 of main.go; 5 TestValidateCmd_* tests all pass |
| 6  | A deploy to a sudo-required path issues exactly one sudo prompt regardless of file count; user-writable paths unaffected | VERIFIED | `SudoExec()` at line 105 of upload.go; `SudoCreds` with `Zero()` at lines 28-37; `TestSudoExec_SinglePromptMultiFile` confirms single prompt across 8 operations; `creds := new(filetransfer.SudoCreds)` in main.go |
| 7  | In `--verbose` mode, remote and local files listed before "Replace all contents?" prompt; each local filename appears exactly once | VERIFIED | Verbose diff block at lines 383-412 of upload.go; per-file `-> filename` arrows removed; single `"Uploading %d files...\n"` summary to stderr before loop (line 296); `TestUploadVerbose_SummaryLine` both subtests pass: verbose_true asserts "Uploading 1 files..." present and no "  -> " arrows |
| 8  | Path-aware sudo detection: probe checks ONLY the parent directory; user-owned target with root-owned parent correctly triggers sudo | VERIFIED | `probeCmd := fmt.Sprintf("test -w %s", ShellQuote(path.Dir(remoteBase)))` at line 245 of upload.go — single operand, no OR clause; `TestUpload_PathAwareSudo_OwnsTargetButParentElevated` passes: user-owned /opt/test-deploy, root-owned /opt → needsSudo=true; `TestUpload_PathAwareSudo_ParentWritable` updated to assert no "||" in probe |

**Score:** 8/8 truths verified

### Required Artifacts

| Artifact | Expected | Status | Details |
|----------|----------|--------|---------|
| `cmd/docker-deploy/main.go` | `version != "dev"` gate in `runVersionTo()`; `buildVersionCmd()`; `buildValidateCmd()`; `SudoCreds` call site | VERIFIED | Line 109: `if version != "dev" && buildTime != "unknown"`; all other artifacts confirmed at lines 28-30, 88, 125, 400 |
| `cmd/docker-deploy/main_test.go` | `TestVersionCmd_DevBuildWithInjectedTime` new regression test | VERIFIED | Present at line 200; passes |
| `internal/filetransfer/upload.go` | `probeCmd` uses only `path.Dir(remoteBase)` — no OR clause; per-file arrow lines removed; `"Uploading %d files...\n"` summary before loop | VERIFIED | Line 245: single `fmt.Sprintf("test -w %s", ShellQuote(path.Dir(remoteBase)))`; no `-> %s` inside upload loop; line 296: summary to stderr gated on `if verbose` |
| `internal/filetransfer/upload_test.go` | `TestUploadVerbose_SummaryLine` (renamed from PerFileStderr); `TestUpload_PathAwareSudo_OwnsTargetButParentElevated` new regression test; `TestUpload_PathAwareSudo_ParentWritable` updated | VERIFIED | All four PathAwareSudo tests pass; SummaryLine subtests pass; OR-probe assertion replaced with no-OR assertion |

### Key Link Verification

| From | To | Via | Status | Details |
|------|----|-----|--------|---------|
| `runVersionTo()` | `version != "dev"` gate | `if version != "dev" && buildTime != "unknown"` at line 109 | WIRED | Confirmed; resolves UAT gap — dev builds with injected buildTime omit Built: |
| `upload.go probeCmd` | `path.Dir(remoteBase)` only | `fmt.Sprintf("test -w %s", ShellQuote(path.Dir(remoteBase)))` at line 245 | WIRED | Single operand; `grep -c '"test -w' upload.go` = 2 (one functional at line 245, one in comment at line 240) |
| `upload.go step 6` | stderr summary | `if verbose { fmt.Fprintf(os.Stderr, "Uploading %d files...\n", len(files)) }` at line 295-297, before loop closure | WIRED | Per-file arrows absent; `grep -n '"  -> %s' upload.go` = 0 results |
| All previously verified links (SC-1 through SC-8 from initial verification) | — | — | WIRED | No regressions detected; full `go test ./... -count=1` exits 0 |

### Data-Flow Trace (Level 4)

Not applicable: phase produces no data-rendering components. All changes are CLI commands, configuration resolution, and SSH primitives.

### Behavioral Spot-Checks

| Behavior | Command | Result | Status |
|----------|---------|--------|--------|
| Full test suite passes (no cache) | `go test ./... -count=1` | All 6 packages ok | PASS |
| All TestVersionCmd_* pass including new regression | `go test ./cmd/docker-deploy/... -run TestVersionCmd -v` | 5/5 PASS | PASS |
| `TestVersionCmd_DevBuildWithInjectedTime` passes | Included above | PASS | PASS |
| All TestUploadVerbose_* pass including SummaryLine | `go test ./internal/filetransfer/... -run TestUploadVerbose -v` | included in full run | PASS |
| All 4 TestUpload_PathAwareSudo_* pass | `go test ./internal/filetransfer/... -run TestUpload_PathAwareSudo -v` | 4/4 PASS (WritablePath, ElevatedPath, ParentWritable, OwnsTargetButParentElevated) | PASS |
| probeCmd is single operand (no OR) | `grep -c '"test -w' upload.go` | 2 (1 functional line + 1 comment) | PASS |
| Per-file arrow lines removed from upload loop | `grep -n '"  -> %s' upload.go` | 0 results | PASS |
| `version != "dev"` gate in runVersionTo | `grep -n 'version != "dev"' cmd/docker-deploy/main.go` | 1 result at line 109 | PASS |

### Probe Execution

Step 7c: SKIPPED — no `scripts/*/tests/probe-*.sh` files found in the repository.

### Requirements Coverage

Phase 13 uses internal planning IDs (13-01 through 13-10). All six original goals and three gap-closure fixes are verified:

| Goal | Status | Evidence |
|------|--------|----------|
| deploy.yaml cwd resolution | SATISFIED | `LoadFile(cwd)` uses `filepath.Join(cwd, "deploy.yaml")` |
| `docker deploy version` subcommand | SATISFIED | `buildVersionCmd()` registered; UAT gap closed: dev builds no longer show Built: |
| `docker deploy validate` subcommand | SATISFIED | `buildValidateCmd()` registered; local-only validation confirmed |
| Consolidated sudo via SudoCreds/SudoExec/sshRun | SATISFIED | Single prompt per deploy confirmed by test |
| Verbose pre-confirm diff | SATISFIED | ReadDir + local file list before prompt; UAT gap closed: files no longer listed twice |
| Path-aware sudo detection | SATISFIED | UAT gap closed: parent-only probe correctly triggers sudo when user owns target but parent is root-owned |
| Verbose `sudo -l` in preflight | SATISFIED | `if cfg.Verbose` block with `runOutput(client, "sudo -l")` in `checkDockerGroup()` |

### Anti-Patterns Found

| File | Line | Pattern | Severity | Impact |
|------|------|---------|----------|--------|
| None found | — | — | — | — |

Scanned files modified by plans 13-08, 13-09, 13-10 (`cmd/docker-deploy/main.go`, `cmd/docker-deploy/main_test.go`, `internal/filetransfer/upload.go`, `internal/filetransfer/upload_test.go`) for `TBD`, `FIXME`, `XXX`, placeholder patterns, stub returns. No actionable anti-patterns found.

### Human Verification Required

None. All previous human verification items are resolved:

- SC-3 (version line for untagged builds): Now fully automated — `TestVersionCmd_DevBuildWithInjectedTime` proves the `version != "dev"` gate works. The ambiguity is eliminated: dev builds always produce 3-line output (no Built:) regardless of whether buildTime was injected by `make build`.

### Gaps Summary

No gaps. All 8 observable truths are verified in the codebase. The three UAT gaps reported after initial verification have been closed by plans 13-08, 13-09, and 13-10, each with targeted tests confirming the fixes. Full test suite passes with `go test ./... -count=1`.

---

_Verified: 2026-05-26T15:30:00Z_
_Verifier: Claude (gsd-verifier)_
