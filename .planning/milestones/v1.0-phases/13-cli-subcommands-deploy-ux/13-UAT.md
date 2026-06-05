---
status: resolved
phase: 13-cli-subcommands-deploy-ux
source: [13-01-SUMMARY.md, 13-02-SUMMARY.md, 13-03-SUMMARY.md, 13-04-SUMMARY.md, 13-05-SUMMARY.md, 13-06-SUMMARY.md, 13-07-SUMMARY.md]
started: 2026-05-26T00:00:00Z
updated: 2026-05-26T00:00:00Z
---

## Current Test

[testing complete]

## Tests

### 1. Cold Start Smoke Test
expected: Kill any running server/service. Clear ephemeral state (temp DBs, caches, lock files). Start the application from scratch. Binary builds without errors and basic invocation (docker deploy --help or docker deploy version) returns live output with no crash.
result: pass

### 2. Version Subcommand Output
expected: Run `docker deploy version` (or `./docker-deploy version` from a fresh build). Output shows 3 lines: "Docker Deploy Version dev", "  Git commit:  <hash>", "  OS/Arch:     <os>/<arch>". No "Built:" line in a dev/untagged build. If built with `make build`, git commit hash is populated; if built with `go build` directly it shows "unknown".
result: issue
reported: "docker deploy version shows 'Built: 2026-05-26T12:09:30Z' in a dev/untagged build. Spec says dev output omits the Built line, but make build always injects buildTime so it appears regardless of tag status."
severity: minor

### 3. Validate Subcommand - Valid Config
expected: In a directory with a valid deploy.yaml, run `docker deploy validate`. Output to stdout: "✓ deploy.yaml is valid". Exit code 0. No SSH connection is made.
result: pass

### 4. Validate Subcommand - Missing File
expected: In a directory with no deploy.yaml, run `docker deploy validate`. stderr shows "deploy.yaml not found". Exit code non-zero. No usage/help block printed by cobra.
result: pass

### 5. Verbose Pre-Confirm Diff (Repeat Deploy)
expected: On a second deploy to an existing remote directory (without --force), running `docker deploy --verbose` shows "Local files (N):" list and "Remote files (M):" list before prompting "Replace all contents? [y/N]". First-time deploy shows "Remote files: (none)" but no prompt.
result: issue
reported: "Mostly works — Local files and Remote files lists appear before the confirm prompt. However the local file list appears twice: once as '-> filename' arrows after the test -w SSH probe, and again in the 'Local files (N):' verbose diff section."
severity: minor

### 6. Force Flag Skips Confirm Prompt
expected: Running `docker deploy --force` against an existing remote deployment proceeds without showing any "Replace all contents?" confirmation prompt. Files are uploaded directly.
result: issue
reported: "Prompt is correctly skipped. But deploy fails: 'renaming existing target to backup: running mv /opt/test-deploy /opt/test-deploy-old-XXXXX: Process exited with status 1'. The mv fails because /opt is not user-writable, but the needsSudo probe returned exit 0 (path is user-writable) on test -w /opt/test-deploy — so sudo was bypassed. mv needs /opt (parent) writable, not /opt/test-deploy."
severity: major

### 7. Verbose sudo -l in Preflight
expected: When running `docker deploy --verbose` and the remote user requires sudo for the deployment directory, the preflight output includes `[sudo -l]` followed by the sudo policy output from the remote. If sudo -l fails or is not needed, nothing extra is shown (failure is silent).
result: pass

## Summary

total: 7
passed: 4
issues: 3
pending: 0
skipped: 0
blocked: 0

## Gaps

- truth: "dev/untagged build output omits 'Built:' line — only tagged builds show it"
  status: resolved
  reason: "User reported: docker deploy version shows 'Built: 2026-05-26T12:09:30Z' in a dev build. make build always injects buildTime so it appears even for untagged builds."
  severity: minor
  test: 2
  root_cause: "runVersionTo checks 'buildTime != unknown' only. make build always injects buildTime via ldflags regardless of tag. Fix: also gate on 'version != dev' so local builds show the 3-line dev format."
  artifacts:
    - path: "cmd/docker-deploy/main.go"
      issue: "runVersionTo line ~109: condition should be 'buildTime != unknown && version != dev'"
  missing:
    - "Change condition: if buildTime != \"unknown\" && version != \"dev\""

- truth: "Verbose pre-confirm diff shows file lists once each (Local files N, Remote files M) before the confirm prompt"
  status: resolved
  reason: "User reported: local file list appears twice — once as '-> filename' arrows after the test -w SSH probe, and again in the 'Local files (N):' diff section. Redundant output."
  severity: minor
  test: 5
  root_cause: "-> arrows print during the /tmp staging upload (line ~348 in upload.go), which always happens before the confirm check. The verbose diff then lists the same files again. Fix: suppress -> arrows when !force && existsBefore since the diff block covers them."
  artifacts:
    - path: "internal/filetransfer/upload.go"
      issue: "line ~348: verbose per-file arrow printed unconditionally; should be suppressed when a verbose diff will follow"
  missing:
    - "Gate the -> arrow print: if verbose && !(existsBefore check is too late — needs restructure OR suppress arrows when confirm-diff will run)"

- truth: "docker deploy --force on a path requiring sudo for mv (parent dir not user-writable) succeeds using sudo"
  status: resolved
  reason: "User reported: mv '/opt/test-deploy' to backup failed with exit status 1. needsSudo probe returned exit 0 on 'test -w /opt/test-deploy' (dir is writable) but mv needs /opt (parent) to be writable — sudo was incorrectly bypassed."
  severity: major
  test: 6
  root_cause: "probe is 'test -w remoteBase || test -w parent' — OR short-circuits on remoteBase being user-writable, so needsSudo=false even when parent (/opt) is root-owned. mv renames in the parent, not inside remoteBase. Fix: change probe to 'test -w parent' only — parent writability is what mkdir/mv/rm all require."
  artifacts:
    - path: "internal/filetransfer/upload.go"
      issue: "probeCmd line ~243: 'test -w remoteBase || test -w parent' should be 'test -w parent' only"
  missing:
    - "Change probeCmd to ShellQuote(path.Dir(remoteBase)) only — drop remoteBase check"
