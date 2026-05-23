---
status: partial
phase: 09-documentation
source: [09-VERIFICATION.md]
started: 2026-05-23T00:00:00Z
updated: 2026-05-23T00:00:00Z
---

## Current Test

[awaiting human testing — requires a live tagged GitHub Release]

## Tests

### 1. Homebrew Tap Formula (SC-09-3)
expected: `brew tap webcane/docker-deploy && brew install docker-deploy` installs the binary; after symlinking to ~/.docker/cli-plugins/, `docker deploy --help` shows plugin help
result: [pending]

### 2. End-to-End Install Smoke Test (SC-09-4)
expected: All three install methods produce a working `docker deploy --help` after install
- Homebrew: `brew tap webcane/docker-deploy && brew install docker-deploy`
- Script: `curl https://raw.githubusercontent.com/webcane/docker-deploy/main/install.sh | sh`
- Manual: download archive from GitHub Releases, extract, move to `~/.docker/cli-plugins/`
result: [pending — requires pushing a `v*` tag to trigger the release workflow]

## Summary

total: 2
passed: 0
issues: 0
pending: 2
skipped: 0
blocked: 0

## Gaps
