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
  warning: 2
  info: 2
  total: 4
status: issues_found
---

# Phase 10: Code Review Report

**Reviewed:** 2026-06-01
**Depth:** standard
**Files Reviewed:** 10
**Status:** issues_found

## Summary

Phase 10 delivers shell tab completion via a `completion` subcommand (`bash`/`zsh`) and a `ListHosts` function that reads `~/.ssh/config` to supply dynamic `--host` flag candidates. The overall design is sound: the `completion` package is well-isolated, the D-03 silent-failure contract is correctly observed throughout, and the main wiring in `buildCompletionCmd`/`buildDeployCmd` is clean. No security issues were found.

Two correctness bugs were found in the SSH config parser (`sshconfig.go`): both stem from the same root cause — the `strings.Fields` splitter does not account for the OpenSSH-documented `Keyword = Value` (equals-sign) syntax. Two low-severity quality items round out the report.

## Warnings

### WR-01: SSH config `Keyword = Value` format silently stores `"="` as the directive value

**File:** `internal/sshconfig/sshconfig.go:65-80`

**Issue:** The parser splits each config line with `strings.Fields()` and unconditionally takes `parts[1]` as the directive value. OpenSSH's `ssh_config(5)` documents that keyword–value pairs may be separated by either whitespace _or_ whitespace-padded `=`: `"HostName = 192.168.1.50"` is valid. When the equals-sign form is used, `strings.Fields` produces `["HostName", "=", "192.168.1.50"]`, so `value := parts[1]` is `"="` instead of the hostname:

```
HostName = 192.168.1.50   →  entry.HostName = "="       (wrong)
Port = 2222               →  strconv.Atoi("=") fails silently, entry.Port = 0
IdentityFile = ~/.ssh/id  →  rawIdentityFiles appended with "="
User = deploy             →  entry.User = "="            (wrong)
```

This causes silent SSH dial failures for users who write their `~/.ssh/config` in the equals-sign style. The same bug exists identically in `LookupHost` (line 80) and `ListHosts` (line 181 context).

**Fix:** Before extracting the value, strip a bare `=` token at `parts[1]` so both styles are normalised:

```go
// After "parts := strings.Fields(line)" and the "if len(parts) < 2" guard,
// normalise "Keyword = Value" to "Keyword Value" form:
if len(parts) >= 3 && parts[1] == "=" {
    parts = append(parts[:1], parts[2:]...)
}
value := parts[1]
```

Apply this normalization in both the `LookupHost` scanner loop (around line 65) and the `ListHosts` scanner loop (around line 181).

---

### WR-02: `ListHosts` emits `"="` as a spurious shell-completion candidate

**File:** `internal/sshconfig/sshconfig.go:187-192`

**Issue:** When a config line reads `Host = minipc`, `parts[1:]` is `["=", "minipc"]`. Neither token contains `*` or `?`, so both are appended to `aliases`. Shell completion then offers `"="` as a valid `--host` value alongside the real alias. This is a distinct observable failure from WR-01 (polluted completions rather than wrong resolved value) but has the same root cause.

**Fix:** Apply the same `parts[1] == "="` strip described in WR-01 before iterating `parts[1:]` in the `ListHosts` `keyword == "host"` branch.

---

## Info

### IN-01: `bash_test.go` defines a `min()` helper that shadows the Go 1.21+ builtin

**File:** `internal/completion/bash_test.go:33-37`

**Issue:** Go 1.21 promoted `min` to a language builtin. The project uses Go 1.26.3. The local `func min(a, b int) int` in `bash_test.go` shadows the builtin. `zsh_test.go` (same `completion_test` package) calls `min()` at line 29 without defining it, relying on the `bash_test.go` definition — an implicit cross-file dependency. Deleting or renaming `bash_test.go` would break `zsh_test.go` with a non-obvious error.

**Fix:** Delete the `min` function from `bash_test.go` entirely. Both `bash_test.go:29` and `zsh_test.go:29` then use the language builtin automatically:

```go
// Remove from bash_test.go (lines 33-37):
func min(a, b int) int {
    if a < b {
        return a
    }
    return b
}
// No change needed at the call sites — builtin min takes over.
```

---

### IN-02: `TestLoadSigners_DelegatesLookupHost` discards `*testing.T`, preventing failure reporting if the test is extended

**File:** `internal/sshconfig/sshconfig_test.go:125`

**Issue:** The function signature uses a blank identifier: `func TestLoadSigners_DelegatesLookupHost(_ *testing.T)`. This makes it impossible to call `t.Fatal`, `t.Error`, or `t.Skip` inside the test body. The current intent (no-panic check only) is fine, but the pattern silently absorbs failures if the function is ever extended with an assertion. It also reads as an oversight to future contributors.

**Fix:** Name the parameter and add a comment to make the intent explicit:

```go
func TestLoadSigners_DelegatesLookupHost(t *testing.T) {
    t.Helper()
    // Only verifies no panic; keys may be absent in CI.
    signers := LoadSigners("/nonexistent/path", "anyhost")
    _ = signers
}
```

---

_Reviewed: 2026-06-01_
_Reviewer: Claude (gsd-code-reviewer)_
_Depth: standard_
