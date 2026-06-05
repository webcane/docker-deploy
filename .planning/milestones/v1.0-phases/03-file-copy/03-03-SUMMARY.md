---
phase: 03-file-copy
plan: "03"
subsystem: deploy-wiring
tags: [go, cobra, sftp, ssh, staging, sudo, interactive-prompt]

# Dependency graph
requires:
  - phase: 03-file-copy
    plan: "01"
    provides: "config.Resolve() 6-arg signature, Config.Excludes, Config.Force"
  - phase: 03-file-copy
    plan: "02"
    provides: "filetransfer.Upload(), filetransfer.WalkFiles(), atomic staging"
  - phase: 02-ssh-transport-config
    provides: "*gossh.Client from Dial()"

provides:
  - "Functional deploy path: Resolve() -> Dial() -> exists-check -> prompt-or-skip -> Upload()"
  - "--exclude (repeatable StringArrayVar) and --force BoolVar flags on cobra command"
  - "Replace-confirmation prompt (default No) for repeat deploys; --force or force: true skips it"
  - "Interactive sudo fallback for target directory creation on remote"
  - "Graceful staged-file recovery when target dir creation fails"

affects: [04-core-deploy-loop, 05-preflight-health, 06-init-wizard]

# Tech tracking
tech-stack:
  added: []
  patterns:
    - "Remote exists-check: dedicated SSH NewSession per CLAUDE.md rule, test -d via session.Output()"
    - "Replace-confirmation: bufio.Scanner on os.Stdin, case-insensitive y/yes check, default No on Enter"
    - "Staging in /tmp/docker-deploy-<unix_ts>: avoids permission issues with /opt hierarchy"
    - "Interactive sudo: term.ReadPassword + sudo -S -p '' with up to 3 password attempts"
    - "Graceful staging fallback: leaves /tmp staged files with manual recovery hint when target creation fails"

key-files:
  created: []
  modified:
    - cmd/docker-deploy/main.go
    - internal/filetransfer/upload.go

key-decisions:
  - "Staging moved from sibling-of-target (.deploy-tmp-<ts> in /opt) to /tmp/docker-deploy-<ts> — avoids permission requirements for staging; target dir creation is a separate step with sudo fallback"
  - "Interactive sudo uses term.ReadPassword (not echo/pipe) for password input on the local terminal — cleaner UX, no password in process args"
  - "Up to 3 sudo password attempts before graceful fallback — matches familiar sudo retry UX"
  - "On sudo failure: leave staged files in /tmp for manual recovery, print exact recovery commands with host and paths"
  - "Replace-confirmation default is No ([y/N]) — accidental Enter never overwrites existing remote target (T-03-07)"

patterns-established:
  - "Replace-confirmation: bufio.Scanner reads from os.Stdin; strings.EqualFold handles 'y' and 'yes'"
  - "runDeploy() uses WalkFiles() before Upload() to get file count for success message without double-walking"

requirements-completed: [DEPLOY-02, DEPLOY-03, FILES-01, FILES-02, FILES-03]

# Metrics
duration: 25min
completed: 2026-05-14
---

# Phase 3 Plan 03: Wire deploy path in main.go Summary

**Full deploy path assembled: --exclude/--force flags, replace-confirmation prompt, /tmp staging with interactive sudo fallback, and SFTP Upload() wired in main.go — verified end-to-end against real SSH host at 192.168.1.99**

## Performance

- **Duration:** 25 min
- **Started:** 2026-05-14T17:10:00Z
- **Completed:** 2026-05-14T17:35:00Z
- **Tasks:** 1 auto task + 1 checkpoint (human-verify)
- **Files modified:** 2 (main.go, upload.go)

## Accomplishments

- main.go: `--exclude` (repeatable StringArrayVar) and `--force` BoolVar registered on cobra command
- main.go: `runDeploy()` implements Resolve() -> Dial() -> SSH exists-check -> replace-confirmation prompt -> WalkFiles() -> Upload() -> success message with file count
- main.go: `runDryRun()` updated to 6-arg Resolve() signature; dry-run behavior unchanged
- upload.go: Staging strategy changed from sibling-of-target to `/tmp/docker-deploy-<unix_ts>` with interactive sudo fallback for target directory creation
- upload.go: Graceful staged-file recovery path with exact manual recovery commands when sudo fails
- Human checkpoint: All 5 verification checks passed against real SSH host (192.168.1.99)

## Task Commits

Each task was committed atomically:

1. **Task 1: Wire deploy RunE with --exclude, --force flags and Upload()** - `2a8e15a` (feat)
2. **Deviation 1: Improve staging permission error with actionable hint** - `6e8ae2b` (fix)
3. **Deviation 2: Stage uploads in remote /tmp with sudo-fallback target creation** - `77e12a1` (feat)
4. **Deviation 3: /tmp staging with interactive sudo fallback for target dir creation** - `d750f03` (feat)

## Files Created/Modified

- `cmd/docker-deploy/main.go` - Added --exclude/--force flags; implemented runDeploy() with full deploy sequence
- `internal/filetransfer/upload.go` - Changed staging to /tmp, added interactive sudo fallback, graceful recovery path

## Decisions Made

- Staging directory moved from `/opt/<project>/.deploy-tmp-<ts>` to `/tmp/docker-deploy-<ts>`: /tmp is always writable by the SSH user; the permission challenge is target directory creation, not staging
- Interactive sudo with term.ReadPassword: password entered on local terminal, piped via SSH with sudo -S; 3 attempts match familiar sudo UX
- Graceful failure path: when target dir cannot be created even with sudo, leave staged files in /tmp and print manual recovery commands — operator can always recover without re-uploading

## Deviations from Plan

### Auto-fixed Issues

**1. [Rule 1 - Bug] Improved staging permission error message with actionable hint**
- **Found during:** Task 1 (deploy wiring)
- **Issue:** Original error "creating staging directory: permission denied" gave no guidance on how to fix
- **Fix:** Added hint message pointing to `sudo mkdir -p <path> && sudo chown <user> <path>` or using `--path` flag to choose a writable target
- **Files modified:** internal/filetransfer/upload.go
- **Verification:** Error message tested manually
- **Committed in:** 6e8ae2b

**2. [Rule 1 - Bug] Changed staging to remote /tmp with sudo fallback**
- **Found during:** Deploy path wiring — verified against real SSH host
- **Issue:** Staging sibling-of-target (`/opt/<project>/.deploy-tmp-<ts>`) requires write permission to the parent `/opt` directory, which is owned by root. The SSH user typically lacks this permission even when they own `/opt/<project>`. Staging in /tmp avoids the bootstrap permission problem entirely.
- **Fix:** Changed `stagingDir` derivation from `filepath.Dir(remoteBase) + "/.deploy-tmp-" + ts` to `"/tmp/docker-deploy-" + ts`. Added `mkdir -p remoteBase` step (without sudo first, sudo fallback second) before the atomic mv.
- **Files modified:** internal/filetransfer/upload.go
- **Verification:** go build ./... exits 0; go test ./... passes; deploy tested against real host
- **Committed in:** 77e12a1

**3. [Rule 2 - Missing Critical] Interactive sudo with up to 3 password attempts and graceful fallback**
- **Found during:** Post-staging target-dir creation
- **Issue:** Bare `mkdir -p remoteBase` fails for fresh VPS where `/opt/<project>` does not exist and is not pre-created. A single-attempt sudo with no retry and no recovery path leaves the deploy in an unclear state.
- **Fix:** (a) First try `mkdir -p` without sudo. (b) On failure: prompt for password via `term.ReadPassword`, pipe to `sudo -S -p ''`, retry up to 3 times. (c) On sudo exhaustion: print exact `ssh host 'sudo mv <staging> <target>'` and `sudo mkdir/chown` recovery commands, then return error — staged files remain in /tmp for manual recovery.
- **Files modified:** internal/filetransfer/upload.go
- **Verification:** Interactive password path tested manually; graceful fallback message confirmed
- **Committed in:** d750f03

---

**Total deviations:** 3 auto-fixed (1 actionable-error message, 1 staging strategy change, 1 missing critical recovery path)
**Impact on plan:** All three deviations necessary for correct operation against real VPS. Staging strategy change is a correctness fix — the original plan assumed /opt staging was feasible, which it is not for typical non-root SSH users. No scope creep.

## Issues Encountered

The original plan specified staging as a sibling directory of the target (inside /opt). Testing against a real SSH host revealed that the SSH deploy user does not have write permission to /opt, making sibling-staging impossible without sudo. Moving staging to /tmp solved the write-permission problem cleanly, shifting the permission challenge solely to target directory creation (which sudo handles).

## User Setup Required

None - no external service configuration required.

## Known Stubs

None — deploy path is fully functional end-to-end.

## Threat Surface Scan

T-03-07: Replace-confirmation default No enforced — `bufio.Scanner` reads a line; only explicit "y" or "yes" (case-insensitive) proceeds. Accidental Enter cancels without error.

T-03-09: SFTP upload wraps the same authenticated `*gossh.Client` from `Dial()`. knownhosts verification already enforced in Phase 2. No new trust surface.

No threat surface beyond what the plan's threat model covers.

## Next Phase Readiness

Phase 3 (File Copy) is complete. All 5 success criteria verified end-to-end against a real SSH host:
1. Files uploaded (non-excluded) to remote /opt/<project>
2. .git/ excluded (default excludes); .env uploaded
3. Atomic staging via /tmp with three-step swap on repeat deploy
4. --exclude extends defaults; --force skips confirmation
5. Human checkpoint approved

Phase 4 (Core Deploy Loop) can proceed: the SSH client and file copy infrastructure are ready; Phase 4 adds `docker compose up -d` execution and output streaming.

## Self-Check

- `cmd/docker-deploy/main.go` — exists
- `internal/filetransfer/upload.go` — exists
- Commits 2a8e15a, 6e8ae2b, 77e12a1, d750f03 — present in git log
- `go build ./...` — passes
- `go test ./...` — passes (13 filetransfer tests + config tests)

## Self-Check: PASSED

---
*Phase: 03-file-copy*
*Completed: 2026-05-14*
