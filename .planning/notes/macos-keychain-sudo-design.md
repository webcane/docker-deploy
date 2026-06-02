---
title: macOS Keychain sudo password design decisions
date: 2026-06-02
context: Exploration session — keystore for sudo passwords
---

## Decision: shell out to `/usr/bin/security` (no CGo)

Rejected `zalando/go-keyring` and `99designs/keyring` because they require CGo,
which complicates cross-compilation. `/usr/bin/security` is present on every macOS
install and has zero additional dependencies.

## Keychain entry structure

- **Service**: `docker-deploy`
- **Account**: `user@host` (e.g. `deploy@192.168.1.10`)
- One entry per remote host — each host gets its own credential

## Lookup flow

1. Try `security find-generic-password -s docker-deploy -a user@host -w`
2. Hit → use as sudo password, skip interactive prompt
3. Miss/wrong → fall through to interactive prompt (existing behaviour)
4. If keychain entry existed but sudo failed → re-prompt, re-offer to update keychain

## Store flow (first successful interactive auth)

1. Prompt: `"Save sudo password for deploy@host to macOS Keychain? [y/N]"`
2. Yes → `security add-generic-password -U -s docker-deploy -a user@host -w <password>`
   (`-U` updates if the entry already exists)
3. No → skip silently, no re-prompt on future deploys

## Known security tradeoff

`-w <password>` passes the password as a CLI argument when *writing*. Briefly visible
in `ps aux`. Acceptable because: one-time write, short-lived process, read path is clean
(password returned on stdout, not in args).

## Implementation home

New `internal/keychain` package. Wired into `SudoCreds` / `promptSudoPasswordFunc`
in `internal/filetransfer/upload.go`.

## Platform scope

macOS only for now. Linux (Secret Service / pass) deferred — see [[keychain-sudo-credential-caching]].
