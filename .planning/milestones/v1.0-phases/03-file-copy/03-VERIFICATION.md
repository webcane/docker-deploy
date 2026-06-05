---
phase: 03-file-copy
verified: 2026-05-15T00:00:00Z
status: passed
score: 13/13 must-haves verified
overrides_applied: 0
re_verification:
  previous_status: passed
  previous_score: 11/11
  gaps_closed:
    - "First deploy places files directly under remoteBase (not inside a timestamped subdirectory): upload.go:207 rm -rf remoteBase before mv; TestUploadFirstDeploy_RmBeforeMv PASS"
    - "Repeat deploy continues to work correctly via existing three-step atomic swap: TestUploadRepeatDeploy_ThreeStepSwapUnchanged PASS"
  gaps_remaining: []
  regressions: []
---

# Phase 3: File Copy Verification Report (Re-verification after 03-05 gap closure)

**Phase Goal:** Local project files are transferred to the remote host atomically via SFTP using smart defaults and user-defined overrides
**Verified:** 2026-05-15
**Status:** passed
**Re-verification:** Yes — after 03-05 gap closure (first-deploy mv nesting bug)

## Goal Achievement

### Observable Truths

| # | Truth | Status | Evidence |
|---|-------|--------|----------|
| 1 | Config.Excludes populated from flag > deploy.yaml > built-in defaults | VERIFIED | `mergeExcludes()` in config.go; `Resolve()` 6-arg signature; TestResolveExcludes 5/5 pass |
| 2 | Config.Force populated from flag > deploy.yaml > defaults | VERIFIED | `cfg.Force = flagForce \|\| file.Target.Force` in config.go; TestResolveForce 4/4 pass |
| 3 | Built-in default excludes always present in Config.Excludes | VERIFIED | `defaultExcludes` (6 patterns) in config.go; mergeExcludes always starts with defaults |
| 4 | User-supplied excludes extend, never replace, the built-in list | VERIFIED | mergeExcludes appends file then flag excludes using seen-map; no mechanism to remove defaults |
| 5 | Files matching any exclude pattern are not uploaded | VERIFIED | ShouldExclude() in filter.go; 10 TestShouldExclude cases pass |
| 6 | Files staged under /tmp/docker-deploy-<timestamp> before being moved | VERIFIED | upload.go:61 `stagingDir := "/tmp/docker-deploy-" + timestamp` |
| 7 | An atomic rename replaces the target directory; a failed mid-swap never corrupts the target | VERIFIED | upload.go:181-213: existsBefore branch 3-step swap with rollback at lines 192-196; first-deploy else branch rm+mv at lines 207-212 |
| 8 | SFTP session wraps the existing *gossh.Client — no second TCP connection | VERIFIED | upload.go:54 `sftp.NewClient(client)`; no second Dial(); ShellQuote uses strings.ReplaceAll |
| 9 | --exclude flag adds entries to the exclude list | VERIFIED | main.go:46 `StringArrayVar(&excludes, "exclude", ...)`; Resolve called with excludes |
| 10 | --force flag skips the replace-confirmation prompt | VERIFIED | main.go:174 `if !resolved.Force { ... prompt ... }`; BoolVar registered at main.go:47 |
| 11 | End-to-end: docker deploy copies files to remote, .env included, .git/ excluded | VERIFIED | Human checkpoint APPROVED (03-04-SUMMARY.md): first deploy and repeat deploy both succeeded against real SSH host with root-owned /opt/ |
| 12 | First deploy places files directly under remoteBase (not inside a timestamped subdirectory) | VERIFIED | upload.go:207 `sudoRun(fmt.Sprintf("rm -rf %s", ShellQuote(remoteBase)))` before mv; TestUploadFirstDeploy_RmBeforeMv PASS — test logs confirm command order: mkdir-p, rm-rf, mv |
| 13 | Repeat deploy continues to work correctly via the existing three-step atomic swap | VERIFIED | upload.go:181-200 existsBefore=true branch unchanged; TestUploadRepeatDeploy_ThreeStepSwapUnchanged PASS; test additionally asserts no direct rm-rf of remoteBase in repeat path |

**Score:** 13/13 truths verified

### Required Artifacts

| Artifact | Expected | Status | Details |
|----------|----------|--------|---------|
| `internal/config/config.go` | Extended Config/TargetConfig structs, updated Resolve() 6-arg signature | VERIFIED | Excludes []string, Force bool on Config; Exclude []string, Force bool on TargetConfig |
| `internal/config/config_test.go` | Table-driven tests for Excludes and Force | VERIFIED | TestResolveExcludes (5 cases), TestResolveForce (4 cases) — all pass |
| `internal/filetransfer/filter.go` | ShouldExclude + WalkFiles | VERIFIED | Both functions exported; 10 ShouldExclude test cases pass |
| `internal/filetransfer/filter_test.go` | TestShouldExclude table-driven tests | VERIFIED | 10 sub-cases; TestWalkFiles; TestWalkFilesSkipsDirs all pass |
| `internal/filetransfer/upload.go` | Upload() with SFTP staging, atomic rename, ShellQuote, sudoRun, first-deploy rm fix | VERIFIED | else branch (lines 201-213): rm -rf remoteBase at line 207, mv at line 210; distinct error "removing target placeholder before first deploy"; existsBefore=true path unchanged |
| `internal/filetransfer/upload_test.go` | Regression tests for first-deploy and repeat-deploy | VERIFIED | TestUploadFirstDeploy_RmBeforeMv: asserts rm-rf precedes mv for first deploy; TestUploadRepeatDeploy_ThreeStepSwapUnchanged: asserts three-step swap and no direct rm-rf on base; both PASS |
| `internal/filetransfer/shellquote_test.go` | Table-driven tests for ShellQuote | VERIFIED | 5 cases including embedded single quote; all pass |
| `cmd/docker-deploy/main.go` | Wired deploy: Resolve -> Dial -> confirm-or-skip -> Upload; ShellQuote for existence check | VERIFIED | runDeploy() full chain; line 179 uses `filetransfer.ShellQuote(resolved.Path)` |

### Key Link Verification

| From | To | Via | Status | Details |
|------|----|-----|--------|---------|
| `main.go` | `internal/filetransfer/upload.go` | `filetransfer.Upload(ctx, client, cwd, resolved.Path, resolved.Excludes)` | WIRED | main.go:206 confirmed |
| `main.go` | `internal/config/config.go` | `config.Resolve(host, path, excludes, force, fileConfig, projectName)` | WIRED | main.go:74 (dry-run) and main.go:140 (deploy) |
| `upload.go` | `github.com/pkg/sftp` | `sftp.NewClient(sshClient)` | WIRED | upload.go:54 confirmed |
| `upload.go` | `filter.go` | `WalkFiles()` called in Upload | WIRED | upload.go:41 |
| `main.go` | `filetransfer.ShellQuote` | `filetransfer.ShellQuote(resolved.Path)` in existence check | WIRED | main.go:179 — CR-01 fix confirmed |
| `sudoRun` | `remoteBase mv/rm ops` | closure reuses sudoPw for all privileged operations | WIRED | upload.go:135-160; used at lines 163, 188, 191, 198, 203, 207, 210 |
| `upload.go else branch` | `mv stagingDir remoteBase` | `sudoRun rm -rf remoteBase` inserted before mv | WIRED | upload.go:207 rm-rf then upload.go:210 mv; ShellQuote wraps remoteBase in both calls |

### Data-Flow Trace (Level 4)

| Artifact | Data Variable | Source | Produces Real Data | Status |
|----------|---------------|--------|--------------------|--------|
| `upload.go:Upload()` | `files []string` | `WalkFiles(localDir, excludes)` via `filepath.WalkDir` | Yes — walks actual local directory | FLOWING |
| `filter.go:ShouldExclude()` | `relPath, excludes` | Caller-supplied; pass-through filter | Yes | FLOWING |
| `main.go:runDeploy()` | `resolved Config` | `config.Resolve()` — merges flags + file + defaults | Yes | FLOWING |

### Behavioral Spot-Checks

| Behavior | Command | Result | Status |
|----------|---------|--------|--------|
| `go build ./...` exits 0 | `go build ./...` | exit 0, no output | PASS |
| `go test ./... -count=1` exits 0 | `go test ./... -count=1` | config PASS (0.459s), filetransfer PASS (0.967s) | PASS |
| TestUploadFirstDeploy_RmBeforeMv passes | `go test ./internal/filetransfer/... -v -run TestUpload` | PASS (0.18s); command log confirms rm-rf before mv in first-deploy path | PASS |
| TestUploadRepeatDeploy_ThreeStepSwapUnchanged passes | `go test ./internal/filetransfer/... -v -run TestUpload` | PASS (0.11s); command log confirms mv-to-backup, mv-staging, rm-backup; no direct rm-rf on base | PASS |
| TestShellQuote all 5 cases pass | included in `go test ./internal/filetransfer/...` | 5/5 PASS including embedded single-quote case | PASS |
| TestResolveExcludes all 5 cases pass | included in `go test ./internal/config/...` | 5/5 PASS | PASS |
| TestResolveForce all 4 cases pass | included in `go test ./internal/config/...` | 4/4 PASS | PASS |

### Probe Execution

No probe scripts found in `scripts/*/tests/probe-*.sh`. Step 7c: SKIPPED (no probes defined for this phase).

### Requirements Coverage

| Requirement | Source Plan | Description | Status | Evidence |
|-------------|------------|-------------|--------|----------|
| DEPLOY-02 | 03-02 | Project files copied via SFTP over existing SSH connection | SATISFIED | `sftp.NewClient(client)` wraps existing `*gossh.Client`; no second TCP dial; upload.go:54 |
| DEPLOY-03 | 03-02, 03-03 | Files staged to `/tmp/docker-deploy-<timestamp>` first, then moved atomically | SATISFIED | upload.go:61 staging pattern; both deploy paths (first and repeat) perform atomic placement; regression tests confirm correct command sequence |
| FILES-01 | 03-02 | Default include list (compose.yaml, .env, Makefile, README.md) | SATISFIED | Exclude-only model: all files included by default; none of these appear in defaultExcludes |
| FILES-02 | 03-02 | Default exclude list: .git/, node_modules/, vendor/, *.log, .DS_Store, __pycache__/ | SATISFIED | config.go `defaultExcludes` contains all 6 patterns |
| FILES-03 | 03-01 | User can extend exclude lists via deploy.yaml | SATISFIED | TargetConfig.Exclude []string yaml-tagged; mergeExcludes appends to defaults |
| WR-07 | 03-05 | Not a REQUIREMENTS.md ID — internal plan label only | NOTE | WR-07 and WR-08 appear only in 03-05-PLAN.md frontmatter `requirements:` field and have no corresponding entries in REQUIREMENTS.md. They are not tracked requirement IDs. The functional content of plan 03-05 is fully covered by DEPLOY-03 (atomic staging). No orphaned official requirements. |
| WR-08 | 03-05 | Not a REQUIREMENTS.md ID — internal plan label only | NOTE | See WR-07 note above. |

All 5 official Phase 3 requirement IDs (DEPLOY-02, DEPLOY-03, FILES-01, FILES-02, FILES-03) are accounted for and satisfied. WR-07 and WR-08 are internal plan labels without REQUIREMENTS.md entries — no orphaned official requirements.

### Anti-Patterns Found

| File | Line | Pattern | Severity | Impact |
|------|------|---------|----------|--------|
| `internal/filetransfer/upload.go` | 93 | `OpenFile` does not preserve file permissions | WARNING | WR-01 (03-REVIEW): executable scripts lose +x on upload; entrypoint scripts may fail on remote. Not a Phase 3 blocker — human UAT approved, known pre-existing issue. |
| `cmd/docker-deploy/main.go` | 199-206 | `WalkFiles` called twice (for count, then inside Upload) | WARNING | WR-02 (03-REVIEW): TOCTOU race; file count in success message may differ from actual upload. Low risk in practice. Pre-existing. |
| `internal/filetransfer/upload.go` | 148, 159 | Sudo password may appear in error chain via `sshExec` format string | WARNING | CR-01 (03-REVIEW): if a post-authentication sudoRun call fails, the error includes the full command string with the cleartext password. Identified in code review, not fixed in Phase 3. |

**Debt marker gate:** No TBD, FIXME, or XXX markers found in any Phase 3 modified files. Debt marker gate: PASS.

**03-05 WR-07/WR-08 note:** These IDs in the plan frontmatter `requirements:` field do not correspond to any entry in REQUIREMENTS.md. They appear to be mislabeled references (possibly intended as internal warning/risk tags analogous to the WR-01..WR-04 code review findings). The plan's actual content addresses DEPLOY-03 (atomic staging correctness). No official requirements are orphaned.

**03-04 deviation note (carried forward):** The plan's "Ownership of remoteBase is set to the connecting user (chown)" must-have was not implemented literally — the `sudoRun` lazy approach instead retries every command with sudo when needed, making explicit chown unnecessary for the deploy to succeed. Human checkpoint confirmed both first and repeat deploys succeed against a real root-owned /opt/.

### Human Verification Required

None — all human UAT items are closed. The UAT-detected first-deploy mv nesting bug is now covered by `TestUploadFirstDeploy_RmBeforeMv` (automated regression test, PASS).

### Gaps Summary

No open gaps. Phase 3 goal is achieved.

The 03-05 gap closure correctly fixed the first-deploy mv nesting bug:
- `upload.go` else branch (lines 201-213): `sudoRun("rm -rf remoteBase")` at line 207 removes the empty placeholder directory before `mv stagingDir remoteBase` at line 210
- The fix is scoped to the `existsBefore=false` path only; the `existsBefore=true` (repeat-deploy) three-step swap at lines 181-200 is unchanged
- `TestUploadFirstDeploy_RmBeforeMv` and `TestUploadRepeatDeploy_ThreeStepSwapUnchanged` both pass, with command-order assertions that directly enforce the required behavior
- Full test suite (`go test ./... -count=1`) passes; `go build ./...` passes

Outstanding warnings (WR-01 file permissions, WR-02 double WalkFiles, CR-01 sudo password in error chain from 03-REVIEW) are pre-existing code quality issues that do not block the phase goal. They are candidates for future gap-closure plans or Phase 4 cleanup.

---

_Verified: 2026-05-15_
_Verifier: Claude (gsd-verifier)_
