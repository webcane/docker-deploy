---
title: macOS Keychain / system keyring integration for sudo credential caching
trigger_condition: After single-prompt sudo consolidation ships and UX feedback confirms password prompting is still friction
planted_date: 2026-05-25
status: promoted
promoted_to: Phase 14 Wave 3
---

## Idea

Allow users to store the remote sudo password in macOS Keychain (or Linux secret-service / pass) so deploys to elevated paths run fully non-interactively after first setup.

## Flow

1. On first deploy requiring sudo, prompt for password and ask: "Save to system keyring for future deploys? [y/N]"
2. Store under a keyed entry (e.g. `docker-deploy:<host>:<user>`)
3. On subsequent deploys, retrieve silently and pipe via `sudo -S`
4. Surface a `--clear-credentials` flag to remove stored entries

## Implementation approach (decided 2026-06-02)

**macOS**: shell out to `/usr/bin/security` — no CGo, zero deps. See [[macos-keychain-sudo-design]].
**Linux**: deferred — Secret Service (DBus) for desktops, headless fallback TBD.
**Windows**: under consideration.

CGo-based libraries (`zalando/go-keyring`, `99designs/keyring`) rejected for macOS due
to cross-compilation complexity.

## Constraints

- Only activate when target path requires sudo (see [[sudo-path-aware-detection]])
- Must fail gracefully if keyring is unavailable — fall back to prompt
- Never log or expose the stored credential
