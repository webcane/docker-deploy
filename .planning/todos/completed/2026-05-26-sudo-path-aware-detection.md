---
title: "Sudo only required when target path needs elevation"
status: pending
priority: P2
source: "promoted from /gsd:capture --note"
created: 2026-05-26
area: tooling
files:
  - internal/filetransfer/upload.go
---

## Problem

The current deploy code always runs the full sudo fallback sequence (direct → passwordless sudo → interactive password) regardless of the target path. For user-writable paths like `~/myproject`, this causes unnecessary sudo probing and password prompts even though the SSH user already has write access.

## Solution

At deploy start, probe whether the configured target path requires elevation:
- Attempt to stat/write the target as the SSH user
- If it succeeds (or the path doesn't exist but the parent is writable), skip all sudo scaffolding entirely
- If it fails with permission denied, proceed with the existing `sudoRunWithFallback` path

This makes deploys to home directories fully passwordless with no special configuration. The check is a simple remote probe added before the `sudoRunWithFallback` closure is invoked.

## Context

Promoted from quick note captured on 2026-05-25.

The detection logic should live near the top of `Upload()` in `internal/filetransfer/upload.go`. The probe could reuse the existing SFTP client (already opened at that point) to attempt a `MkdirAll` on the target path as the SSH user — if that succeeds, set a `needsSudo=false` flag and bypass `sudoRunWithFallback` entirely for all subsequent mkdir/mv commands.

## Acceptance Criteria

- [ ] Deploy to a user-writable path (e.g. `~/myproject`) never invokes sudo or prompts for a password
- [ ] Deploy to `/opt/<project>` still uses the full sudo fallback sequence
- [ ] The probe does not leave partial state if the path doesn't exist yet
