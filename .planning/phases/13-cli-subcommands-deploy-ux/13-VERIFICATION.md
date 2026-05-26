---
phase: 13-cli-subcommands-deploy-ux
verified: 2026-05-26T14:00:00Z
status: human_needed
score: 7/8 must-haves verified
overrides_applied: 0
human_verification:
  - test: "Run 'docker deploy version' from a non-tagged local build. Confirm the output line reads 'Docker Deploy Version dev' with a 'Git commit:' line below it. Then run 'make build && ./bin/docker-deploy version' to confirm git commit hash appears."
    expected: "Version line shows 'dev' (or injected tag), Git commit line shows short hash from git rev-parse. Untagged/dev builds show 3 lines; tagged builds show 4 lines with Built: timestamp."
    why_human: "SC-3 says 'untagged builds print the short git commit hash' — the implementation prints 'dev' on the version line and the hash on a separate 'Git commit:' line. This satisfies the spirit but the SC wording is ambiguous. Human confirmation needed that the intent is satisfied by the 2-field approach."
---

# Phase 13: CLI Subcommands & Deploy UX Verification Report

**Phase Goal:** Fix six self-contained Go issues: resolve `deploy.yaml` relative to cwd, add `docker deploy version` and `docker deploy validate` subcommands, consolidate remote sudo calls into a single SSH session, add a verbose pre-confirm file diff, and add path-aware sudo detection to skip guaranteed-to-fail direct copy attempts on elevated paths.
**Verified:** 2026-05-26T14:00:00Z
**Status:** human_needed
**Re-verification:** No — initial verification

## Goal Achievement

### Observable Truths

| #  | Truth | Status | Evidence |
|----|-------|--------|----------|
| 1  | `deploy.yaml` is resolved relative to cwd; no hardcoded absolute paths | ✓ VERIFIED | `config.LoadFile(cwd)` calls `filepath.Join(dir, "deploy.yaml")` with no internal `os.Getwd()` call; `grep -n "os\.Getwd" internal/config/config.go` returns no results; `TestLoadFile_CwdRelative` present at line 768 of config_test.go |
| 2  | `docker deploy version` is a standalone subcommand that prints version and exits 0 | ✓ VERIFIED | `buildVersionCmd()` defined at line 88 of main.go; `cmd.AddCommand(buildVersionCmd())` at line 80; `TestVersionCmd_Registered`, `TestVersionCmd_ExitZero` in main_test.go; `runVersionTo(w io.Writer)` returns nil |
| 3  | When built from tagged commit, version string is semver tag; untagged shows commit hash | ? UNCERTAIN | The version line prints `version` var (injected as `{{.Version}}` by GoReleaser for tags, or "dev" for dev builds). The git commit hash is printed on a *separate* line. SC-3 says "untagged builds print the short git commit hash" — the hash IS printed but on the `Git commit:` line, not the `Docker Deploy Version` line. Spirit satisfied but wording ambiguous; human confirmation needed. |
| 4  | Version values injected at build time via ldflags; no runtime git invocation | ✓ VERIFIED | `var gitCommit = "unknown"` and `var buildTime = "unknown"` at lines 29-30 of main.go; Makefile injects `-X main.gitCommit=$(shell git rev-parse --short HEAD)` and `-X main.buildTime=...`; `.goreleaser.yaml` has `-X main.gitCommit={{.ShortCommit}}` and `-X main.buildTime={{.Date}}`; no `exec.Command("git"...)` anywhere in source |
| 5  | `docker deploy validate` exits 0 with "✓ deploy.yaml is valid" on good config; exits non-zero on bad config; no SSH connection made | ✓ VERIFIED | `buildValidateCmd()` and `runValidate()` at lines 125-175 of main.go; uses `os.Stat` to detect missing file, `config.LoadFile`+`config.Resolve` for validation; no `sshpkg.Dial` call in `runValidate()`. `TestValidateCmd_Registered`, `TestValidateCmd_ValidConfig`, `TestValidateCmd_MissingFile`, `TestValidateCmd_InvalidYAML`, `TestValidateCmd_NoSSH` all present in main_test.go |
| 6  | A deploy to a sudo-required path issues exactly one sudo prompt regardless of file count; user-writable paths unaffected | ✓ VERIFIED | `SudoExec()` exported at line 105 of upload.go; `SudoCreds` with `Zero()` at lines 28-37; `TestSudoExec_SinglePromptMultiFile` at line 1047 of upload_test.go confirms single prompt across 8 operations; `creds := new(filetransfer.SudoCreds); defer creds.Zero()` in main.go at line 400-401 |
| 7  | In `--verbose` mode, remote and local files listed before "Replace all contents?" prompt | ✓ VERIFIED | Verbose diff block at lines 383-412 of upload.go; `sftpClient.ReadDir(remoteBase)` at line 399; "Local files" printed at line 389; first-deploy "Remote files: (none)" at line 445; `TestUploadVerbose_PreConfirmDiff`, `TestUploadVerbose_FirstDeploy_NoRemote` in upload_test.go; "Replace all contents" absent from main.go (prompt moved into Upload()) |
| 8  | Path-aware sudo detection: `test -w` probe skips SudoExec for user-writable paths; elevated paths use full fallback | ✓ VERIFIED | `needsSudo` probe at lines 243-255 of upload.go; `test -w remoteBase \|\| test -w path.Dir(remoteBase)` pattern; `execCmd` closure at lines 259-276 dispatches to `SudoExec` or `sshRun(nil)` based on probe result; `path.Dir` (not `filepath.Dir`) used; `TestUpload_PathAwareSudo_WritablePath`, `TestUpload_PathAwareSudo_ElevatedPath`, `TestUpload_PathAwareSudo_ParentWritable` in upload_test.go |

**Score:** 7/8 truths verified (1 uncertain — SC-3 wording ambiguity)

### Required Artifacts

| Artifact | Expected | Status | Details |
|----------|----------|--------|---------|
| `internal/config/config.go` | `LoadFile(cwd string)` using `filepath.Join(cwd, "deploy.yaml")`; no internal `os.Getwd()` | ✓ VERIFIED | Function exists; `grep os.Getwd config.go` returns 0 results |
| `internal/config/config_test.go` | `TestLoadFile_CwdRelative` confirming cwd-relative path | ✓ VERIFIED | Present at line 768 |
| `cmd/docker-deploy/main.go` | `var gitCommit`, `var buildTime`; `buildVersionCmd()`; `buildValidateCmd()`; `SudoCreds` call site | ✓ VERIFIED | All present at lines 28-30, 88, 125, 400 |
| `cmd/docker-deploy/main_test.go` | `TestVersionCmd_*` (4 tests); `TestValidateCmd_*` (5 tests) | ✓ VERIFIED | All 9 tests present; lines 128-334 |
| `Makefile` | Extended build target with `gitCommit` and `buildTime` ldflags | ✓ VERIFIED | Lines 5-8: `-X main.gitCommit=$(shell git rev-parse --short HEAD ...)` and `-X main.buildTime=$(shell date ...)` |
| `.goreleaser.yaml` | `ldflags` block with `-X main.gitCommit={{.ShortCommit}}` and `-X main.buildTime={{.Date}}` | ✓ VERIFIED | Lines 10-11: both entries present |
| `internal/filetransfer/upload.go` | `SudoExec`, `SudoCreds`, `sshRun`, `needsSudo` probe, verbose diff block; old functions deleted | ✓ VERIFIED | `SudoExec` at line 105; `SudoCreds` at line 28; `sshRun` at line 61; `needsSudo` probe at line 248; verbose diff at lines 383-446; `sudoRunWithFallback`, `sshExec`, `sshExecWithSudoPassword`, `sudoPw` all absent (grep returns 0 results) |
| `internal/filetransfer/upload_test.go` | `TestSudoExec_*`, `TestSudoCreds_Zero`, `TestSudoExec_SinglePromptMultiFile`, `TestUploadVerbose_*`, `TestUpload_PathAwareSudo_*` | ✓ VERIFIED | All expected tests present at lines 731-1282 |
| `internal/preflight/checks.go` | `if cfg.Verbose` block with `runOutput(client, "sudo -l")` inside `checkDockerGroup()` | ✓ VERIFIED | Lines 279-284; `[sudo -l]` prefix present; no else branch on failure |
| `internal/preflight/checks_test.go` | `TestCheckDockerGroup_SudoL_VerboseShown`, `TestCheckDockerGroup_SudoL_FailureSilenced`, `TestCheckDockerGroup_SudoL_NotVerbose` | ✓ VERIFIED | All 3 tests present at lines 247-324 |

### Key Link Verification

| From | To | Via | Status | Details |
|------|----|-----|--------|---------|
| `buildDeployCmd()` | `buildVersionCmd()` | `cmd.AddCommand(buildVersionCmd())` at line 80 | ✓ WIRED | Confirmed present |
| `buildDeployCmd()` | `buildValidateCmd()` | `cmd.AddCommand(buildValidateCmd())` at line 81 | ✓ WIRED | Confirmed present |
| `.goreleaser.yaml ldflags` | `var gitCommit, var buildTime` in main.go | `-X main.gitCommit={{.ShortCommit}} -X main.buildTime={{.Date}}` | ✓ WIRED | Both vars and ldflags confirmed |
| `runValidate()` | `config.LoadFile + config.Resolve` | `config.LoadFile(cwd)` then `config.Resolve(config.FlagOpts{}, ...)` at lines 159-170 | ✓ WIRED | Full sequence present |
| `Upload()` | `SudoExec()` | `execCmd` closure dispatches via `needsSudo` probe result | ✓ WIRED | Lines 259-276 of upload.go |
| `main.go runDeploy()` | `filetransfer.SudoCreds` | `creds := new(filetransfer.SudoCreds); defer creds.Zero()` | ✓ WIRED | Lines 400-401 of main.go |
| `Upload()` verbose diff | `sftpClient.ReadDir(remoteBase)` | Called at line 399 only when `verbose && !force && existsBefore` | ✓ WIRED | Confirmed |
| `checkDockerGroup()` | `runOutput(client, "sudo -l")` | `if cfg.Verbose` block at line 279 | ✓ WIRED | Confirmed |

### Data-Flow Trace (Level 4)

Not applicable: phase produces no data-rendering components. All changes are CLI commands, configuration resolution, and SSH primitives — no data displayed from external queries.

### Behavioral Spot-Checks

| Behavior | Command | Result | Status |
|----------|---------|--------|--------|
| `go build ./...` compiles cleanly | `go build ./...` | exit 0 | ✓ PASS |
| Full test suite passes | `go test ./...` | exit 0, 6 packages pass | ✓ PASS |
| `sudoRunWithFallback` removed | `grep -c "sudoRunWithFallback" internal/filetransfer/upload.go` | 0 | ✓ PASS |
| Old sshExec functions removed | `grep -c "func sshExec\b" internal/filetransfer/upload.go` | 0 | ✓ PASS |
| `SudoExec` exported | `grep -c "func SudoExec" internal/filetransfer/upload.go` | 1 | ✓ PASS |
| `sshRun` unified primitive | `grep -c "func sshRun" internal/filetransfer/upload.go` | 1 | ✓ PASS |
| `sudoPw` removed from main.go | `grep -c "sudoPw" cmd/docker-deploy/main.go` | 0 | ✓ PASS |
| `SudoCreds` used in main.go | `grep -c "SudoCreds" cmd/docker-deploy/main.go` | 2 | ✓ PASS |
| Confirm prompt removed from main.go | `grep -c "Replace all contents" cmd/docker-deploy/main.go` | 0 | ✓ PASS |
| `sftpClient.ReadDir` in upload.go | `grep -c "sftpClient\.ReadDir" internal/filetransfer/upload.go` | 1 | ✓ PASS |
| `needsSudo` probe present | `grep -c "needsSudo" internal/filetransfer/upload.go` | 6 | ✓ PASS |
| `test -w` probe command present | `grep -c "test -w" internal/filetransfer/upload.go` | 2 | ✓ PASS |
| `path.Dir` used (not `filepath.Dir`) | `grep -c "filepath\.Dir" internal/filetransfer/upload.go` | 1 (comment only, no functional use) | ✓ PASS |
| `[sudo -l]` in checks.go | `grep -c "\[sudo -l\]" internal/preflight/checks.go` | 1 | ✓ PASS |
| No `os.Getwd` inside `LoadFile` | `grep -n "os\.Getwd" internal/config/config.go` | 0 results | ✓ PASS |

### Probe Execution

Step 7c: SKIPPED — no `scripts/*/tests/probe-*.sh` files found in the repository.

### Requirements Coverage

The requirement IDs 13-01 through 13-07 are phase-internal planning IDs — they do not correspond to entries in `.planning/REQUIREMENTS.md` (which uses PLUG-*, DEPLOY-*, CFG-*, CHECK-*, HEALTH-* namespaces). Phase 13's ROADMAP entry lists `Requirements: TBD` and the plans use internal 13-XX IDs as work-breakdown references, not traceability IDs.

Cross-checking against REQUIREMENTS.md v1 entries that Phase 13 substantively advances:

| Internal Req | REQUIREMENTS.md Analogue | Description | Status | Evidence |
|-------------|--------------------------|-------------|--------|----------|
| 13-01 | (internal, no v1 analogue) | deploy.yaml cwd resolution | ✓ SATISFIED | `LoadFile(cwd)` uses `filepath.Join(cwd, "deploy.yaml")` with no internal `os.Getwd()` |
| 13-02 | (internal, no v1 analogue) | `docker deploy version` subcommand | ✓ SATISFIED | `buildVersionCmd()` registered; ldflags injected in Makefile + GoReleaser |
| 13-03 | (internal, no v1 analogue) | `docker deploy validate` subcommand | ✓ SATISFIED | `buildValidateCmd()` registered; local-only validation confirmed |
| 13-04 | DEPLOY-07 (partial advance) | Consolidated sudo: `SudoExec`, `SudoCreds`, `sshRun` | ✓ SATISFIED | `SudoExec` exported; fallback chain implemented; single prompt per deploy (SC-6) |
| 13-05 | OBS-03 (v2, now implemented) | Verbose pre-confirm diff | ✓ SATISFIED | `sftpClient.ReadDir` + local file list before prompt in upload.go |
| 13-06 | DEPLOY-07 (partial advance) | Path-aware sudo detection | ✓ SATISFIED | `test -w` probe at upload start; `execCmd` closure dispatches correctly |
| 13-07 | (internal, no v1 analogue) | Verbose `sudo -l` in CHECK-04 | ✓ SATISFIED | `if cfg.Verbose` block with `runOutput(client, "sudo -l")` in `checkDockerGroup()` |

### Anti-Patterns Found

| File | Line | Pattern | Severity | Impact |
|------|------|---------|----------|--------|
| None found | — | — | — | — |

Scanned all files modified in this phase for: `TBD`, `FIXME`, `XXX`, `placeholder`, `return null`, `return []`, hardcoded empty values, console.log-only implementations. No actionable anti-patterns found.

Note: `filepath.Dir` appears once in upload.go at a comment — not functional code. Confirmed correct: all remote path operations use `path.Dir` (Linux-appropriate). The one `filepath.Dir` occurrence is in a comment describing the rationale for using `path.Dir`.

### Human Verification Required

#### 1. SC-3: Version line for untagged/dev builds

**Test:** Run `make build && ./bin/docker-deploy version`
**Expected per SC-3 literal:** "Untagged builds print the short git commit hash" — the version line (first line of output) should show the git commit hash for non-tagged builds
**Expected per actual implementation:** The first line reads `Docker Deploy Version dev`, and the second line reads `  Git commit:  <short-hash>`. The hash IS printed but on a separate line, not as the version string.
**Why human:** The ROADMAP SC-3 wording is ambiguous: "the version string is the short git commit hash" could mean (a) the version field on line 1 shows the hash, or (b) the output includes the hash somewhere. The plan spec (D-03 dev format) explicitly shows "Docker Deploy Version dev" with a separate "Git commit:" line — the implementation matches the plan spec. Human needed to confirm whether SC-3 is satisfied by the plan's intended design or whether the version field itself should show the commit hash for dev builds.

### Gaps Summary

No gaps found. All 7 implementation truths are verified in the codebase. The one UNCERTAIN item (SC-3) is an ambiguity between the ROADMAP success criterion wording and the plan's explicit design decision — both the Makefile and GoReleaser inject `version=dev` (not the commit hash) for the version field, with the commit hash on a separate line. This is consistent with Docker CLI's own `docker version` output format.

---

_Verified: 2026-05-26T14:00:00Z_
_Verifier: Claude (gsd-verifier)_
