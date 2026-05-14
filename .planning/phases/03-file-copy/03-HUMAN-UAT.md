---
status: partial
phase: 03-file-copy
source: [03-VERIFICATION.md]
started: 2026-05-14T00:00:00Z
updated: 2026-05-14T00:00:00Z
---

## Current Test

[awaiting human testing]

## Tests

### 1. First deploy (target absent)
expected: files upload, .git/ absent, .env present, success message printed
result: [pending]

### 2. Repeat deploy — confirmation prompt
expected: "Target exists. Replace all contents? [y/N]" shown, Enter cancels
result: [pending]

### 3. --force skips prompt
expected: no prompt, deploy proceeds immediately
result: [pending]

### 4. Executable script permissions (WR-01 scope check)
expected: determine if +x scripts lose execute bit after upload (known issue per WR-01)
result: [pending]

## Summary

total: 4
passed: 0
issues: 0
pending: 4
skipped: 0
blocked: 0

## Gaps
