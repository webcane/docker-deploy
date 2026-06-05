---
phase: 09-documentation
plan: "02"
subsystem: distribution
tags: [install, shell, sha256, cosign, posix]
dependency_graph:
  requires: []
  provides: [install.sh]
  affects: []
tech_stack:
  added: []
  patterns: [curl-pipe-sh, posix-sh, sha256-verification, cosign-keyless]
key_files:
  created:
    - install.sh
  modified: []
decisions:
  - "POSIX sh used (#!/bin/sh, no bashisms) for maximum cross-platform compatibility"
  - "SHA256 always enforced via sha256sum or shasum fallback — hard abort on mismatch"
  - "cosign is optional: prints informational message and continues when absent, aborts on cosign verification failure when present"
  - "Temp dir cleaned via trap EXIT — ensures cleanup even if script aborts mid-download"
  - "INSTALL_VERSION defaults to GitHub API latest-release tag_name via sed extraction"
metrics:
  duration: "74s"
  completed_date: "2026-05-22T16:57:13Z"
  tasks_completed: 1
  tasks_total: 1
---

# Phase 09 Plan 02: install.sh One-Liner Installer Summary

**One-liner:** POSIX sh installer that downloads the correct docker-deploy binary from GitHub Releases, verifies SHA256, optionally runs cosign, and places the binary in `~/.docker/cli-plugins/docker-deploy`.

## Tasks Completed

| Task | Name | Commit | Files |
|------|------|--------|-------|
| 1 | Create install.sh with OS/arch detection, SHA256 verification, cosign fallback | 11b084e | install.sh |

## What Was Built

`install.sh` at the repo root enables the curl-pipe-sh one-liner install pattern. Key behaviors:

- **OS/arch detection** — `uname -s` maps to `linux`/`darwin`; `uname -m` maps to `amd64`/`arm64`; unsupported values cause an immediate `exit 1` with a descriptive message.
- **Version pinning** — `INSTALL_VERSION` env var overrides the default (latest release fetched from GitHub API).
- **Download** — archive and `checksums.txt` fetched from the matching GitHub Release URL into a `mktemp -d` temporary directory.
- **SHA256 verification (always enforced)** — uses `sha256sum` when available, falls back to `shasum -a 256`; hard-aborts with "ERROR: SHA256 checksum mismatch — aborting" on failure.
- **cosign verification (graceful fallback)** — when cosign is installed, downloads `checksums.txt.sig` and `checksums.txt.pem` and runs `cosign verify-blob`; when cosign is absent, prints exactly: "cosign not found — skipping signature verification, checking SHA256 only" and continues (per D-14).
- **Install** — extracts binary from archive, creates `~/.docker/cli-plugins/` if needed, moves binary with `chmod +x`.
- **Upgrade messaging** — prints "Updated docker-deploy vOLD → vNEW" on upgrade, "Installed docker-deploy vX to ~/.docker/cli-plugins/docker-deploy" on first install (per D-13).
- **Cleanup** — `trap 'rm -rf "$TMPDIR"' EXIT` ensures temp dir removal on any exit path.

## Verification Results

```
sh -n install.sh  → syntax OK
grep -c 'SHA256\|sha256\|checksums'  → 15
grep -c 'cosign'  → 5
grep -c 'uname'   → 2
grep -c 'INSTALL_VERSION'  → 9
grep -c '\.docker/cli-plugins'  → 1
head -1 install.sh  → #!/bin/sh
```

All 12 acceptance criteria: PASS.

## Deviations from Plan

None — plan executed exactly as written.

## Known Stubs

None — install.sh is a complete implementation. No hardcoded empty values or placeholders. The GitHub Releases artifacts (binaries, `checksums.txt`, `checksums.txt.sig`, `checksums.txt.pem`) are produced by the GoReleaser workflow (plan 09-01); install.sh is ready to work against them once a release is tagged.

## Threat Flags

None — all threat mitigations match the plan's threat model. T-09-02-01 (binary tampering) is mitigated by SHA256 always-verified + cosign optional. No new trust boundaries introduced beyond those in the plan.

## Self-Check

- FOUND: install.sh
- FOUND: 09-02-SUMMARY.md
- FOUND commit: 11b084e
- No STATE.md or ROADMAP.md modifications: confirmed
- Self-Check: PASSED
