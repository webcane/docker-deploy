---
created: 2026-06-02T00:00:00Z
title: "Implement macOS Keychain / system keyring sudo credential caching"
area: general
files: [internal/filetransfer/sudo.go]
---

## Problem

Sudo passwords are prompted on every deploy and held in memory only for the
duration of the session (`SudoCreds.Zero()` wipes on exit). Users on
password-protected sudo remotes must re-enter their password each run.

## Deferred from

Phase 13 (CLI Subcommands & Deploy UX) — Wave 3 was explicitly deferred.
The `SudoExec`/`SudoCreds` infrastructure in `internal/filetransfer/sudo.go`
is the intended hook-in point.

## Solution

Add opt-in credential caching via OS keyring:
- Use `go-keyring` or `99designs/keyring` for cross-platform support
- Store under a service key scoped to `docker-deploy/<host>`
- New flag `--clear-credentials` / config `clear_credentials: true` to wipe
  the cached entry on demand
- Cache is only written when the user explicitly opts in (e.g. `remember_sudo: true`
  in `deploy.yaml`)

Security note: keyring storage is per-user and encrypted by the OS, but the
risk profile should be documented — this stores a sudo password, not just a
deploy token.
