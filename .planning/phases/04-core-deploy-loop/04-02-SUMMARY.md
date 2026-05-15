---
phase: "04"
plan: "02"
subsystem: compose
tags: [compose, tdd, ssh, pty, output-streaming, exit-code]
dependency_graph:
  requires: [internal/filetransfer (ShellQuote), golang.org/x/crypto/ssh, golang.org/x/term]
  provides: [RunCompose, internal/compose package]
  affects: [cmd/docker-deploy/main.go (Plan 03 wires RunCompose into runDeploy)]
tech_stack:
  added: [internal/compose package]
  patterns: [PTY vs pipe output routing via term.IsTerminal, sync.WaitGroup goroutine drain, errors.As ExitError extraction]
key_files:
  created:
    - internal/compose/run.go
    - internal/compose/run_test.go
  modified: []
decisions:
  - "RunCompose uses session.Start() not session.Run() per plan discretion note; allows wg.Wait() before session.Wait() in non-TTY path"
  - "Non-TTY: wg.Wait() called before session.Wait() to ensure all pipe output is flushed before checking exit status"
  - "PTY: session.Stderr set to os.Stdout (intentional — PTY merges streams per D-01)"
  - "session.Stdin intentionally not connected to os.Stdin to prevent interactive input reaching remote (T-04-02-03)"
  - "composeFile is NOT ShellQuote'd; responsibility for filepath.Base() validation is on callers per T-04-01-01 and Plan 01 SUMMARY"
metrics:
  duration: "~7 min"
  completed: "2026-05-15"
  tasks_completed: 1
  files_modified: 2
---

# Phase 4 Plan 02: RunCompose SSH Execution Summary

RunCompose() executes docker compose up on a remote host via SSH with PTY/pipe output routing and exit code propagation.

## What Was Built

New `internal/compose` package with a single exported function:

- `RunCompose(ctx context.Context, client *gossh.Client, remotePath, composeFile string) error`
- Constructs exec command: `docker compose -f '<remotePath>'/<composeFile> up -d --remove-orphans`
- Uses `filetransfer.ShellQuote()` on `remotePath` to prevent shell injection from paths with spaces or special characters (T-04-02-01)
- **PTY path** (`term.IsTerminal(os.Stdout.Fd())` true): calls `session.RequestPty("xterm-256color", h, w, modes)`, sets `session.Stdout = session.Stderr = os.Stdout` — PTY merges both streams
- **Non-TTY path** (CI/piped): two goroutines drain `session.StdoutPipe()→os.Stdout` and `session.StderrPipe()→os.Stderr`; `sync.WaitGroup` ensures both complete before `session.Wait()` returns (T-04-02-04)
- Exit code 0 → `nil`; Exit code N → `fmt.Fprintf(os.Stderr, "Deploy failed: docker compose exited with code N\n")` + non-nil error (T-04-02-05, D-12, DEPLOY-05)
- Fresh `NewSession()` per call, `defer session.Close()` — per CLAUDE.md Rule 3

## TDD Gate Compliance

RED commit: `f523f6b` — `test(04-02): add failing tests for RunCompose`
GREEN commit: `13963c8` — `feat(04-02): implement RunCompose with PTY/pipe output routing and exit code propagation`

Both gate commits present in correct order. All five `TestRunCompose_*` tests pass.

Test coverage:
- `TestRunCompose_CommandConstruction` — verifies exact exec command `"docker compose -f '/opt/myapp'/compose.yaml up -d --remove-orphans"`
- `TestRunCompose_ExitCodeZero` — nil error on exit 0
- `TestRunCompose_ExitCodeNonZero` — non-nil error containing "docker compose exited with code 1" on exit 1
- `TestRunCompose_ShellQuoteRemotePath` — command contains `"'/opt/my app'"` for path with space
- `TestRunCompose_NewSessionPerCall` — session counter increments to 2 after two sequential calls

## Deviations from Plan

None — plan executed exactly as written.

## Known Stubs

None. RunCompose is fully implemented. Plan 03 will wire it into `runDeploy()` in `cmd/docker-deploy/main.go`.

## Threat Flags

All five threats from the plan's threat register are mitigated in implementation:

| Flag | File | Description |
|------|------|-------------|
| T-04-02-01 mitigated | internal/compose/run.go | `filetransfer.ShellQuote(remotePath)` wraps remotePath in single quotes with embedded-quote escaping |
| T-04-02-02 mitigated | internal/compose/run.go | composeFile is a validated basename (callers responsible per Plan 01 threat model); not shell-quoted |
| T-04-02-03 mitigated | internal/compose/run.go | `session.Stdin` not connected to `os.Stdin`; PTY is output-only |
| T-04-02-04 mitigated | internal/compose/run.go | `sync.WaitGroup` ensures both drain goroutines complete before return |
| T-04-02-05 accepted | internal/compose/run.go | `errors.As(waitErr, &exitErr)` idiomatic gossh pattern; non-ExitError errors wrapped and returned |

## Self-Check: PASSED

- internal/compose/run.go: FOUND
- internal/compose/run_test.go: FOUND
- Commit f523f6b (RED): FOUND
- Commit 13963c8 (GREEN): FOUND
- All 5 TestRunCompose_* tests: PASS
- grep "func RunCompose" internal/compose/run.go: FOUND
- grep "RequestPty" internal/compose/run.go: FOUND
- grep "remove-orphans" internal/compose/run.go: FOUND
- InsecureIgnoreHostKey in production code: NOT FOUND (correct)
