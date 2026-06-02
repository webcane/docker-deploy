---
phase: 10-add-phase-autosuggestion
reviewed: 2026-06-02T00:00:00Z
depth: standard
files_reviewed: 10
files_reviewed_list:
  - cmd/docker-deploy/main.go
  - cmd/docker-deploy/main_test.go
  - contrib/docker-deploy.bash
  - contrib/install-completions.sh
  - internal/completion/bash.go
  - internal/completion/bash_test.go
  - internal/completion/zsh.go
  - internal/completion/zsh_test.go
  - internal/sshconfig/sshconfig.go
  - internal/sshconfig/sshconfig_test.go
findings:
  critical: 2
  warning: 4
  info: 2
  total: 8
status: issues_found
---

# Phase 10: Code Review Report

**Reviewed:** 2026-06-02T00:00:00Z
**Depth:** standard
**Files Reviewed:** 10
**Status:** issues_found

## Summary

Phase 10 adds static shell completion scripts (`contrib/docker-deploy.bash`, `contrib/_docker-deploy`), an install helper (`contrib/install-completions.sh`), a new `internal/sshconfig` package with `ListHosts` for SSH config alias autosuggestion, and wires everything into the CLI via a hidden `completion` subcommand. The Go code is generally well-structured. Two security-class issues were found: an unverified download in the install script and a command-injection vector in the generated bash completion template. Four warnings cover an ignored error that silently breaks key loading, a host-key-parsing gap that drops valid `Keyword=Value` lines, a confusing `$SHELL` fallback, and unexpanded tilde in error messages.

## Critical Issues

### CR-01: install-completions.sh downloads script without integrity check

**File:** `contrib/install-completions.sh:89`
**Issue:** `curl -fsSL "$DOWNLOAD_URL" -o "$DEST"` writes the downloaded file directly to the completion directory with no checksum or signature verification. The destination (`$DEST`) is a shell script that will be sourced by the user's shell on every login. A MITM attacker, a compromised CDN edge node, or a future supply-chain compromise of the `raw.githubusercontent.com` delivery path can serve arbitrary shell code that is silently installed and executed. This is especially severe because the same tool manages `.env` files containing secrets (per CLAUDE.md).
**Fix:**
```sh
# After downloading, verify SHA-256 against a known-good checksum.
# Option A — embed expected hash per-version in the script:
EXPECTED_SHA256="<hash-of-release>"
actual=$(shasum -a 256 "$DEST" | awk '{print $1}')
if [ "$actual" != "$EXPECTED_SHA256" ]; then
  echo "Checksum mismatch — aborting installation" >&2
  rm -f "$DEST"
  exit 1
fi

# Option B (simpler) — publish a .sha256 sidecar and verify:
curl -fsSL "${DOWNLOAD_URL}.sha256" -o "${DEST}.sha256"
(cd "$(dirname "$DEST")" && shasum -a 256 -c "${DEST}.sha256")
rm -f "${DEST}.sha256"
```

### CR-02: eval of user-influenced words in bash completion template

**File:** `contrib/docker-deploy.bash:48`
**Issue:** `out=$(eval "${requestComp}" 2>/dev/null)` constructs `requestComp` from `${words[*]}` (line 26), which is the raw command-line tokens typed by the user. `eval` turns those tokens back into shell code. If a user types a completion invocation containing shell metacharacters (e.g., `` docker-deploy --host `id` `` and then presses TAB), the backtick or `$()` construct in `words[*]` will be re-evaluated. This is the standard cobra-generated completion pattern but it constitutes a code-execution path triggered by TAB completion, making it exploitable if an attacker can place metacharacters into the argument list (e.g., via a rogue `$PATH` entry, a malicious `docker-deploy` alias, or a crafted compose file path autocomplete). The risk is amplified because this tool operates on SSH credentials.
**Fix:** This is generated code — the fix is to update cobra's `GenBashCompletionV2` template or use the `--no-descriptions` flag at generation time, neither of which alone removes `eval`. At minimum, document the known risk. If the project controls the shell template (it is committed statically), replace the `eval` call with a direct command invocation:
```sh
# Replace:
out=$(eval "${requestComp}" 2>/dev/null)

# With (avoids re-evaluation of metacharacters in args):
out=$("${words[0]}" __completeNoDesc "${args[@]}" 2>/dev/null)
```
Note: `args=("${words[@]:1}")` (array, line 25) must be used here — do not use `${args[*]}` (word-split string).

## Warnings

### WR-01: Silently ignored UserHomeDir error corrupts IdentityFile expansion

**File:** `internal/sshconfig/sshconfig.go:125`
**Issue:** `home, _ := os.UserHomeDir()` discards the error. When `os.UserHomeDir()` fails (no `$HOME`, no passwd entry — possible in containers or restricted environments), `home` is `""`. Then `expandPath("~/.ssh/id_ed25519", "", ...)` expands `~/...` via `filepath.Join("", ".ssh/id_ed25519")` which resolves to the relative path `.ssh/id_ed25519`. The subsequent `loadSigner` call silently skips it (key not found), producing an empty signers list with no diagnostic. The user sees an SSH authentication failure with no indication of the root cause.
**Fix:**
```go
home, err := os.UserHomeDir()
if err != nil {
    // Return found=true but with empty IdentityFiles; caller falls back to defaults.
    // Logging is not available here; rely on LoadSigners fallback.
    return entry, true
}
```
Or at minimum log a warning so the silent failure is visible.

### WR-02: Keyword=Value (no spaces) ssh_config form silently dropped

**File:** `internal/sshconfig/sshconfig.go:214-223`
**Issue:** `parseConfigLine` calls `strings.Fields(line)` which splits on whitespace only. The line `IdentityFile=/home/alice/.ssh/id_ed25519` (no spaces around `=`) produces a single-element `parts` slice, fails the `len(parts) < 2` guard, and returns `("", nil)` — the directive is silently lost. While `Keyword=Value` without spaces is not the most common form in practice, `ssh_config(5)` documents it as valid for most keywords. A user with such a config will silently get no configured identity files and may fail authentication.
**Fix:**
```go
func parseConfigLine(line string) (keyword string, values []string) {
    line = strings.TrimSpace(line)
    if line == "" || strings.HasPrefix(line, "#") {
        return "", nil
    }
    // Handle "Keyword=Value" with no spaces before splitting on whitespace.
    if idx := strings.IndexByte(line, '='); idx > 0 {
        key := strings.TrimSpace(line[:idx])
        val := strings.TrimSpace(line[idx+1:])
        if key != "" && val != "" {
            return strings.ToLower(key), strings.Fields(val)
        }
    }
    parts := strings.Fields(line)
    if len(parts) < 2 {
        return "", nil
    }
    keyword = strings.ToLower(parts[0])
    if len(parts) >= 3 && parts[1] == "=" {
        parts = append(parts[:1], parts[2:]...)
    }
    return keyword, parts[1:]
}
```

### WR-03: Empty $SHELL produces unhelpful error in install-completions.sh

**File:** `contrib/install-completions.sh:22`
**Issue:** `shell=$(basename "${SHELL:-}")` — when `$SHELL` is unset, `${SHELL:-}` expands to an empty string and `basename ""` returns `""`. The subsequent `case "$shell" in` falls to `*) exit 1` with the message `"unsupported shell:  (supported: bash, zsh)"` — the shell name in the message is blank. Users running this from a minimal environment where `$SHELL` is unset get no actionable guidance.
**Fix:**
```sh
if [ -z "${SHELL:-}" ]; then
  echo "SHELL environment variable is not set; cannot detect shell" >&2
  echo "Set SHELL=/bin/bash or SHELL=/bin/zsh and re-run." >&2
  exit 1
fi
shell=$(basename "$SHELL")
```

### WR-04: loadGlobalConfig error message contains unexpanded tilde

**File:** `cmd/docker-deploy/main.go:275`
**Issue:** When `config.LoadFile` fails for the global config, the error message is built with `filepath.Join(globalDir, "deploy.yaml")` where `globalDir` is already a fully-resolved absolute path (from `filepath.Join(home, ".docker", "cli-plugins")`). However, the outer error wraps this as `"global config %s: %w"` with the `globalDir` path. This is actually correct since `home` is resolved from `os.UserHomeDir()`. The issue is that the error message uses `filepath.Join(globalDir, "deploy.yaml")` in the `fmt.Errorf` call — but `globalDir` is already the full path, so this is fine. **Revised finding:** The actual issue is that when `os.UserHomeDir()` fails (line 268), the error returned is `"cannot determine home directory for global config: ..."` — there is no test for this path, and the caller (`runValidate`, `runDryRun`, `runDeploy`) wraps it generically as `"loading global config: ..."`, losing the specific context. Low-severity but the missing home dir error path lacks test coverage.
**Fix:** Add a test case that stubs `os.UserHomeDir` failure (e.g., by temporarily unsetting `$HOME` in an integration test) or document that this path is accepted as untested.

## Info

### IN-01: t.Helper() called in top-level test function

**File:** `internal/sshconfig/sshconfig_test.go:126`
**Issue:** `t.Helper()` is called at line 126 inside `TestLoadSigners_DelegatesLookupHost`, which is a top-level `*testing.T` test function, not a helper called by other tests. `t.Helper()` marks a function as a test helper so that failure line numbers point to the caller rather than the helper — it has no effect when called in a top-level test function and is misleading to readers.
**Fix:** Remove the `t.Helper()` call from the top-level test function.

### IN-02: Dead code: unreachable default case returns nil without error

**File:** `cmd/docker-deploy/main.go:183`
**Issue:** The `default: return nil` branch in `buildCompletionCmd`'s `RunE` is acknowledged as unreachable in the comment. Silently returning `nil` from an "impossible" branch means if cobra's validation ever changes or a code refactor introduces a new case without updating the switch, the command silently no-ops instead of failing visibly.
**Fix:**
```go
default:
    // This should never be reached because cobra's OnlyValidArgs rejects
    // any value not in ValidArgs before RunE fires.
    return fmt.Errorf("unsupported shell: %q", args[0])
```

---

_Reviewed: 2026-06-02T00:00:00Z_
_Reviewer: Claude (gsd-code-reviewer)_
_Depth: standard_
