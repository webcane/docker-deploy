---
phase: 10-add-phase-autosuggestion
reviewed: 2026-06-01T00:00:00Z
depth: standard
files_reviewed: 10
files_reviewed_list:
  - internal/sshconfig/sshconfig.go
  - internal/sshconfig/sshconfig_test.go
  - internal/completion/completion.go
  - internal/completion/bash.go
  - internal/completion/zsh.go
  - internal/completion/completion_test.go
  - internal/completion/bash_test.go
  - internal/completion/zsh_test.go
  - cmd/docker-deploy/main.go
  - cmd/docker-deploy/main_test.go
findings:
  critical: 0
  warning: 3
  info: 4
  total: 7
status: issues_found
---

# Phase 10: Code Review Report

**Reviewed:** 2026-06-01T00:00:00Z
**Depth:** standard
**Files Reviewed:** 10
**Status:** issues_found

## Summary

Phase 10 introduces shell tab-completion support via a new `internal/completion` package, a `completion [bash|zsh]` subcommand in `cmd/docker-deploy/main.go`, and a new `internal/sshconfig` package that parses `~/.ssh/config` to power the `--host` flag completions. The implementation is generally sound and handles the D-03 silent-failure contract correctly throughout.

Three warnings were found: a whitespace-split parser that silently truncates IdentityFile paths containing spaces, a missing port-range validation that could store an invalid port from a corrupt SSH config, and an implicit cross-file dependency in the test suite. Four informational items cover a known OpenSSH multi-block behaviour gap, the shadowed `min` builtin, inconsistent `os.Stat` usage in `ComposeFileCompletionFunc`, and an unexpanded bare `~` edge case.

No critical/security issues were found.

## Warnings

### WR-01: `value := parts[1]` silently truncates IdentityFile paths containing spaces

**File:** `internal/sshconfig/sshconfig.go:80`
**Issue:** The parser extracts directive values with `value := parts[1]`, where `parts` is `strings.Fields(line)`. Any SSH config directive whose value contains whitespace (e.g. `IdentityFile "/home/user/my keys/id_ed25519"` or an unquoted path with a backslash-escaped space) is silently truncated to the first whitespace-delimited token. For `IdentityFile`, this produces an invalid path that `loadSigner` will silently skip — meaning keys in paths with spaces are never loaded, with no diagnostic. `HostName` and `User` directives are also affected if they contain spaces (an edge case, but legal in quoted SSH config values).

**Fix:** Reconstruct the full value from `parts[1:]` using `strings.Join`, which at minimum handles the common unquoted multi-word case. Full SSH config quoting semantics (quoted strings, backslash escapes) are out of scope for this parser, but joining all remaining tokens is strictly better than taking only the first:
```go
// Replace line 80:
value := strings.Join(parts[1:], " ")
```
Add a comment noting that quoted-string parsing per `ssh_config(5)` is not implemented, consistent with the existing D-11 note on Include.

---

### WR-02: No port-range validation — out-of-range Port stored and propagated

**File:** `internal/sshconfig/sshconfig.go:111`
**Issue:** `strconv.Atoi(value)` succeeds for any integer including values outside the valid TCP port range (1–65535), e.g. `Port 99999` or `Port 0`. These values are stored in `entry.Port` and propagated to `sshpkg.DialConfig.Port`. The SSH dial will then fail with a confusing OS-level error rather than an actionable "invalid port" message. `Port 0` is a special concern: the caller treats 0 as "unset, default to 22" (main.go line 326), so a literal `Port 0` in the SSH config would incorrectly become port 22 instead of being rejected.

**Fix:**
```go
case "port":
    if active && !found {
        if p, err := strconv.Atoi(value); err == nil && p >= 1 && p <= 65535 {
            entry.Port = p
        }
        // values outside 1-65535 silently ignored (invalid SSH config)
    }
```

---

### WR-03: `zsh_test.go` calls `min()` defined only in `bash_test.go` — implicit cross-file test dependency

**File:** `internal/completion/zsh_test.go:29`
**Issue:** `zsh_test.go` calls `min(100, len(out))` without defining `min`. The function is defined in `bash_test.go` (lines 33–37). Both files share the `completion_test` package so this compiles, but it creates an implicit coupling: `zsh_test.go` depends on `bash_test.go` being present. If `bash_test.go` were deleted or renamed, the build would fail with a confusing "undefined: min" error. With Go 1.21+ the builtin `min` is available, making the `bash_test.go` definition redundant AND `zsh_test.go`'s implicit dependency invisible (the builtin would be used instead if `bash_test.go` is gone), masking the issue but not eliminating it.

**Fix:** Delete `func min` from `bash_test.go` (lines 33–37) — it shadows the Go 1.21+ builtin. Both `bash_test.go` and `zsh_test.go` should use the builtin `min` directly. No import or definition needed:
```go
// bash_test.go — remove these lines:
func min(a, b int) int {
    if a < b {
        return a
    }
    return b
}
```

## Info

### IN-01: OpenSSH multi-block directive inheritance not implemented — only first matching block is applied

**File:** `internal/sshconfig/sshconfig.go:86-91`
**Issue:** OpenSSH processes ALL matching `Host` blocks and applies each directive from the first block that specifies it (first-match-wins per directive). The current parser stops collecting directives as soon as the matching block ends (sets `found=true` and breaks on the next `Host` line). A common pattern — specific alias block followed by a `Host *` defaults block — means directives like `User`, `IdentityFile`, and `Port` in the `Host *` block are silently ignored for the matched alias if the specific block omits them.

**Example that silently fails:**
```
Host myserver
  HostName 192.168.1.10

Host *
  User deploy
  IdentityFile ~/.ssh/id_ed25519
```
`LookupHost("myserver")` returns `User=""` and no IdentityFiles, instead of the OpenSSH-expected `User="deploy"` and `~/.ssh/id_ed25519`.

This is an architectural decision consistent with the D-11 note on silently skipping Include directives. No fix is required unless the completion use case demands full OpenSSH compatibility, but the limitation should be documented in the `LookupHost` godoc:
```go
// NOTE: Only the first matching Host block is applied. Subsequent blocks
// (including "Host *" defaults) are not merged. This differs from OpenSSH's
// first-match-wins-per-directive behaviour.
```

---

### IN-02: `ComposeFileCompletionFunc` uses implicit cwd via bare `os.Stat` — inconsistent with sibling functions

**File:** `internal/completion/completion.go:65`
**Issue:** `ComposeFileCompletionFunc` calls `os.Stat(name)` with a bare filename (`"compose.yaml"`, `"docker-compose.yml"`), relying implicitly on the process's current working directory. The sibling functions `HostCompletionFunc` and `PathCompletionFunc` both call `os.Getwd()` explicitly. The implicit approach works correctly at runtime but is harder to test in isolation (tests must `os.Chdir` — which they do, but tests run sequentially with shared cwd state) and does not produce a clear error if `os.Getwd()` would have failed.

**Fix:** For consistency, acquire cwd explicitly:
```go
func ComposeFileCompletionFunc(_ *cobra.Command, _ []string, _ string) ([]cobra.Completion, cobra.ShellCompDirective) {
    cwd, err := os.Getwd()
    if err != nil {
        return nil, cobra.ShellCompDirectiveNoFileComp
    }
    var suggestions []cobra.Completion
    for _, name := range []string{"compose.yaml", "docker-compose.yml"} {
        if _, err := os.Stat(filepath.Join(cwd, name)); err == nil {
            suggestions = append(suggestions, cobra.Completion(name))
        }
    }
    return suggestions, cobra.ShellCompDirectiveNoFileComp
}
```

---

### IN-03: `expandPath` does not expand bare `~` (without trailing slash)

**File:** `internal/sshconfig/sshconfig.go:245-247`
**Issue:** The tilde expansion guard is `strings.HasPrefix(path, "~/")`, which requires a slash after `~`. A bare `IdentityFile ~` (without a path component) is not expanded and is passed literally as `"~"` to `loadSigner`, which then fails with a file-not-found error (silently skipped per design). This is a very rare edge case — `IdentityFile ~` is not a valid key path — but the behaviour is surprising and undocumented.

**Fix:** Add handling for the exact `"~"` case, or document the limitation:
```go
if path == "~" {
    path = homeDir
} else if strings.HasPrefix(path, "~/") {
    path = filepath.Join(homeDir, path[2:])
}
```

---

### IN-04: `Register` silently discards completion registration errors — typo in flag name would be undetectable

**File:** `internal/completion/completion.go:19-21`
**Issue:** All three `RegisterFlagCompletionFunc` calls discard their errors with `_ =`. If a flag name is misspelled (e.g., `"composefile"` instead of `"compose-file"`), the completion function is silently not registered. The comment acknowledges this. At present the flag names match exactly, so this is not a bug. However, there is no test that verifies registration actually succeeded by checking the returned error — the existing tests only verify `GetFlagCompletionFunc` returns ok, not that Register returned no error internally.

**Fix:** Not required given the flags are verified by test. Optionally, panic in dev builds if registration fails, using a build tag:
```go
// In Register(), for defensive debugging during development:
if err := cmd.RegisterFlagCompletionFunc("host", HostCompletionFunc); err != nil {
    // Flag not defined yet — caller must define flags before calling Register.
    // This is a programming error, not a runtime error.
    panic(fmt.Sprintf("completion.Register: %v", err))
}
```

---

_Reviewed: 2026-06-01T00:00:00Z_
_Reviewer: Claude (gsd-code-reviewer)_
_Depth: standard_
