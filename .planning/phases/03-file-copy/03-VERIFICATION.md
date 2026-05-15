---
phase: 03-file-copy
verified: 2026-05-15T00:00:00Z
status: passed
score: 11/11 must-haves verified
overrides_applied: 0
re_verification:
  previous_status: human_needed
  previous_score: 9/11
  gaps_closed:
    - "shellQuote escapes embedded single quotes (CR-01): ShellQuote now uses strings.ReplaceAll; main.go:179 uses filetransfer.ShellQuote(resolved.Path)"
    - "Atomic swap rollback on step-2 failure (CR-02): upload.go lines 192-196 restore remoteBase from backup on failure; error includes backup path"
    - "First deploy against root-owned /opt: sudoRun lazy-sudo collects password interactively and retries; human checkpoint APPROVED"
    - "Repeat deploy confirmation prompt: human checkpoint APPROVED against real SSH host"
  gaps_remaining: []
  regressions: []
---

# Phase 3: File Copy Verification Report (Re-verification)

**Phase Goal:** Implement atomic SFTP file copy with exclude filtering so `docker deploy` copies local project files to the remote VPS, excluding noise patterns, and stages atomically.
**Verified:** 2026-05-15
**Status:** passed
**Re-verification:** Yes — after gap closure (03-04 plan) and human UAT approval

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
| 7 | An atomic rename replaces the target directory; a failed mid-swap never corrupts the target | VERIFIED | upload.go:188-199: 3-step swap; step-2 failure triggers rollback at lines 192-196; error includes backup path (`oldDir`) |
| 8 | SFTP session wraps the existing *gossh.Client — no second TCP connection | VERIFIED | upload.go:54 `sftp.NewClient(client)`; no second Dial(); ShellQuote CR-01 fix confirmed |
| 9 | --exclude flag adds entries to the exclude list | VERIFIED | main.go:46 `StringArrayVar(&excludes, "exclude", ...)`; Resolve called with excludes |
| 10 | --force flag skips the replace-confirmation prompt | VERIFIED | main.go:174 `if !resolved.Force { ... prompt ... }`; BoolVar registered at main.go:47 |
| 11 | End-to-end: docker deploy copies files to remote, .env included, .git/ excluded | VERIFIED | Human checkpoint APPROVED (03-04-SUMMARY.md): first deploy and repeat deploy both succeeded against real SSH host with root-owned /opt/ |

**Score:** 11/11 truths verified

### Required Artifacts

| Artifact | Expected | Status | Details |
|----------|----------|--------|---------|
| `internal/config/config.go` | Extended Config/TargetConfig structs, updated Resolve() 6-arg signature | VERIFIED | Excludes []string, Force bool on Config; Exclude []string, Force bool on TargetConfig |
| `internal/config/config_test.go` | Table-driven tests for Excludes and Force | VERIFIED | TestResolveExcludes (5 cases), TestResolveForce (4 cases) — all pass |
| `internal/filetransfer/filter.go` | ShouldExclude + WalkFiles | VERIFIED | Both functions exported; 10 ShouldExclude test cases pass |
| `internal/filetransfer/filter_test.go` | TestShouldExclude table-driven tests | VERIFIED | 10 sub-cases; TestWalkFiles; TestWalkFilesSkipsDirs all pass |
| `internal/filetransfer/upload.go` | Upload() with SFTP staging, atomic rename, ShellQuote, sudoRun | VERIFIED | ShellQuote uses strings.ReplaceAll; sudoRun lazy-sudo closure; 3-step swap with rollback; WR-01 permission warning unchanged |
| `internal/filetransfer/shellquote_test.go` | Table-driven tests for ShellQuote | VERIFIED | 5 cases including embedded single quote; all pass |
| `cmd/docker-deploy/main.go` | Wired deploy: Resolve -> Dial -> confirm-or-skip -> Upload; ShellQuote for existence check | VERIFIED | runDeploy() full chain; line 179 uses `filetransfer.ShellQuote(resolved.Path)` |

### Key Link Verification

| From | To | Via | Status | Details |
|------|----|-----|--------|---------|
| `main.go` | `internal/filetransfer/upload.go` | `filetransfer.Upload(ctx, client, cwd, resolved.Path, resolved.Excludes)` | WIRED | main.go:206 — confirmed |
| `main.go` | `internal/config/config.go` | `config.Resolve(host, path, excludes, force, fileConfig, projectName)` | WIRED | main.go:74 (dry-run) and main.go:140 (deploy) |
| `upload.go` | `github.com/pkg/sftp` | `sftp.NewClient(sshClient)` | WIRED | upload.go:54 confirmed |
| `upload.go` | `filter.go` | `WalkFiles()` called in Upload | WIRED | upload.go:41 |
| `main.go` | `filetransfer.ShellQuote` | `filetransfer.ShellQuote(resolved.Path)` in existence check | WIRED | main.go:179 — CR-01 fix confirmed |
| `sudoRun` | `remoteBase mv/rm ops` | closure reuses sudoPw for all privileged operations | WIRED | upload.go:135-160; used at lines 163, 188, 191, 198, 203 |

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
| `go vet ./...` exits 0 | `go vet ./...` | exit 0, no output | PASS |
| `go test ./...` exits 0 | `go test ./... -count=1` | config PASS, filetransfer PASS | PASS |
| TestShellQuote all 5 cases pass | `go test ./internal/filetransfer/ -v -run TestShellQuote` | 5/5 PASS including embedded single-quote case | PASS |
| TestResolveExcludes all 5 cases pass | `go test ./internal/config/ -v -run TestResolveExcludes` | 5/5 PASS | PASS |
| TestResolveForce all 4 cases pass | `go test ./internal/config/ -v -run TestResolveForce` | 4/4 PASS | PASS |
| TestShouldExclude all 10 cases pass | `go test ./internal/filetransfer/ -v` | 10/10 PASS; TestWalkFiles PASS; TestWalkFilesSkipsDirs PASS | PASS |

### Probe Execution

No probe scripts found in `scripts/*/tests/probe-*.sh`. Step 7c: SKIPPED (no probes defined for this phase).

### Requirements Coverage

| Requirement | Source Plan | Description | Status | Evidence |
|-------------|------------|-------------|--------|----------|
| DEPLOY-02 | 03-02 | Project files copied via SFTP over existing SSH connection | SATISFIED | `sftp.NewClient(client)` wraps existing `*gossh.Client`; no second TCP dial; upload.go:54 |
| DEPLOY-03 | 03-02, 03-03 | Files staged to `/tmp/docker-deploy-<timestamp>` first, then moved atomically | SATISFIED | upload.go:61 staging pattern; 3-step swap with rollback at lines 181-205 |
| FILES-01 | 03-02 | Default include list (compose.yaml, .env, Makefile, README.md) | SATISFIED | Exclude-only model: all files included by default; none of these appear in defaultExcludes |
| FILES-02 | 03-02 | Default exclude list: .git/, node_modules/, vendor/, *.log, .DS_Store, __pycache__/ | SATISFIED | config.go `defaultExcludes` contains all 6 patterns |
| FILES-03 | 03-01 | User can extend exclude lists via deploy.yaml | SATISFIED | TargetConfig.Exclude []string yaml-tagged; mergeExcludes appends to defaults |

All 5 Phase 3 requirement IDs (DEPLOY-02, DEPLOY-03, FILES-01, FILES-02, FILES-03) are accounted for and satisfied.

### Anti-Patterns Found

| File | Line | Pattern | Severity | Impact |
|------|------|---------|----------|--------|
| `internal/filetransfer/upload.go` | 93 | `OpenFile` does not preserve file permissions | WARNING | WR-01: executable scripts lose +x on upload; entrypoint scripts may fail on remote. Not a Phase 3 blocker — human UAT approved. |
| `cmd/docker-deploy/main.go` | 199-206 | `WalkFiles` called twice (for count, then inside Upload) | WARNING | WR-02: TOCTOU race; file count in success message may differ from actual upload. Low risk in practice. |

**Debt marker gate:** No TBD, FIXME, or XXX markers found in any Phase 3 modified files. Debt marker gate: PASS.

**03-04 deviation note:** The plan's "Ownership of remoteBase is set to the connecting user (chown)" must-have was not implemented literally — the `sudoRun` lazy approach instead retries every command with sudo when needed, making explicit chown unnecessary for the deploy to succeed. This is a safe deviation: human checkpoint confirmed both first and repeat deploys succeed against a real root-owned /opt/.

### Human Verification Required

None — all human UAT items from the initial verification are closed:

1. First deploy (target absent): APPROVED — files uploaded, .git/ excluded, staging dir renamed to /opt/<project>
2. Repeat deploy confirmation prompt: APPROVED — prompt shown, Enter cancels, y proceeds
3. --force skips prompt: code-verified (main.go:174 `if !resolved.Force`)
4. File permissions WR-01: acknowledged as WARNING; human UAT proceeded without requiring +x preservation

### Gaps Summary

No open gaps. All two BLOCKER gaps from the initial verification are closed:

- CR-01 (shellQuote injection): Resolved — `ShellQuote` uses `strings.ReplaceAll(s, "'", "'\\''")` at upload.go:262; main.go:179 uses `filetransfer.ShellQuote(resolved.Path)`. TestShellQuote 5/5 pass.
- CR-02 (no rollback): Resolved — 3-step swap at upload.go:181-205; step-2 failure triggers `mv oldDir remoteBase` rollback at line 193; error messages at lines 194 and 196 include the backup path for manual recovery.

Remaining warnings (WR-01 file permissions, WR-02 double WalkFiles) are pre-existing and do not block the phase goal.

---

_Verified: 2026-05-15_
_Verifier: Claude (gsd-verifier)_
