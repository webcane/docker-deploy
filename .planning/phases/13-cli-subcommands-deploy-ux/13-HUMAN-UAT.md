---
status: partial
phase: 13-cli-subcommands-deploy-ux
source: [13-VERIFICATION.md]
started: 2026-05-26T00:00:00Z
updated: 2026-05-26T00:00:00Z
---

## Current Test

[awaiting human testing]

## Tests

### 1. SC-3 wording: untagged builds print git commit hash

**Context:** ROADMAP success criterion 3 says "untagged builds print the short git commit hash". The implementation prints:

```
Docker Deploy Version dev
  Git commit:  <hash>
  OS/Arch:     <os>/<arch>
```

The hash IS printed on line 2 (`Git commit: <hash>`), but the version field on line 1 shows `"dev"`, not the hash itself.

The plan's own spec (D-03) explicitly defines this 3-line dev format with a separate commit line.

**Question:** Does the ROADMAP SC-3 intent mean (a) the hash must appear somewhere in the output, or (b) the version field itself must show the commit hash for dev builds?

expected: Human confirms that showing the hash on the "Git commit:" line satisfies SC-3, OR requests that the version field show the commit hash for untagged builds
result: [pending]

## Summary

total: 1
passed: 0
issues: 0
pending: 1
skipped: 0
blocked: 0

## Gaps
