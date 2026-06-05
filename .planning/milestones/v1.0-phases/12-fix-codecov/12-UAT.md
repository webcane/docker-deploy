---
status: complete
phase: 12-fix-codecov
source: 12-01-SUMMARY.md, 12-02-SUMMARY.md, 12-03-SUMMARY.md, 12-04-SUMMARY.md
started: 2026-05-25T00:00:00Z
updated: 2026-05-25T11:55:00Z
---

## Current Test

<!-- OVERWRITE each test - shows where we are -->

[testing complete]

## Tests

### 1. Cold Start Smoke Test
expected: Kill any running server/service. Clear ephemeral state. Build the binary from scratch with `go build ./cmd/docker-deploy/`. Binary compiles without errors. Running `./docker-deploy --help` (or `docker deploy --help` if installed) returns a help screen without crashing.
result: pass

### 2. Plugin Description Uses "remote host"
expected: Running `docker deploy --help` (or `./docker-deploy docker-cli-plugin-metadata` for raw metadata) shows "remote host" — not "remote VPS" — in the plugin description and help text.
result: pass

### 3. README Installation Section Simplified
expected: Opening README.md, the Installation section shows only the install script (two curl commands) plus a link to INSTALL.md. No Homebrew block, no manual binary block, no go install block directly in README.
result: pass

### 4. INSTALL.md Exists with All Four Methods
expected: INSTALL.md exists at the repo root. It contains four sections: Install script, Homebrew, Manual binary, and go install — each as a flat `##` heading (no "Option N:" prefix). All original install instructions are present.
result: pass

### 5. COMPARISON.md Feedback Section
expected: At the bottom of COMPARISON.md, after the "When NOT to use docker-deploy" section, there is a "Missing a tool?" section containing a link to the GitHub Issues page so users can suggest additions to the comparison table.
result: pass

## Summary

total: 5
passed: 5
issues: 0
pending: 0
skipped: 0
blocked: 0

## Gaps

[none yet]
