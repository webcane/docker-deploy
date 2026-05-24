---
phase: 11-ci-tooling-polish
reviewed: 2026-05-24T00:00:00Z
depth: standard
files_reviewed: 18
files_reviewed_list:
  - .github/dependabot.yml
  - .github/workflows/ci.yml
  - .github/workflows/release.yml
  - .golangci.yml
  - .goreleaser.yaml
  - Makefile
  - README.md
  - cmd/docker-deploy/main.go
  - codecov.yml
  - internal/compose/run_test.go
  - internal/config/config.go
  - internal/filetransfer/upload.go
  - internal/filetransfer/upload_test.go
  - internal/health/poll.go
  - internal/preflight/checks.go
  - internal/preflight/checks_test.go
  - internal/ssh/client.go
  - internal/ssh/knownhosts.go
  - internal/sshconfig/sshconfig.go
findings:
  critical: 4
  warning: 5
  info: 3
  total: 12
fixed: 11
skipped: 1
status: fixed
---

# Phase 11: Code Review Report

**Reviewed:** 2026-05-24
**Depth:** standard
**Files Reviewed:** 18
**Status:** issues_found

## Summary

Phase 11 added Codecov coverage reporting, bumped GitHub Actions versions, added Dependabot, automated the Homebrew symlink lifecycle via goreleaser `post_install`, and introduced a golangci-lint v2 gate. The CI/CD configuration contains the most significant issues: non-existent action versions will cause every workflow run to fail, the release workflow's branch detection pattern is incorrect so releases will never fire, and the `.golangci.yml` errcheck settings-placement is wrong for v2. Core Go source files (ssh, filetransfer, config, health, preflight) are well-structured and respect all project security rules — no `InsecureIgnoreHostKey` in production paths, goroutine+select SSH timeout is correctly implemented, and atomic file copy staging is preserved.

---

## Critical Issues

### CR-01: Non-existent GitHub Actions versions break every workflow run

**File:** `.github/workflows/ci.yml:12`, `.github/workflows/ci.yml:14`, `.github/workflows/ci.yml:27`, `.github/workflows/ci.yml:29`, `.github/workflows/ci.yml:53`, `.github/workflows/ci.yml:55`, `.github/workflows/release.yml:18`, `.github/workflows/release.yml:22`, `.github/workflows/release.yml:27`

**Issue:** Every workflow references versions that do not exist as published GitHub Actions releases:
- `actions/checkout@v6` — latest stable is `v4`; `v6` does not exist
- `actions/setup-go@v6` — latest stable is `v5`; `v6` does not exist
- `codecov/codecov-action@v6` — latest stable is `v5`; `v6` does not exist
- `goreleaser/goreleaser-action@v7` — latest stable is `v6`; `v7` does not exist

GitHub Actions resolves action references by tag. When a tag does not exist, the runner fails with "Could not find action" before executing any step. Every lint, test, integration, and release job will fail immediately on any trigger. No code can ship through this pipeline.

**Fix:**
```yaml
# ci.yml and release.yml — replace all occurrences
- uses: actions/checkout@v4
- uses: actions/setup-go@v5
  with:
    go-version-file: go.mod
# ci.yml test job only
- uses: codecov/codecov-action@v5
# release.yml only
- uses: goreleaser/goreleaser-action@v6
  with:
    version: "~> v2"
    args: release --clean
```

---

### CR-02: Release workflow branch detection pattern never matches tags

**File:** `.github/workflows/release.yml:10-12`

**Issue:**
```yaml
if: >
  github.event.workflow_run.conclusion == 'success' &&
  startsWith(github.event.workflow_run.head_branch, 'v')
```

The `workflow_run` event sets `head_branch` to the **branch name** from which the workflow was triggered, not to the tag name. When CI is triggered by a tag push (e.g., `v1.2.3`), `head_branch` is set to the tag name — however, `workflow_run.head_branch` for tag-triggered runs is the **tag ref** in some GitHub configurations and may be empty or set to the default branch in others. The reliable field for tag detection in `workflow_run` is `github.event.workflow_run.head_commit.message` or checking `head_branch` against a git ref pattern, but this is not consistent.

More critically: the CI workflow (`ci.yml`) is triggered by `push: tags: ['v*']` and `pull_request`. A tag push triggers CI with `head_branch` set to the tag (e.g., `v1.2.3`). `startsWith('v1.2.3', 'v')` is `true`, so this particular check appears to work. However, the Release workflow will also fire whenever **any pull request** passes CI (since PR runs also set `conclusion == 'success'`), and for PRs `head_branch` is the PR branch name, which typically does not start with `v`. So for PRs it correctly does not release. The real bug is: if a branch named `vX-something` (e.g., `v2-refactor`) passes CI, a release will be triggered from that branch. A developer creating a branch starting with `v` will inadvertently trigger a release workflow run.

**Fix:** Filter on `head_branch` exactly matching a semver tag pattern using a regex, or use the `github.ref` on the CI side to pass the tag through:
```yaml
if: >
  github.event.workflow_run.conclusion == 'success' &&
  github.event.workflow_run.event == 'push' &&
  startsWith(github.event.workflow_run.head_branch, 'v') &&
  contains(github.event.workflow_run.head_branch, '.')
```
The additional `contains(..., '.')` check ensures bare `v` prefix branch names (e.g., `v2-refactor`) do not trigger releases, since semantic version tags always contain a dot (e.g., `v1.2.3`).

---

### CR-03: `.golangci.yml` settings block is misplaced for v2 — errcheck exclusions silently ignored

**File:** `.golangci.yml:9-18`

**Issue:**
```yaml
linters:
  disable-all: true
  enable:
    - errcheck
    - govet
    - staticcheck
  settings:
    errcheck:
      exclude-functions:
        - fmt.Fprintf
        ...
```

In golangci-lint v2 the `settings` key is a **top-level** key, not a sub-key of `linters`. The v2 config schema is:
```yaml
linters:
  enable: [...]

linters-settings:   # top-level in v1; in v2 it is just `settings` at top-level
  errcheck:
    ...
```

Placing `settings` inside `linters:` is silently ignored by golangci-lint v2 — the YAML parses without error (extra keys under `linters:` are not fatal in v2) but the errcheck exclusions are never applied. The effect is that `fmt.Fprintf`, `fmt.Fprintln`, `(*sftp.Client).Close`, and `(*ssh.Client).Close` return-value errors are checked, and errcheck will produce false-positive lint failures on all of these intentional no-check call sites throughout the codebase.

**Fix:**
```yaml
version: "2"

linters:
  disable-all: true
  enable:
    - errcheck
    - govet
    - staticcheck

settings:
  errcheck:
    exclude-functions:
      - fmt.Fprintf
      - fmt.Fprintln
      - fmt.Fprint
      - (io.Closer).Close
      - (*github.com/pkg/sftp.Client).Close
      - (*golang.org/x/crypto/ssh.Client).Close
      - (*golang.org/x/crypto/ssh.Session).Close

formatters:
  enable:
    - goimports
  settings:
    goimports:
      local-prefixes:
        - github.com/webcane/docker-deploy

issues:
  max-issues-per-linter: 0
  max-same-issues: 0
```

---

### CR-04: Sudo password exposed in SSH command string (command injection via echo)

**File:** `internal/filetransfer/upload.go:274`

**Issue:**
```go
sudoCmd := fmt.Sprintf("echo %s | sudo -S -p '' sh -c %s", ShellQuote(pw), ShellQuote(cmd))
if sshExec(client, sudoCmd) == nil {
```

`ShellQuote(pw)` wraps the password in single quotes and escapes embedded single quotes using `'\''`. This prevents word-splitting and glob expansion, but the entire shell command is passed to `sshExec` which runs it via `session.Run()`. The SSH server receives the full command string including the password. The concern is:

1. **SSH server logs**: Many SSH daemons log the executed command string. If `sshd_config` has `LogLevel VERBOSE` or the server uses audit logging, the literal password appears in server-side logs. This is a credential exposure risk to anyone with log access.
2. **Process list exposure**: `echo <password> | sudo -S` passes the credential through a pipeline where the shell `echo` argument may briefly appear in `/proc` on Linux systems between fork and exec.

The verbosity path (line 271) correctly redacts the command:
```go
fmt.Fprintf(os.Stderr, "[ssh] (sudo password cmd redacted)\n")
```
But the password itself still flows through the SSH exec command text.

**Fix:** Use SSH's native stdin piping via `session.StdinPipe()` rather than `echo pw |`. This keeps the password out of the command string entirely:
```go
session, err := client.NewSession()
if err != nil {
    return fmt.Errorf("creating SSH session: %w", err)
}
defer session.Close()

stdin, err := session.StdinPipe()
if err != nil {
    return fmt.Errorf("opening stdin pipe: %w", err)
}

sudoCmd := fmt.Sprintf("sudo -S -p '' sh -c %s", ShellQuote(cmd))
if err := session.Start(sudoCmd); err != nil {
    return fmt.Errorf("starting sudo command: %w", err)
}
fmt.Fprintln(stdin, pw)
stdin.Close()
if err := session.Wait(); err != nil {
    return fmt.Errorf("sudo command failed: %w", err)
}
return nil
```

---

## Warnings

### WR-01: `go vet` runs after coverage upload — vet failures don't gate coverage reporting

**File:** `.github/workflows/ci.yml:36-47`

**Issue:** The step order in the `test` job is: Build → Run tests → Upload coverage → Run vet. If `go vet` finds a real issue, coverage data from potentially-broken code is already uploaded to Codecov before the failure is detected. Vet is a correctness check and should precede test execution to avoid polluting coverage history with unvetted code.

**Fix:**
```yaml
- name: Run vet
  run: go vet ./...

- name: Run tests
  run: go test -coverprofile=coverage.out ./...

- name: Upload coverage to Codecov
  uses: codecov/codecov-action@v5
  with:
    files: coverage.out
    fail_ci_if_error: false
```

---

### WR-02: `health/poll.go` polls container state, not HEALTHCHECK health — the status switch is wrong

**File:** `internal/health/poll.go:207-218`

**Issue:** `inspectHealth` runs:
```go
cmd := "docker inspect --format '{{.State.Status}}' " + filetransfer.ShellQuote(containerName)
```

`{{.State.Status}}` returns the container's **lifecycle state**: `created`, `running`, `paused`, `restarting`, `removing`, `exited`, `dead`. It does NOT return the HEALTHCHECK status (`healthy`, `unhealthy`, `starting`). The `pollContainers` function switch is:
```go
case "running":
    done[container] = true
case "exited", "dead":
    return false, fmt.Errorf(...)
default:
    // continue polling
```

This means a container is considered "healthy" as soon as it enters the `running` state — even if it has a HEALTHCHECK defined that would report `starting` or `unhealthy`. The health polling provides no actual health guarantee for containers with HEALTHCHECK defined, which is the primary use case for health polling.

The package-level comment (line 73-76) correctly describes the intended behavior using `healthy`/`unhealthy`/`starting`, but the implementation uses `{{.State.Status}}` (lifecycle) instead of `{{.State.Health.Status}}` (health check status).

**Fix:**
```go
// For containers with HEALTHCHECK defined:
cmd := "docker inspect --format '{{if .State.Health}}{{.State.Health.Status}}{{else}}no-healthcheck{{end}}' " + filetransfer.ShellQuote(containerName)
```

Then handle the status values as documented in the package comment:
```go
switch status {
case "healthy", "no-healthcheck":
    done[container] = true
case "unhealthy":
    fmt.Fprintf(os.Stderr, "Health check failed: container %s is unhealthy\n", container)
    return false, fmt.Errorf("health: container %s is unhealthy", container)
default:
    // "starting" or unexpected — continue polling
}
```

---

### WR-03: `listContainers` `docker ps` command format string is unquoted — shell injection via project name label value

**File:** `internal/health/poll.go:162`

**Issue:**
```go
cmd := "docker ps --filter label=com.docker.compose.project=" + filetransfer.ShellQuote(projectName) + " --format '{{.Names}}'"
```

`ShellQuote(projectName)` wraps the project name in single quotes (e.g., `'myapp'`). The resulting command is:
```
docker ps --filter label=com.docker.compose.project='myapp' --format '{{.Names}}'
```

The label filter value is passed as `label=key=value`. In Docker's CLI flag parsing, the `=value` is parsed as part of the flag argument — single-quoting `'myapp'` is correct for shell word-splitting prevention. However, the `--format` string `'{{.Names}}'` is a shell literal that's fine. The actual concern is that `projectName` is derived from `filepath.Base(cwd)` in `main.go`, which is the directory name — a directory name can contain characters like spaces or single quotes, and `ShellQuote` does handle embedded single quotes correctly. This is lower risk than it first appears.

The real issue is more subtle: `ShellQuote('myapp')` produces `'myapp'` but the docker filter is `label=com.docker.compose.project='myapp'` — docker's filter parsing sees the key as `com.docker.compose.project` and the value as `myapp` (with quotes stripped by the shell). This is correct. However, a project name containing `=` would break the filter parsing at the Docker daemon level since Docker parses `label=key=value` by splitting at the first `=`, and `value` cannot itself contain `=` in label filter values. This is an edge case but could produce incorrect results.

**Fix:** This is acceptable as-is for the common case, but document the constraint in code:
```go
// Note: projectName is filepath.Base(cwd) — directory names containing '='
// will produce malformed label filters. This is a known Docker CLI limitation.
cmd := "docker ps --filter label=com.docker.compose.project=" + filetransfer.ShellQuote(projectName) + " --format '{{.Names}}'"
```

---

### WR-04: `sshconfig.go` SSH config parser only matches first `Host` keyword per block — multi-hostname blocks silently fail

**File:** `internal/sshconfig/sshconfig.go:70-78`

**Issue:**
```go
case "host":
    active = hostMatches(value, hostname)
```

`parts[1]` is used (`value := parts[1]`), but SSH config allows multiple patterns on a single `Host` line:
```
Host prod-server backup-server *.example.com
    IdentityFile ~/.ssh/id_ed25519
```

The parser only checks `parts[1]` (the first pattern) and ignores `parts[2:]`. If the user's `~/.ssh/config` has a multi-host `Host` block and the target hostname matches a later pattern (e.g., `parts[2]`), the `IdentityFile` for that block is silently not loaded. SSH auth falls back to the agent or default key files. In the best case, auth still succeeds via agent; in the worst case, the user gets an "SSH auth failed" error that is difficult to diagnose since their config appears correct.

**Fix:**
```go
case "host":
    active = false
    for _, pattern := range parts[1:] {
        if hostMatches(pattern, hostname) {
            active = true
            break
        }
    }
```

---

### WR-05: `dependabot.yml` does not cover Go module dependencies

**File:** `.github/dependabot.yml`

**Issue:** Dependabot is configured only for `github-actions`. The Go module dependencies (Docker CLI, cobra, crypto/ssh, testcontainers, etc.) receive no automated security or version update PRs. This is especially relevant for `golang.org/x/crypto/ssh` which is a security-sensitive dependency.

**Fix:**
```yaml
updates:
  - package-ecosystem: "github-actions"
    directory: "/"
    schedule:
      interval: "weekly"
    open-pull-requests-limit: 5

  - package-ecosystem: "gomod"
    directory: "/"
    schedule:
      interval: "weekly"
    open-pull-requests-limit: 5
```

---

## Info

### IN-01: `asKeyError` uses direct type assertion instead of `errors.As` — won't unwrap wrapped errors

**File:** `internal/ssh/knownhosts.go:98-104`

**Issue:**
```go
func asKeyError(err error, target **knownhosts.KeyError) bool {
    ke, ok := err.(*knownhosts.KeyError)
    if ok {
        *target = ke
    }
    return ok
}
```

This performs a direct type assertion rather than using `errors.As`. If the `knownhosts` package ever wraps a `*KeyError` inside another error (via `fmt.Errorf("...: %w", ke)`), the assertion will fail and the error will fall through to the `default` case in the callback, returning the raw error to the SSH dialer rather than the structured `UnknownHostError` or `KeyMismatchError`. The TOFU prompt and key-mismatch warning would silently not trigger.

Currently `golang.org/x/crypto/ssh/knownhosts` returns unwrapped `*KeyError`, so this works. But it is fragile against library updates.

**Fix:**
```go
import "errors"

func asKeyError(err error, target **knownhosts.KeyError) bool {
    return errors.As(err, target)
}
```

---

### IN-02: `Makefile` `test-ci` uses `$(eval ...)` which silently suppresses Docker context errors

**File:** `Makefile:15`

**Issue:**
```makefile
$(eval DOCKER_HOST ?= $(shell docker context inspect --format '{{(index .Endpoints "docker").Host}}' 2>/dev/null))
```

`2>/dev/null` discards any error from `docker context inspect`. If Docker is not running or the context is misconfigured, `DOCKER_HOST` is silently set to empty string, and `go test` proceeds with `DOCKER_HOST=` — testcontainers will fail to connect to the Docker daemon with an opaque error. The `/dev/null` redirect makes the root cause invisible.

**Fix:**
```makefile
test-ci:
	TESTCONTAINERS_RYUK_DISABLED=true \
	  go test -v -tags integration -timeout 15m ./integration/...
```
Let testcontainers resolve the Docker socket itself (it handles context detection internally), or remove the `2>/dev/null` suppression so Docker context errors surface clearly.

---

### IN-03: `README.md` install script uses `curl | sh` over HTTP — supply chain risk

**File:** `README.md:22-23`

**Issue:**
```markdown
curl https://raw.githubusercontent.com/webcane/docker-deploy/master/install.sh | sh
```

This is an HTTP-to-shell pipe. Even though `raw.githubusercontent.com` is served over HTTPS (the `https://` prefix is correct in the URL), the pattern is a known supply chain risk: if the `master` branch is compromised, all users who run this command get the malicious script. The script is fetched from `master` with no version pin or integrity check.

**Fix:** Add a SHA-256 checksum verification step, or at minimum document pinning by version tag:
```bash
# Pinned version (recommended):
curl -fsSL https://raw.githubusercontent.com/webcane/docker-deploy/v1.0.0/install.sh | sh

# Or with integrity check:
curl -fsSL https://raw.githubusercontent.com/.../install.sh -o install.sh
echo "<sha256sum>  install.sh" | sha256sum -c
sh install.sh
```
This is an INFO item rather than BLOCKER because the actual URL uses HTTPS, but it is worth addressing for a security-sensitive tool that handles SSH keys and `.env` files.

---

_Reviewed: 2026-05-24_
_Reviewer: Claude (gsd-code-reviewer)_
_Depth: standard_
