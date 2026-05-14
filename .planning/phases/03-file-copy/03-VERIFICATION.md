---
phase: 03-file-copy
verified: 2026-05-14T00:00:00Z
status: human_needed
score: 9/11 must-haves verified
overrides_applied: 0
human_verification:
  - test: "End-to-end deploy against real SSH host: first deploy (target absent)"
    expected: "Files upload to remote, .git/ excluded, .env present, staging dir created then renamed to /opt/<project>"
    why_human: "Requires a live SSH server; cannot verify SFTP transfer, atomic rename, or directory state without connecting"
  - test: "End-to-end deploy: repeat deploy prompts for confirmation; default Enter cancels"
    expected: "Prompt 'Target ... exists on ... Replace all contents? [y/N]' shown; pressing Enter cancels without error"
    why_human: "Interactive stdin behavior and remote directory detection cannot be verified without a live SSH host"
  - test: "--force skips confirmation prompt"
    expected: "No prompt shown, deploy proceeds immediately"
    why_human: "Requires a live SSH host with an existing target directory"
  - test: "File permissions preserved: executable scripts retain +x after upload"
    expected: "An entrypoint.sh with chmod +x locally arrives on remote with execute bit set"
    why_human: "WR-01 (code review): upload.go does not call sftpClient.Chmod() — permissions are NOT preserved. This is a functional gap that needs human confirmation of the actual impact and decision on fix priority"
gaps:
  - truth: "An atomic rename replaces the target directory; a failed mid-swap never corrupts the target"
    status: partial
    reason: "CR-02 (code review): if the second mv (stagingDir -> remoteBase) fails after the first mv (remoteBase -> remoteBase.old-<ts>) has succeeded, the code returns an error without attempting rollback. remoteBase is absent at that point. No rollback is attempted. Tracked in 03-REVIEW.md CR-02."
    artifacts:
      - path: "internal/filetransfer/upload.go"
        issue: "Lines 184-191: three-step swap has no rollback on step-2 failure; error message does not tell operator where the backup is"
    missing:
      - "Rollback attempt: if mv stagingDir remoteBase fails, attempt mv remoteBase.old-<ts> remoteBase before returning"
      - "Or at minimum: include oldDir path in the error message for manual recovery"
  - truth: "SFTP session wraps the existing *gossh.Client — no second TCP connection"
    status: partial
    reason: "CR-01 (code review): shellQuote() does not escape embedded single quotes. A path containing a single quote breaks out of the shell quoting context, enabling command injection on the remote. The same flaw exists inline in main.go:179 where resolved.Path is interpolated into a shell command without using shellQuote. Tracked in 03-REVIEW.md CR-01."
    artifacts:
      - path: "internal/filetransfer/upload.go"
        issue: "Line 252-254: shellQuote() returns \"'\" + s + \"'\" without escaping embedded single quotes"
      - path: "cmd/docker-deploy/main.go"
        issue: "Line 179: inline fmt.Sprintf interpolates resolved.Path inside single-quoted shell command without escaping"
    missing:
      - "shellQuote must replace each ' with '\\'' before wrapping"
      - "main.go:179 must also use the fixed shellQuote (or export it from filetransfer)"
---

# Phase 3: File Copy Verification Report

**Phase Goal:** Implement atomic SFTP file copy with exclude filtering so `docker deploy` copies local project files to the remote VPS, excluding noise patterns, and stages atomically.
**Verified:** 2026-05-14
**Status:** human_needed (2 tracked REVIEW issues affect two must-have truths; 4 items need live-SSH human verification)
**Re-verification:** No — initial verification

## Goal Achievement

### Observable Truths

| # | Truth | Status | Evidence |
|---|-------|--------|----------|
| 1 | Config.Excludes populated from flag > deploy.yaml > built-in defaults | VERIFIED | `mergeExcludes()` in config.go:125; `Resolve()` at line 195; TestResolveExcludes all 5 sub-cases pass |
| 2 | Config.Force populated from flag > deploy.yaml > defaults | VERIFIED | `cfg.Force = flagForce \|\| file.Target.Force` at config.go:196; TestResolveForce all 4 sub-cases pass |
| 3 | Built-in default excludes are always present in Config.Excludes | VERIFIED | `defaultExcludes` var (6 patterns) at config.go:18; mergeExcludes always starts with defaults; dedup preserves them |
| 4 | User-supplied excludes extend, never replace, the built-in list | VERIFIED | mergeExcludes appends file then flag excludes to defaults using seen-map; no mechanism to remove defaults |
| 5 | Files matching any exclude pattern are not uploaded | VERIFIED | ShouldExclude() in filter.go handles directory-prefix, deep-path-component, glob-basename, and exact-match; 10 TestShouldExclude cases all pass |
| 6 | Files staged under /tmp/docker-deploy-<timestamp> before being moved | VERIFIED | upload.go:60 `stagingDir := "/tmp/docker-deploy-" + timestamp`; matches DEPLOY-03 and REQUIREMENTS.md |
| 7 | An atomic rename replaces the target directory; a failed mid-swap never corrupts the target | PARTIAL (WARNING) | Three-step swap implemented (upload.go:182-198); however CR-02: if mv2 (staging→remoteBase) fails, no rollback of mv1 (remoteBase→old) is attempted — remoteBase ends up absent. Tracked in 03-REVIEW.md. |
| 8 | SFTP session wraps the existing *gossh.Client — no second TCP connection | VERIFIED (with warning) | upload.go:53 `sftp.NewClient(client)` wraps same client; no second Dial(). CR-01 shellQuote flaw is a security issue but does not affect the transport wiring truth. |
| 9 | --exclude flag adds entries to the exclude list | VERIFIED | main.go:46 `StringArrayVar(&excludes, "exclude", ...)` ; Resolve called with excludes at main.go:74,140 |
| 10 | --force flag skips the replace-confirmation prompt | VERIFIED | main.go:174 `if !resolved.Force { ... prompt ... }`; BoolVar registered at main.go:47 |
| 11 | End-to-end: docker deploy copies files to remote, .env included, .git/ excluded | UNCERTAIN | Cannot verify without a live SSH host. SUMMARY reports human checkpoint approved against 192.168.1.99 but verifier cannot re-run this. Routed to human verification. |

**Score:** 9/11 truths verified (1 partial/warning, 1 uncertain/human-needed)

### Required Artifacts

| Artifact | Expected | Status | Details |
|----------|----------|--------|---------|
| `internal/config/config.go` | Extended Config/TargetConfig structs, updated Resolve() 6-arg signature | VERIFIED | Excludes []string, Force bool on Config; Exclude []string, Force bool on TargetConfig; Resolve takes 6 params |
| `internal/config/config_test.go` | Table-driven tests for Excludes and Force | VERIFIED | TestResolveExcludes (5 cases), TestResolveForce (4 cases) — all pass |
| `internal/filetransfer/filter.go` | ShouldExclude + WalkFiles | VERIFIED | Both functions exported; 10 ShouldExclude test cases pass; WalkFiles returns sorted paths |
| `internal/filetransfer/filter_test.go` | TestShouldExclude table-driven tests | VERIFIED | 10 sub-cases including .env not-excluded case; TestWalkFiles; TestWalkFilesSkipsDirs |
| `internal/filetransfer/upload.go` | Upload() with SFTP staging and atomic rename | VERIFIED (with warning) | Upload exported, sftp.NewClient wraps client, staging in /tmp, three-step swap present; CR-01 and CR-02 are tracked issues |
| `cmd/docker-deploy/main.go` | Wired deploy: Resolve → Dial → confirm-or-skip → Upload | VERIFIED | runDeploy() implements full chain; --exclude and --force flags registered |

### Key Link Verification

| From | To | Via | Status | Details |
|------|----|-----|--------|---------|
| `main.go` | `internal/filetransfer/upload.go` | `filetransfer.Upload(ctx, client, cwd, resolved.Path, resolved.Excludes)` | WIRED | main.go:206 — exact call confirmed |
| `main.go` | `internal/config/config.go` | `config.Resolve(host, path, excludes, force, fileConfig, projectName)` | WIRED | main.go:74 (dry-run) and main.go:140 (deploy) — 6-arg calls confirmed |
| `upload.go` | `github.com/pkg/sftp` | `sftp.NewClient(sshClient)` | WIRED | upload.go:53 confirmed |
| `upload.go` | `filter.go` | `WalkFiles()` called in Upload | WIRED | upload.go:41 `files, err := WalkFiles(localDir, excludes)` |

### Data-Flow Trace (Level 4)

| Artifact | Data Variable | Source | Produces Real Data | Status |
|----------|---------------|--------|--------------------|--------|
| `upload.go:Upload()` | `files []string` | `WalkFiles(localDir, excludes)` — reads filesystem via `filepath.WalkDir` | Yes — walks actual local directory | FLOWING |
| `filter.go:ShouldExclude()` | `relPath, excludes` | Caller-supplied; no state | Pass-through filter | FLOWING |
| `main.go:runDeploy()` | `resolved Config` | `config.Resolve()` — merges flags + file + defaults | Yes — real merge | FLOWING |

### Behavioral Spot-Checks

| Behavior | Command | Result | Status |
|----------|---------|--------|--------|
| `go build ./...` exits 0 | `go build ./...` | exit 0, no output | PASS |
| `go vet ./...` exits 0 | `go vet ./...` | exit 0, no output | PASS |
| `go test ./...` exits 0 | `go test ./...` | all packages pass (config + filetransfer); cmd/docker-deploy has no tests | PASS |
| `--exclude` and `--force` appear in `docker deploy --help` | binary help output | Both flags present with correct descriptions | PASS |
| TestResolveExcludes all 5 cases pass | `go test ./internal/config/ -v -run TestResolveExcludes` | 5/5 PASS | PASS |
| TestResolveForce all 4 cases pass | `go test ./internal/config/ -v -run TestResolveForce` | 4/4 PASS | PASS |
| TestShouldExclude all 10 cases pass | `go test ./internal/filetransfer/ -v` | 10/10 PASS; TestWalkFiles PASS; TestWalkFilesSkipsDirs PASS | PASS |

### Probe Execution

No probe scripts found in `scripts/*/tests/probe-*.sh`. Step 7c: SKIPPED (no probes defined for this phase).

### Requirements Coverage

| Requirement | Source Plan | Description | Status | Evidence |
|-------------|------------|-------------|--------|----------|
| DEPLOY-02 | 03-02 | Project files copied via SFTP over existing SSH connection | SATISFIED | `sftp.NewClient(client)` wraps existing `*gossh.Client`; no second TCP dial; upload.go:53 |
| DEPLOY-03 | 03-02, 03-03 | Files staged to `/tmp/docker-deploy-<timestamp>` first, then moved atomically | SATISFIED | upload.go:60 staging pattern; REQUIREMENTS.md text matches implementation exactly |
| FILES-01 | 03-02 | Default include list (compose.yaml, .env, Makefile, README.md) | SATISFIED | Exclude-only model (D-01 in 03-CONTEXT.md): all files are included by default; none of compose.yaml, .env, Makefile, README.md appear in defaultExcludes — they are always copied |
| FILES-02 | 03-02 | Default exclude list: .git/, node_modules/, vendor/, *.log, .DS_Store, __pycache__/ | SATISFIED | config.go:18-20 `defaultExcludes` contains all 6 patterns exactly |
| FILES-03 | 03-01 | User can extend exclude lists via deploy.yaml | SATISFIED | TargetConfig.Exclude []string yaml-tagged; mergeExcludes appends it to defaults; --exclude flag also supported |

All 5 Phase 3 requirement IDs (DEPLOY-02, DEPLOY-03, FILES-01, FILES-02, FILES-03) are accounted for and satisfied by the implementation.

### Anti-Patterns Found

| File | Line | Pattern | Severity | Impact |
|------|------|---------|----------|--------|
| `internal/filetransfer/upload.go` | 252-254 | `shellQuote()` does not escape embedded single quotes | BLOCKER (security) | CR-01: command injection on remote for any path containing `'`; tracked in 03-REVIEW.md for fix via `/gsd-code-review --fix` |
| `cmd/docker-deploy/main.go` | 179 | Inline `fmt.Sprintf("test -d '%s' ...")` with unescaped resolved.Path | BLOCKER (security) | Same CR-01 class; not routed through shellQuote; needs same fix |
| `internal/filetransfer/upload.go` | 184-198 | No rollback if mv2 (staging→remoteBase) fails after mv1 (remoteBase→old) succeeds | BLOCKER (correctness) | CR-02: remoteBase goes absent on step-2 mv failure; tracked in 03-REVIEW.md |
| `internal/filetransfer/upload.go` | 190-191 | Cleanup failure (`rm -rf oldDir`) treated as deploy failure, returns error | WARNING | WR-03: deployment succeeded but process exits with error code; misleads CI/CD |
| `internal/filetransfer/upload.go` | 92 | `OpenFile` does not preserve file permissions | WARNING | WR-01: executable scripts lose `+x` on upload; entrypoint scripts will fail to execute on remote |
| `cmd/docker-deploy/main.go` | 199-206 | `WalkFiles` called twice (once for count, once inside Upload) | WARNING | WR-02: TOCTOU race; file count in success message may differ from actual upload count |

**Debt marker gate:** No TBD, FIXME, or XXX markers found in any Phase 3 modified files. Debt marker gate: PASS.

**Anti-pattern note:** The two BLOCKER anti-patterns (CR-01, CR-02) are documented in `03-REVIEW.md` and are explicitly tracked for fix via `/gsd-code-review --fix`. Per the verification instructions, these are factored into scoring but do not block phase progression — they are the subject of a planned fix pass. Truth #7 is marked PARTIAL and Truth #8 is VERIFIED-with-warning accordingly.

### Human Verification Required

#### 1. First Deploy (Target Absent)

**Test:** Create a temp project directory with compose.yaml, .env, README.md, and `.git/config`. Run `docker deploy --host ssh://user@host`.

**Expected:** Only compose.yaml, .env, README.md are uploaded. `.git/config` is absent on remote. `/opt/<project>` exists on remote with correct files.

**Why human:** Requires a live SSH host with SFTP capability. Cannot verify remote filesystem state programmatically without connecting.

#### 2. Repeat Deploy — Confirmation Prompt (Default No)

**Test:** With `/opt/<project>` already on remote, run `docker deploy --host ssh://user@host` and press Enter at the prompt.

**Expected:** Prompt "Target /opt/... exists on ..host.. Replace all contents? [y/N]" appears. Enter cancels without error. Remote directory unchanged.

**Why human:** Interactive stdin behavior and remote directory detection require a live SSH connection.

#### 3. --force Skips Prompt

**Test:** Run `docker deploy --host ssh://user@host --force` with existing target directory.

**Expected:** No prompt shown. Deploy proceeds immediately and succeeds.

**Why human:** Requires live SSH host with existing target directory.

#### 4. File Permissions After Upload (WR-01 Impact Assessment)

**Test:** Create a project with `entrypoint.sh` that has `chmod +x`. Deploy. On remote, run `ls -la /opt/<project>/entrypoint.sh`.

**Expected (per WR-01):** File will likely show `-rw-r--r--` (no execute bit). Confirm the scope of impact.

**Why human:** WR-01 is a code-review finding (warning). Human confirmation of actual behavior is needed to inform fix priority. The `sftpClient.OpenFile()` call does not pass mode attributes.

### Gaps Summary

Two tracked code-review issues affect must-have truths and are classified as gaps pending fix:

**Gap 1 (CR-01 — BLOCKER security):** `shellQuote()` in upload.go does not escape embedded single quotes, enabling command injection for any path value containing a `'` character. The same flaw exists inline in main.go:179. Every SSH exec command in the deploy path (mkdir, mv, rm, test -d) is affected. Fix: replace embedded `'` with `'\''` in shellQuote; apply to main.go:179 as well.

**Gap 2 (CR-02 — BLOCKER correctness):** The three-step atomic swap has no rollback on step-2 failure. If `mv stagingDir remoteBase` fails after `mv remoteBase old` has succeeded, `remoteBase` is absent and no recovery is attempted. Fix: on step-2 failure, attempt `mv old remoteBase` before returning; at minimum, include `oldDir` path in the error message.

Both gaps are documented in `03-REVIEW.md` and are scheduled for repair via `/gsd-code-review --fix`. They do not prevent phase progression but must be closed before Phase 3 is considered fully complete.

---

_Verified: 2026-05-14_
_Verifier: Claude (gsd-verifier)_
