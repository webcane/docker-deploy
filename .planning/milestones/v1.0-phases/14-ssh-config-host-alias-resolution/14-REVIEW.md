---
phase: 14-ssh-config-host-alias-resolution
reviewed: 2026-05-29T00:00:00Z
depth: standard
files_reviewed: 5
files_reviewed_list:
  - internal/sshconfig/sshconfig.go
  - internal/sshconfig/sshconfig_test.go
  - internal/config/config.go
  - internal/config/config_test.go
  - cmd/docker-deploy/main.go
findings:
  critical: 1
  warning: 3
  info: 2
  total: 6
status: issues_found
---

# Phase 14: Code Review Report

**Reviewed:** 2026-05-29
**Depth:** standard
**Files Reviewed:** 5
**Status:** issues_found

## Summary

Phase 14 adds SSH config alias resolution via a new `sshconfig.LookupHost` function and wires it into `config.resolveHostString`. The core parsing logic is sound and the test coverage is solid for happy-path cases. However, there are three issues that affect correctness or security in real-world use: a silent misparsing of IPv6 addresses in `HostName` directives, an undetected I/O error path in the SSH config scanner, and a username validation false-rejection that blocks valid SSH config users. There is also one misleading code comment that incorrectly describes a `break` statement's scope.

---

## Critical Issues

### CR-01: IPv6 `HostName` directives in SSH config are silently misparsed

**File:** `internal/config/config.go:267-272`

**Issue:** `resolveHostString` builds a synthetic `ssh://` URL by concatenating `entry.HostName` directly into the string. When `HostName` is an IPv6 address (e.g. `::1` or `fd00::1`), the resulting URL is malformed because IPv6 literals require bracket-wrapping in URIs (RFC 3986 §3.2.2). `url.Parse` silently succeeds on the malformed URL, interpreting the colons as host:port separators:

```
entry.HostName = "::1"
synthetic     = "ssh://alice@::1"   // malformed
url.Hostname() = ":"                 // WRONG: should be "::1"
url.Port()     = "1"                 // WRONG: should be absent
```

The SSH dial then uses `Hostname=":"` and `Port=1`, silently connecting to the wrong target (or failing at TCP level with a confusing network error). No validation error is ever returned to the user.

This is security-adjacent: a misconfigured or adversarially crafted SSH config could redirect the SSH connection to a different host with no warning, bypassing known_hosts verification for the intended target.

**Fix:** Bracket-wrap the `HostName` before constructing the synthetic URL when it contains a colon:

```go
// In resolveHostString, before building synthetic:
hostLiteral := entry.HostName
if strings.Contains(hostLiteral, ":") {
    // IPv6 address — must be bracket-wrapped in URLs (RFC 3986 §3.2.2)
    hostLiteral = "[" + hostLiteral + "]"
}
synthetic := "ssh://" + userPart + hostLiteral + portPart
```

---

## Warnings

### WR-01: `scanner.Err()` never checked — I/O errors during SSH config parse are silently swallowed

**File:** `internal/sshconfig/sshconfig.go:54-118`

**Issue:** `bufio.Scanner.Scan()` stops and returns `false` on both EOF and I/O errors, but only `scanner.Err()` distinguishes between them. The `for scanner.Scan()` loop in `LookupHost` terminates without ever calling `scanner.Err()`. If an I/O error occurs mid-read (e.g. on a slow NFS-mounted home directory), the function silently returns a partial result: either a truncated `HostEntry` with `found=true` (if the error occurred after the `Host` line was read), or `found=false` (if the error occurred before it). In either case the caller receives no signal that config parsing failed.

**Fix:** Check `scanner.Err()` after the loop and propagate it:

```go
// After the for scanner.Scan() loop, before the post-loop found check:
if err := scanner.Err(); err != nil {
    return HostEntry{}, false
}
```

Or change the signature to `(HostEntry, bool, error)` to propagate the error to callers (preferred, but a larger change).

---

### WR-02: `isValidUnixUsername` rejects valid SSH config `User` directive values

**File:** `internal/config/config.go:88-98` and `config.go:267-272`

**Issue:** `resolveHostString` embeds the `User` value from an SSH config `User` directive verbatim into a `ssh://user@hostname` URL, then routes it through `ParseHost` which calls `isValidUnixUsername`. The allowlist (`a-z A-Z 0-9 . _ -`) rejects usernames containing `+`, `@`, or other characters that are valid POSIX usernames on some Linux distributions (see `useradd(8)`: "Usernames may only be up to 32 characters long" — no character set restriction beyond what `/etc/passwd` parses). A user with `User user+admin` in their `~/.ssh/config` receives:

```
--host flag: invalid host URL "ssh://user+admin@host": username "user+admin" contains disallowed characters
```

This is a misleading error (it references an internal synthetic URL the user never wrote) and a false rejection of a configuration they set in their own SSH config.

**Fix:** The username validation in `isValidUnixUsername` is designed for operator-facing error messages (sudoers suggestion path, per the comment). It should not gate alias resolution. Either:

1. Skip `isValidUnixUsername` in the alias path by using `url.User(entry.User)` to build the `*url.Userinfo` directly rather than going through string concatenation and `ParseHost`:

```go
u := &url.URL{
    Scheme: "ssh",
    User:   url.User(entry.User), // preserves user as-is, no re-validation
    Host:   hostLiteral + portPart,
}
return Host{
    User:     entry.User,
    Hostname: entry.HostName,
    Port:     port,
}, nil
```

2. Or add a bypass in `resolveHostString` that builds `Host{}` directly from `HostEntry` fields without constructing a synthetic URL at all, since the fields are already parsed.

---

### WR-03: `break` on line 88 exits the `switch`, not the `for` loop — misleading comment causes maintenance risk

**File:** `internal/sshconfig/sshconfig.go:86-89`

**Issue:** The comment "If we already found our block, stop scanning" is incorrect. In Go, a bare `break` inside a `switch` statement exits the `switch` case and returns control to the enclosing `for` loop — it does NOT terminate the loop. The scanning continues for every remaining line in the file. This is not a correctness bug today (the `if active && !found` guards on every other case correctly ignore subsequent content), but the misleading comment will cause a future maintainer to trust the early-exit claim and possibly remove those guards.

**Fix:** Use a labeled break to actually stop the loop, or at minimum fix the comment:

```go
scan:
for scanner.Scan() {
    // ...
    case "host":
        if active && !found {
            found = true
        }
        if found {
            break scan  // actually exits the for loop
        }
        // ...
}
```

---

## Info

### IN-01: SSH config `IdentityFile` tokens (`%d`, `%r`, `%h`) are not expanded

**File:** `internal/sshconfig/sshconfig.go:177-186`

**Issue:** `expandPath` only handles `~/` prefix expansion. OpenSSH supports `%d` (home directory), `%r` (remote username), `%h` (hostname), `%u` (local username), and other tokens in `IdentityFile` paths. A user with `IdentityFile %d/.ssh/id_%r` in their config will get the literal string passed to `os.ReadFile`, which will fail silently (the key is skipped per the "silently skip unloadable keys" design). The user experiences "no keys available" without any indication of why. This is within the stated scope limitations (D-11 scope is limited) but should be documented as a known limitation.

**Fix:** Add a comment to `expandPath` documenting the known limitation, or emit a warning log when a path contains `%` characters:

```go
// expandPath expands ~ to the user home directory in SSH config paths.
// Note: OpenSSH % tokens (e.g. %d, %r, %h) are not expanded — paths
// containing % are returned as-is and will fail to load silently.
func expandPath(path string) string {
```

---

### IN-02: `--host` flag description does not mention SSH config alias support

**File:** `cmd/docker-deploy/main.go:73`

**Issue:** The flag description reads `"Remote host in ssh://user@host:port format (overrides deploy.yaml)"`. Phase 14 adds bare alias support (e.g. `--host minipc`), but the help text was not updated. Users who read `docker deploy --help` will not know they can use SSH config aliases.

**Fix:**
```go
cmd.Flags().StringVar(&host, "host", "", "Remote host: ssh://user@host:port URL or SSH config alias (overrides deploy.yaml)")
```

---

_Reviewed: 2026-05-29_
_Reviewer: Claude (gsd-code-reviewer)_
_Depth: standard_
