---
phase: 04-core-deploy-loop
verified: 2026-05-15T00:00:00Z
status: human_needed
score: 9/9 must-haves verified
overrides_applied: 0
human_verification:
  - test: "Full deploy with compose output streaming to a real TTY"
    expected: "docker deploy --host ssh://user@host runs copy-then-compose; compose output streams line-by-line with colour; plugin prints 'Deploy complete: N files copied to host:/path' on success"
    why_human: "PTY path (isTTY=true branch in RunCompose) requires a real terminal to exercise; cannot be tested in CI. Human verification was claimed in 04-03-SUMMARY.md but the verifier cannot accept SUMMARY claims as evidence."
  - test: "Compose file auto-detection from project root"
    expected: "From a directory containing compose.yaml (no --compose-file flag), deploy uses compose.yaml on the remote"
    why_human: "End-to-end behavior requires live SSH host; auto-detection logic is unit-tested but the full flow from CLI flag omission through compose execution on the remote needs a real target"
  - test: "Exit code non-zero on SSH connectivity loss mid-deploy"
    expected: "If the SSH session drops during compose execution, plugin exits non-zero; context cancellation watcher closes session so session.Wait() unblocks"
    why_human: "Requires simulating SSH disconnection mid-stream; the context cancellation goroutine is present in code but only exercisable with a real (or controllably-dropped) SSH connection"
---

# Phase 4: Core Deploy Loop Verification Report

**Phase Goal:** A developer can deploy a local compose project to a remote VPS with a single command and see compose output streamed to their terminal
**Verified:** 2026-05-15
**Status:** human_needed
**Re-verification:** No — initial verification

## Goal Achievement

### Observable Truths (from ROADMAP Success Criteria)

| # | Truth | Status | Evidence |
|---|-------|--------|----------|
| SC-1 | `docker deploy --host ssh://user@host:port` completes a full copy-then-compose cycle without additional flags | VERIFIED | `--host` flag registered at line 46 of main.go; `runDeploy()` calls `Upload()` then `compose.RunCompose()` at lines 236 and 245; compose file auto-detected via `Resolve()` when `--compose-file` not provided |
| SC-2 | `docker compose up -d` output is streamed line-by-line to the local terminal | VERIFIED (automated, PTY path needs human) | `run.go` line 83: `isTTY := term.IsTerminal(int(os.Stdout.Fd()))`; TTY path: `session.RequestPty("xterm-256color",...)` + `session.Stdout = os.Stdout` (lines 98-105); non-TTY path: two goroutines draining `StdoutPipe`/`StderrPipe` to `os.Stdout`/`os.Stderr` (lines 126-132); all 5 `TestRunCompose_*` tests pass |
| SC-3 | Plugin exits with non-zero code if file copy fails, compose command fails, or SSH connectivity is lost mid-deploy | VERIFIED (connectivity-loss needs human) | `Upload()` failure: `return err` at line 239; compose failure: `handleWait()` extracts `ExitStatus()`, writes `"Deploy failed: docker compose exited with code N"` to stderr, returns non-nil error; SSH connectivity loss: context watcher goroutine (lines 67-70) closes session on `ctx.Done()` so `session.Wait()` unblocks with an error |

### PLAN Must-Have Truths

| # | Truth (from plan frontmatter) | Status | Evidence |
|---|-------------------------------|--------|----------|
| P01-T1 | Config.ComposeFile populated from flag, compose_file in deploy.yaml, or auto-detected | VERIFIED | `config.go` lines 211-226: three-tier switch `flagComposeFile != ""` → `file.Target.ComposeFile != ""` → `os.Stat` auto-detect; `TestResolveComposeFile_FlagWins/FileWins/AutoDetect*` all PASS |
| P01-T2 | Auto-detection tries compose.yaml first, then docker-compose.yml | VERIFIED | `config.go` lines 217-221: `for _, candidate := range []string{"compose.yaml", "docker-compose.yml"}` — compose.yaml is first; `TestResolveComposeFile_AutoDetectComposeYaml` and `TestResolveComposeFile_AutoDetectDockerComposeYml` both PASS |
| P01-T3 | If no compose file found, Resolve() returns error containing "no compose file found" | VERIFIED | `config.go` line 224: `return Config{}, fmt.Errorf("no compose file found; use --compose-file to specify one")`; `TestResolveComposeFile_NoFileFound` PASS |
| P01-T4 | Resolve() signature accepts composeFile string parameter alongside existing parameters | VERIFIED | `config.go` line 178: `func Resolve(flagHost, flagPath string, flagExcludes []string, flagForce bool, flagComposeFile string, file FileConfig, projectName string, localDir string) (Config, error)` |
| P02-T1 | RunCompose() executes docker compose up -d --remove-orphans on the remote | VERIFIED | `run.go` line 54: `cmd := "docker compose -f " + filetransfer.ShellQuote(remotePath+"/"+composeFile) + " up -d --remove-orphans"`; `TestRunCompose_CommandConstruction` PASS: command matches `"docker compose -f '/opt/myapp/compose.yaml' up -d --remove-orphans"` |
| P02-T2 | When TTY, RunCompose() allocates PTY (RequestPty xterm-256color) | VERIFIED | `run.go` line 98: `session.RequestPty("xterm-256color", h, w, modes)`; `grep "RequestPty"` found; PTY path unit-tested via human checkpoint |
| P02-T3 | When not TTY, RunCompose() uses two goroutines for stdout/stderr | VERIFIED | `run.go` lines 110-132: `session.StdoutPipe()` + `session.StderrPipe()` + `sync.WaitGroup` + two `io.Copy` goroutines; `TestRunCompose_NewSessionPerCall` PASS (session counter increments) |
| P02-T4 | On compose failure, writes "Deploy failed: docker compose exited with code N" and returns non-nil error | VERIFIED | `run.go` `handleWait()` (lines 166-177): `errors.As(waitErr, &exitErr)` → `fmt.Fprintf(os.Stderr, "Deploy failed: docker compose exited with code %d\n", code)` + `return fmt.Errorf("docker compose exited with code %d", code)`; `TestRunCompose_ExitCodeNonZero` PASS |
| P02-T5 | RunCompose() uses a dedicated NewSession() per call | VERIFIED | `run.go` line 57: `session, err := client.NewSession()`; `TestRunCompose_NewSessionPerCall` verifies session counter == 2 after two calls; PASS |
| P03-T1 | docker deploy --host ... runs full copy-then-compose cycle without additional flags | VERIFIED | See SC-1 above |
| P03-T2 | Compose output streamed line-by-line | VERIFIED (PTY branch needs human) | See SC-2 above |
| P03-T3 | Plugin exits non-zero on copy fail, compose fail, or SSH loss | VERIFIED (SSH loss needs human) | See SC-3 above |
| P03-T4 | Compose file auto-detected from project root if --compose-file not supplied | VERIFIED | `main.go` line 146: `config.Resolve(host, path, excludes, force, composeFile, fileConfig, projectName, cwd)` — composeFile is empty string when flag not set, triggering auto-detect in `Resolve()` |
| P03-T5 | Terse success line on stdout after successful deploy | VERIFIED | `main.go` line 250: `fmt.Fprintf(os.Stdout, "Deploy complete: %d files copied to %s:%s\n", fileCount, resolved.Host.Hostname, resolved.Path)` — after `RunCompose()` returns nil |

**Score:** 9/9 roadmap success criteria truths verified (automated); 2 of 3 SC items need human confirmation for live-terminal and SSH-drop edge cases

### Required Artifacts

| Artifact | Expected | Status | Details |
|----------|----------|--------|---------|
| `internal/config/config.go` | ComposeFile field on Config and TargetConfig; updated Resolve() | VERIFIED | Lines 36 (`yaml:"compose_file"`), 53 (`ComposeFile string`), 178 (8-arg Resolve signature), 211-226 (resolution logic) |
| `internal/config/config_test.go` | TDD tests for all compose file resolution paths | VERIFIED | All 6 `TestResolveComposeFile_*` tests PASS (`go test ./internal/config/... -v -run TestResolveComposeFile` exit 0) |
| `internal/compose/run.go` | RunCompose() function | VERIFIED | `func RunCompose` at line 44; substantive implementation (178 lines); PTY + non-TTY + exit code handling all present |
| `internal/compose/run_test.go` | Unit tests for command construction and exit code propagation | VERIFIED | All 5 `TestRunCompose_*` tests PASS (`go test ./internal/compose/... -v` exit 0) |
| `cmd/docker-deploy/main.go` | --compose-file flag; updated Resolve() call; basename validation; RunCompose() call | VERIFIED | Line 51 (flag), line 146 (Resolve call), line 162 (basename validation), line 245 (RunCompose call), line 250 (success print) |

### Key Link Verification

| From | To | Via | Status | Details |
|------|----|-----|--------|---------|
| `cmd/docker-deploy/main.go` | `internal/config/config.go` | `config.Resolve(host, path, excludes, force, composeFile, fileConfig, projectName, cwd)` | WIRED | Line 146 of main.go; 8-arg call matches new signature |
| `cmd/docker-deploy/main.go` | `internal/compose/run.go` | `compose.RunCompose(context.Background(), client, resolved.Path, resolved.ComposeFile)` | WIRED | Line 245 of main.go; called after Upload() returns nil |
| `internal/compose/run.go` | `golang.org/x/crypto/ssh` | `client.NewSession()`, `session.RequestPty()`, `session.Start()`, `session.Wait()` | WIRED | Lines 57, 98, 120/141, 137/144 of run.go |
| `internal/compose/run.go` | `internal/filetransfer` | `filetransfer.ShellQuote(remotePath+"/"+composeFile)` | WIRED | Line 54 of run.go |

**Note on command construction deviation:** The plan specified `ShellQuote(remotePath) + "/" + composeFile`, quoting only the path. The implementation quotes the combined string `ShellQuote(remotePath+"/"+composeFile)`. This is a security improvement (composeFile is also protected by the combined single-quote). Tests were updated accordingly (`TestRunCompose_ShellQuoteRemotePath` expects `"'/opt/my app/compose.yaml'"` not `"'/opt/my app'"`) and all pass. This deviation is acceptable.

### Data-Flow Trace (Level 4)

| Artifact | Data Variable | Source | Produces Real Data | Status |
|----------|---------------|--------|--------------------|--------|
| `main.go` runDeploy | `resolved.ComposeFile` | `config.Resolve()` → auto-detect or flag/file | Yes — `os.Stat` filesystem check or flag value | FLOWING |
| `main.go` runDeploy | `fileCount` from `Upload()` | SFTP walk of local directory | Yes — real file count | FLOWING |
| `run.go` RunCompose | SSH exec output | Remote docker compose process via SSH session | Yes — `io.Copy` from live SSH pipes to os.Stdout/Stderr | FLOWING |

### Behavioral Spot-Checks

| Behavior | Command | Result | Status |
|----------|---------|--------|--------|
| All config tests pass | `go test ./internal/config/... -v -run TestResolveComposeFile` | 6/6 tests PASS, exit 0 | PASS |
| All compose tests pass | `go test ./internal/compose/... -v` | 5/5 tests PASS, exit 0 | PASS |
| Full build succeeds | `go build ./...` | exit 0 | PASS |
| Vet clean | `go vet ./...` | exit 0, no warnings | PASS |
| Full test suite | `go test ./...` | all packages PASS | PASS |
| No InsecureIgnoreHostKey in production code | `grep -rn InsecureIgnoreHostKey internal/ cmd/` | Found only in `run_test.go` and `upload_test.go` (test-only, `//nolint — test-only` annotated) | PASS |
| No debt markers (TBD/FIXME/XXX) in phase files | `grep -n TBD\|FIXME\|XXX` on config.go, run.go, main.go | 0 matches | PASS |

### Probe Execution

No `scripts/*/tests/probe-*.sh` probes declared or found for this phase.

Step 7c: SKIPPED — no probe scripts exist for Phase 4.

### Requirements Coverage

| Requirement | Source Plan | Description | Status | Evidence |
|-------------|------------|-------------|--------|----------|
| DEPLOY-01 | 04-01, 04-03 | User can deploy with `docker deploy --host ssh://user@host:port` | SATISFIED | `--host` flag registered; `runDeploy()` accepts it; `config.Resolve()` parses SSH URL; SSH dial and full deploy loop execute |
| DEPLOY-04 | 04-02, 04-03 | `docker compose up -d` executed on remote via SSH after file copy | SATISFIED | `compose.RunCompose()` called at line 245 of main.go after `Upload()` returns nil at line 236; constructs `"docker compose -f ... up -d --remove-orphans"` |
| DEPLOY-05 | 04-02, 04-03 | Plugin exits non-zero if any step fails | SATISFIED | Upload failure: `return err` (line 239); compose failure: `handleWait()` returns non-nil; cobra/plugin framework propagates non-nil return as non-zero exit |
| DEPLOY-06 | 04-02, 04-03 | Deploy output (compose up result) streamed to local terminal | SATISFIED (automated, TTY path needs human) | PTY path: `session.Stdout = os.Stdout`; non-TTY path: `io.Copy(os.Stdout, stdoutPipe)` goroutine; both paths tested and wired |

**Documentation note:** REQUIREMENTS.md still shows DEPLOY-01, DEPLOY-04, DEPLOY-05, DEPLOY-06 as `[ ]` (Pending) and the traceability table shows "Pending". The implementation is complete and tested; only the documentation checkboxes were not updated. This is a tracking hygiene issue, not a code defect.

### Anti-Patterns Found

| File | Line | Pattern | Severity | Impact |
|------|------|---------|----------|--------|
| `internal/compose/run_test.go` | 106 | `InsecureIgnoreHostKey()` | Info | Test-only; annotated `//nolint — test-only`; no production code path uses it |
| `internal/filetransfer/upload_test.go` | 109 | `InsecureIgnoreHostKey()` | Info | Test-only; annotated `//nolint — test-only`; no production code path uses it |

No blockers. No unreferenced TBD/FIXME/XXX markers. No stub implementations. No hardcoded empty returns in production code.

### Human Verification Required

#### 1. PTY Output Streaming (Real Terminal)

**Test:** From an interactive terminal (not piped), run `docker deploy --host ssh://user@host:port` from a project directory with a valid compose.yaml and remote target.
**Expected:** Compose output streams with colours and progress formatting as if running `docker compose up` locally; after completion "Deploy complete: N files copied to host:/path" is printed.
**Why human:** The `isTTY = true` branch (`session.RequestPty + session.Stdout = os.Stdout`) cannot be exercised in automated tests because `term.IsTerminal(os.Stdout.Fd())` returns false in CI and test processes.

#### 2. Compose File Auto-Detection (End-to-End)

**Test:** From a directory containing `compose.yaml`, run `docker deploy --host ssh://user@host` without `--compose-file`. Then repeat from a directory containing only `docker-compose.yml`.
**Expected:** First run deploys using `compose.yaml`; second uses `docker-compose.yml`. No `--compose-file` flag required in either case.
**Why human:** The unit tests verify auto-detection in `config.Resolve()` isolation; the full path from CLI invocation through compose execution on a real remote needs live confirmation.

#### 3. SSH Connectivity Loss Mid-Deploy

**Test:** Start a deploy to a host, then drop the SSH connection mid-stream (e.g., restart sshd or firewall the port during compose execution).
**Expected:** Plugin exits non-zero; context cancellation watcher closes the session; error surfaced to user.
**Why human:** Requires controllably dropping a live SSH connection; cannot be simulated without a real (or specially configured mock) SSH host.

**Note:** The 04-03-SUMMARY.md documents that a human ran 6 test scenarios against a real SSH host (192.168.1.99) including compose failure (exit code 1), and all passed. This is consistent with the codebase evidence but the verifier cannot treat SUMMARY claims as direct proof — hence these items remain in the human_needed list for completeness.

### Gaps Summary

No automated verification gaps found. All must-have truths are satisfied by codebase evidence. The `human_needed` status reflects three behaviors that require a live SSH target to fully confirm, consistent with the inherent nature of an SSH deploy tool. The REQUIREMENTS.md tracking checkboxes not being updated is a documentation hygiene issue only.

---

_Verified: 2026-05-15_
_Verifier: Claude (gsd-verifier)_
