---
phase: 03-file-copy
plan: "02"
subsystem: filetransfer
tags: [go, sftp, filefilter, atomic-deploy, tdd]

# Dependency graph
requires:
  - phase: 02-ssh-transport-config
    provides: "*gossh.Client from Dial()"
  - phase: 03-file-copy
    plan: "01"
    provides: Config.Excludes []string

provides:
  - ShouldExclude(relPath string, excludes []string) bool
  - WalkFiles(localDir string, excludes []string) ([]string, error)
  - Upload(ctx, client, localDir, remoteBase string, excludes []string) error

affects: [03-03-deploy-wiring, 04-core-deploy-loop]

# Tech tracking
tech-stack:
  added:
    - "github.com/pkg/sftp v1.13.10 (direct import added to go.sum)"
  patterns:
    - "Directory pattern matching: exact + prefix + component scan for deep paths"
    - "Glob basename matching: filepath.Match against path.Base(relPath)"
    - "Atomic staging: .deploy-tmp-<timestamp>, mv old to .old-<timestamp>, mv staging to target, rm -rf old"
    - "SSH exec isolation: fresh NewSession() per command per CLAUDE.md rule"

key-files:
  created:
    - internal/filetransfer/filter.go
    - internal/filetransfer/filter_test.go
    - internal/filetransfer/upload.go
  modified:
    - go.sum

key-decisions:
  - "ShouldExclude does three-level directory matching: exact (dir == pattern sans /), prefix (relPath starts with pattern), component scan (any segment equals dir name). This handles both top-level .git/ and deep node_modules/."
  - "WalkFiles uses filepath.SkipDir on excluded directories for efficiency — avoids descending into .git even on large repos"
  - "Upload closes sftpClient before running SSH mv/rename commands (correct resource ordering)"
  - "go get github.com/pkg/sftp was needed to add go.sum entry even though go.mod already listed the package"

patterns-established:
  - "Remote path operations use path (not filepath) — remote is Linux, local may be any OS"
  - "Shell quoting via shellQuote() wraps paths in single quotes — T-03-05 mitigation for SSH exec safety"

requirements-completed: [DEPLOY-02, DEPLOY-03, FILES-01, FILES-02]

# Metrics
duration: 8min
completed: 2026-05-14
---

# Phase 3 Plan 02: filetransfer Package Summary

**File filter (ShouldExclude/WalkFiles) and SFTP Upload with atomic staging implemented; all 13 tests pass; go build and go vet clean**

## Performance

- **Duration:** 8 min
- **Started:** 2026-05-14T17:00:00Z
- **Completed:** 2026-05-14T17:08:00Z
- **Tasks:** 2
- **Files created:** 3 (filter.go, filter_test.go, upload.go)

## Accomplishments

- filter.go: ShouldExclude handles directory prefix patterns (.git/), deep path component matching (node_modules/ anywhere), glob basename matching (*.log), and exact matches (.DS_Store) — .env is never excluded
- filter.go: WalkFiles walks localDir, returns sorted relative paths not excluded; short-circuits with filepath.SkipDir on excluded directories
- filter_test.go: 10 TestShouldExclude sub-cases + TestWalkFiles + TestWalkFilesSkipsDirs — all pass
- upload.go: Upload enumerates via WalkFiles, stages under `<base>.deploy-tmp-<timestamp>`, performs 3-step atomic swap when target exists, single mv on first deploy
- upload.go: SFTP session wraps existing *gossh.Client (no second TCP), each SSH exec uses a fresh session

## Task Commits

Each task was committed atomically:

1. **Task 1 RED: Failing tests for ShouldExclude and WalkFiles** - `dfd3177` (test)
2. **Task 1 GREEN: Implement ShouldExclude and WalkFiles** - `057dc03` (feat)
3. **Task 2: Implement Upload with atomic staging** - `4d66a91` (feat)

## Files Created/Modified

- `internal/filetransfer/filter.go` - ShouldExclude + WalkFiles implementation
- `internal/filetransfer/filter_test.go` - Table-driven tests (13 total)
- `internal/filetransfer/upload.go` - Upload with SFTP staging and atomic rename
- `go.sum` - Added go.sum entries for github.com/pkg/sftp direct import

## Decisions Made

- ShouldExclude does three-level directory matching: exact match, prefix match, and component scan for deep paths (handles `a/node_modules/b/file.js` with pattern `node_modules/`)
- WalkFiles returns filepath.SkipDir on excluded directories (not just skipping the dir entry) — avoids descending into large trees like node_modules
- Upload closes sftpClient before SSH exec commands (resource ordering correctness)
- Remote path operations use `path` package (not `filepath`) since remote is always Linux

## Deviations from Plan

### Auto-fixed Issues

**1. [Rule 3 - Blocking] Added go.sum entry for github.com/pkg/sftp direct import**
- **Found during:** Task 2 verification (go build)
- **Issue:** go.mod listed github.com/pkg/sftp as a dependency but go.sum lacked the entry needed for a direct package import
- **Fix:** Ran `go get github.com/pkg/sftp@v1.13.10` to populate go.sum
- **Files modified:** go.sum
- **Verification:** go build ./internal/filetransfer/ passes
- **Committed in:** 4d66a91 (Task 2 commit)

---

**Total deviations:** 1 auto-fixed (blocking - missing go.sum entry)
**Impact on plan:** No scope change. The go.sum update is a standard Go module maintenance step.

## Known Stubs

None — Upload is fully implemented but untested against a real SFTP server. Integration tests will be wired in Plan 03-03 (deploy wiring) or Phase 4.

## Threat Surface Scan

T-03-05 mitigated: shellQuote() wraps all SSH exec path arguments in single quotes. remoteBase derives from config.Resolve() (validated). Staging dir name uses only alphanumerics + Unix timestamp integer — no shell-special characters.

No new threat surface beyond what the plan's threat model covers.

## Self-Check

- `internal/filetransfer/filter.go` — exists
- `internal/filetransfer/filter_test.go` — exists
- `internal/filetransfer/upload.go` — exists
- Commits dfd3177, 057dc03, 4d66a91 — verified via git log

## Self-Check: PASSED

---
*Phase: 03-file-copy*
*Completed: 2026-05-14*
