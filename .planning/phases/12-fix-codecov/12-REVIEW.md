---
phase: 12-fix-codecov
reviewed: 2026-05-24T00:00:00Z
depth: standard
files_reviewed: 4
files_reviewed_list:
  - COMPARISON.md
  - INSTALL.md
  - README.md
  - cmd/docker-deploy/main.go
findings:
  critical: 0
  warning: 3
  info: 2
  total: 5
status: issues_found
---

# Phase 12: Code Review Report

**Reviewed:** 2026-05-24
**Depth:** standard
**Files Reviewed:** 4
**Status:** issues_found

## Summary

This phase touches four files: two documentation files (README.md, INSTALL.md), one comparison document (COMPARISON.md), and one Go source file (cmd/docker-deploy/main.go). The Go change is a two-string-literal edit ("remote VPS" → "remote host"). The documentation changes add INSTALL.md, restructure README installation, and append a "Missing a tool?" section to COMPARISON.md.

No security vulnerabilities or data loss risks were found. Three warnings were identified: a `GOBIN` tilde-expansion portability issue in INSTALL.md, a silent deploy-cancellation path in main.go, and a documentation inconsistency between README.md and main.go after the "VPS" → "host" rename. Two informational items cover a sentinel value in dry-run and a discarded session close error.

---

## Warnings

### WR-01: `GOBIN=~/.docker/cli-plugins` — tilde not expanded by all shells

**File:** `INSTALL.md:51`
**Issue:** The `go install` example uses `GOBIN=~/.docker/cli-plugins`. Tilde expansion (`~`) is performed by interactive shells (bash/zsh), but is NOT expanded when the command is run via `sh -c`, a Makefile, a CI script, or any non-interactive context. In those environments `GOBIN` will be set to the literal string `~/.docker/cli-plugins`, and `go install` will create a directory named `~` in the working directory rather than installing to the expected location. The plugin will not be discoverable by Docker CLI.

**Fix:** Use `$HOME` instead of `~`, which is expanded by all POSIX-compliant shells including non-interactive ones:

```bash
GOBIN=$HOME/.docker/cli-plugins go install github.com/webcane/docker-deploy/cmd/docker-deploy@latest
```

---

### WR-02: Silent deploy cancellation when user answers "N"

**File:** `cmd/docker-deploy/main.go:323-327`
**Issue:** When the user declines the replace-confirmation prompt by entering "N", "no", or pressing Enter, the function returns `nil` with no message printed to stderr. The EOF-with-empty-input case at line 317–320 correctly prints "No input received — deploy cancelled." but the normal declination path is completely silent. A user who presses Enter expecting the default behaviour will see the cursor return to the shell prompt with no confirmation that the deploy was cancelled, which can be mistaken for a hang or a successful no-op deploy.

**Fix:** Print a cancellation notice before returning `nil`:

```go
if !strings.EqualFold(answer, "y") && !strings.EqualFold(answer, "yes") {
    fmt.Fprintln(os.Stderr, "Deploy cancelled.")
    return nil
}
```

---

### WR-03: README tagline still says "VPS" after "remote VPS" → "remote host" rename

**File:** `README.md:5`
**Issue:** The Phase 12 goal was to replace "remote VPS" with "remote host" throughout (main.go lines 40–41 were updated). However, line 5 of README.md retains the word "VPS" in the one-line tagline:

> "Deploy your docker-compose project to any SSH-accessible VPS with a single command"

This is inconsistent with the intent of the rename and with the "What is docker-deploy?" paragraph on line 9, which correctly uses "remote host".

**Fix:**

```markdown
Deploy your docker-compose project to any SSH-accessible host with a single command — no git required on the remote.
```

---

## Info

### IN-01: Dry-run hard-codes sentinel compose file name — silently differs from deploy behaviour

**File:** `cmd/docker-deploy/main.go:133`
**Issue:** `runDryRun` passes the string `"docker-compose.yml"` as a sentinel to `config.Resolve` to skip auto-detection. This is pre-existing, documented in a comment, and intentional. However, the sentinel value is the legacy v1 filename, not the v2-preferred `compose.yaml`. If the auto-detection logic inside `config.Resolve` ever changes to check the sentinel value's content rather than just bypassing detection, this will silently select the wrong file. The comment documents the intent ("sentinel: skips auto-detect; value is unused in dry-run") but the coupling is fragile.

**Fix (low priority):** Introduce a named constant or a dedicated zero value (`""`), and update `config.Resolve` to treat an empty `ComposeFile` as "auto-detect". This eliminates the implicit sentinel contract.

---

### IN-02: `INSTALL.md` install script — fetched URL is pinned to `v1.0.0` but `INSTALL_VERSION` env var implies selectability

**File:** `INSTALL.md:10-17`
**Issue:** The "Or to select a specific version at install time" example shows:

```bash
curl -fsSL https://raw.githubusercontent.com/webcane/docker-deploy/v1.0.0/install.sh | INSTALL_VERSION=v1.0.0 sh
```

The URL path still fetches the script from `v1.0.0`. If a user wants version `v1.2.0`, they would write `INSTALL_VERSION=v1.2.0 sh`, but they would still be executing the `v1.0.0` script — which may not know how to install `v1.2.0`. The example is misleading because the script URL must also be updated to match the desired version. This is a documentation correctness issue, not a code bug, but readers will follow the pattern literally.

**Fix:** Clarify that both the URL tag and `INSTALL_VERSION` must be updated together, or show a pattern where `INSTALL_VERSION` drives a URL-agnostic latest script:

```bash
# To install a specific version, update both the URL tag and INSTALL_VERSION:
curl -fsSL https://raw.githubusercontent.com/webcane/docker-deploy/v1.2.0/install.sh | INSTALL_VERSION=v1.2.0 sh
```

---

_Reviewed: 2026-05-24_
_Reviewer: Claude (gsd-code-reviewer)_
_Depth: standard_
