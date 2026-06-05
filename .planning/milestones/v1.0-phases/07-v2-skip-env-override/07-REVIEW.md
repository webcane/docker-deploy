---
phase: 07-v2-skip-env-override
reviewed: 2026-05-21T00:00:00Z
depth: standard
files_reviewed: 8
files_reviewed_list:
  - cmd/docker-deploy/main.go
  - cmd/docker-deploy/main_test.go
  - internal/compose/run.go
  - internal/compose/run_test.go
  - internal/config/config.go
  - internal/config/config_test.go
  - internal/filetransfer/upload.go
  - internal/filetransfer/upload_test.go
findings:
  critical: 0
  warning: 3
  info: 2
  total: 5
status: issues_found
---

# Phase 7: Code Review Report

**Reviewed:** 2026-05-21T00:00:00Z
**Depth:** standard
**Files Reviewed:** 8
**Status:** issues_found

## Summary

This phase delivers `--skip-env` and `--verbose` flags end-to-end, including `.env` backup/restore in the atomic-swap path and verbose SSH-command logging throughout the deploy pipeline. The core logic is sound: config resolution correctly OR-merges `SkipEnv`, `mergeExcludes` deduplicates `.env` reliably, and the three-step atomic swap handles backup/restore on all error paths in the `existsBefore == true` branch. Tests are well-structured with an in-process mock SSH server.

Three warnings and two info items were found. No blockers.

## Warnings

### WR-01: Dead function `sshExecVerbose` — verbose helper never called

**File:** `internal/filetransfer/upload.go:48`
**Issue:** `sshExecVerbose` is defined and documented as the intended abstraction for combining `sshExec` + verbose logging, but it is never called anywhere in the codebase. All verbose SSH-command logging in `Upload()` is instead done inline with ad-hoc `if verbose { fmt.Fprintf(...) }` guards surrounding direct `sshExec` calls. The function is unreachable dead code. This means a future caller may use `sshExecVerbose` believing it matches the behavior of the inline verbose blocks — but the inline blocks (e.g. `sudoRunWithFallback`) log the verbose output even when the command *fails*, while `sshExecVerbose` logs only on the error branch via `errors.As`. The two patterns are slightly inconsistent, making the dead function a latent correctness trap.
**Fix:** Either delete `sshExecVerbose` and leave the inline patterns, or replace all the inline `if verbose` + `sshExec` pairs in `Upload()` / `sudoRunWithFallback` with `sshExecVerbose` calls.

---

### WR-02: Verbose mode does not log `→ exit N` when `tryDirectCopy` or `tryPasswordlessSudo` fails

**File:** `internal/filetransfer/upload.go:240-258`
**Issue:** The `sudoRunWithFallback` closure logs `[ssh] <cmd>` before attempting each step and `→ exit 0` when a step succeeds, but it logs nothing when a step fails. When both step 1 (direct copy) and step 2 (passwordless sudo) fail and the code falls through to the interactive password prompt, a `--verbose` user sees:

```
[ssh] mkdir -p '/opt/myapp'
[ssh] sudo -n sh -c 'mkdir -p '\''/opt/myapp'\'''
WARNING: passwordless sudo not configured...
```

There is no `→ exit N` after either of the failed attempts. This is inconsistent with the `sshExecVerbose` contract (which does log the failure code) and with compose/run.go's verbose logging (line 166-168), which always logs the exit code. An operator debugging a failed deploy in verbose mode gets incomplete information.
**Fix:** After the `if ok { return nil }` guards in steps 1 and 2, add an `else if verbose` branch that logs `→ exit N` (or at minimum `→ failed`):
```go
// After tryDirectCopy
if ok {
    if verbose { fmt.Fprintf(os.Stderr, "  → exit 0\n") }
    return nil
}
if verbose {
    fmt.Fprintf(os.Stderr, "  → exit 1 (direct copy failed, trying sudo)\n")
}
```

---

### WR-03: `remoteExists` verbose logging always emits `→ exit 0` regardless of actual result

**File:** `internal/filetransfer/upload.go:199-207`
**Issue:** The verbose block around `remoteExists` unconditionally prints `→ exit 0` when `err == nil`, even when the directory is absent (exit code from `test -d` is 1 in that case). The condition `if verbose && err == nil` treats a successful SSH command that returns "absent" as `exit 0`, but `test -d` exits with code 1 when the path does not exist. The `sshExecOutput` function swallows the exit code by reading stdout only (via `session.Output`), so `err` is nil even when the test fails. The verbose log is therefore misleading: it always says `exit 0` on a first deploy where the target does not exist.

```go
existsBefore, err := remoteExists(client, remoteBase)
if verbose && err == nil {
    fmt.Fprintf(os.Stderr, "  → exit 0\n")   // BUG: says "exit 0" even when test -d returned 1
}
```

This is a logging accuracy issue, not a functional bug (the `existsBefore` bool is computed correctly from the stdout output). However it gives a false signal to operators and could mislead debugging.
**Fix:** Replace the unconditional `exit 0` with a result-aware log:
```go
if verbose && err == nil {
    if existsBefore {
        fmt.Fprintf(os.Stderr, "  → exists\n")
    } else {
        fmt.Fprintf(os.Stderr, "  → absent\n")
    }
}
```

---

## Info

### IN-01: `sudoPw` populated in `sudoRunWithFallback` but never reused across calls

**File:** `internal/filetransfer/upload.go:282-283` and `cmd/docker-deploy/main.go:307-311`
**Issue:** `sudoPw` is passed into `Upload()` and stored when a password is successfully used (`*sudoPw = pw` at line 283). The comment in `main.go` says it is "reused across operations to avoid prompting multiple times." However, inside `sudoRunWithFallback`, the stored `*sudoPw` is never checked before re-prompting. Each invocation of `sudoRunWithFallback` always falls through to the interactive password prompt if direct copy and passwordless sudo both fail — it does not first try `*sudoPw` from a prior successful attempt. This means a deploy that requires password-sudo for `mkdir`, `mv`, and `rm -rf` will prompt the user three separate times instead of once.
**Fix:** At the top of step 3 in `sudoRunWithFallback`, attempt the cached password before prompting:
```go
// Try cached password before prompting.
if *sudoPw != "" {
    sudoCmd := fmt.Sprintf("echo %s | sudo -S -p '' sh -c %s", ShellQuote(*sudoPw), ShellQuote(cmd))
    if sshExec(client, sudoCmd) == nil {
        return nil
    }
}
// Fall through to interactive prompt...
```

---

### IN-02: Dry-run passes hardcoded sentinel `"docker-compose.yml"` for `ComposeFile`, masking auto-detect errors

**File:** `cmd/docker-deploy/main.go:105`
**Issue:** `runDryRun` calls `config.Resolve` with `ComposeFile: "docker-compose.yml"` as a sentinel to bypass auto-detection. The comment explains this is intentional. However, if auto-detection would fail (neither `compose.yaml` nor `docker-compose.yml` present in the working directory), the dry-run succeeds in config resolution but a subsequent real deploy would fail immediately at the same step. The dry-run is documented as "verify SSH connectivity and print resolved config" — not revealing config resolution failures reduces its diagnostic value.

This is a design trade-off, not strictly a bug, but the sentinel silently hides a class of misconfiguration that dry-run could surface.
**Fix:** Remove the sentinel and let auto-detect run normally in dry-run. If auto-detect fails, the error is surfaced early — which is exactly what dry-run is for. The comment `"value is unused in dry-run"` acknowledges that `composeFile` is not used post-resolution, so detection failure is still safe to surface.

---

_Reviewed: 2026-05-21_
_Reviewer: Claude (gsd-code-reviewer)_
_Depth: standard_
