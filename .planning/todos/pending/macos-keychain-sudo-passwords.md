---
title: Implement macOS Keychain integration for sudo passwords
date: 2026-06-02
priority: medium
---

## Goal

Store remote sudo passwords in macOS Keychain so users are not prompted on every deploy.

## Implementation

### New package: `internal/keychain`

```go
// Lookup retrieves a stored sudo password. Returns ("", nil) if not found.
func Lookup(host, user string) (string, error)

// Store saves a sudo password. Overwrites existing entry (-U flag).
func Store(host, user, password string) error

// Delete removes a stored entry (for --clear-credentials support).
func Delete(host, user string) error
```

Shell out to `/usr/bin/security`:
- Lookup: `security find-generic-password -s docker-deploy -a user@host -w`
- Store: `security add-generic-password -U -s docker-deploy -a user@host -w <password>`
- Delete: `security delete-generic-password -s docker-deploy -a user@host`

### Wire into `internal/filetransfer/upload.go`

Extend `SudoCreds` (or wrap `promptSudoPasswordFunc`) to:
1. Check keychain before prompting interactively
2. After successful interactive prompt, ask user to save
3. If keychain entry fails sudo auth, fall back to prompt + re-offer to update

### Graceful fallback

If `/usr/bin/security` is unavailable or returns an error → behave as today (interactive prompt).

### Tests

- Unit test `internal/keychain` by mocking exec calls
- Integration: no change needed (existing sudo prompt tests still apply)

## Reference

Design decisions: [[macos-keychain-sudo-design]]
Seed / phase tracking: [[keychain-sudo-credential-caching]]
