---
status: complete
phase: 11-ci-tooling-polish
source: 11-01-SUMMARY.md, 11-02-SUMMARY.md, 11-03-SUMMARY.md, 11-04-SUMMARY.md
started: 2026-05-24T00:00:00Z
updated: 2026-05-24T12:00:00Z
---

## Current Test
<!-- OVERWRITE each test - shows where we are -->

[testing complete]

## Tests

### 1. README Codecov Badge URL
expected: Open README.md and find the Codecov badge. The badge URL must reference `/branch/master/` (not `/branch/main/`).
result: pass

### 2. CI Coverage Upload Configured
expected: In `.github/workflows/ci.yml`, the test job runs `go test -coverprofile=coverage.out ./...` and has a Codecov upload step using `codecov/codecov-action@v4` with `fail_ci_if_error: false` and no `token:` key.
result: pass

### 3. codecov.yml Suppresses Noisy PR Comments
expected: `codecov.yml` exists at repo root with `comment.require_changes: true` (suppresses comments when coverage hasn't changed) and `coverage.status.project.default.target: auto`.
result: pass

### 4. FORCE_JAVASCRIPT_ACTIONS_TO_NODE24 Removed
expected: Running `grep -r FORCE_JAVASCRIPT_ACTIONS_TO_NODE24 .github/` returns no output — the workaround is gone from all workflow files.
result: pass

### 5. Dependabot Weekly GitHub Actions Updates
expected: `.github/dependabot.yml` exists with `package-ecosystem: github-actions`, `directory: "/"`, and `schedule: weekly`. `open-pull-requests-limit: 5`.
result: pass

### 6. make lint Passes
expected: Running `make lint` from the project root executes golangci-lint on the codebase and exits 0 (no lint errors, or only expected ones).
result: pass

### 7. make fmt Runs
expected: Running `make fmt` executes `goimports -w -local github.com/webcane/docker-deploy ./...` without error.
result: issue
reported: "stat ./...: no such file or directory / make: *** [fmt] Error 2"
severity: major
fixed: "Changed Makefile fmt target to use find . -name '*.go' | xargs goimports -w -local ..."

### 8. CI Lint Job Runs Parallel with Test
expected: In `.github/workflows/ci.yml`, a `lint` job exists that installs `golangci-lint@v1.64.8` and runs `make lint`. The job has no `needs:` key (runs in parallel with `test`, not after it).
result: pass

### 9. Goreleaser Homebrew Post-Install Creates Symlink
expected: `.goreleaser.yaml` brews block has a `post_install` that creates `~/.docker/cli-plugins/docker-deploy` symlink. It includes `begin/rescue Errno::EPERM` fallback that prints an `opoo` warning on macOS Tahoe instead of failing.
result: issue
reported: "symlink is missing after brew install. and no fallback message to make manually"
severity: major

### 10. Goreleaser Homebrew Uninstall Removes Symlink
expected: `.goreleaser.yaml` brews block has a `custom_block` containing `def uninstall` that calls `File.delete` with a `File.exist?` guard to remove the symlink on `brew uninstall`.
result: issue
reported: "custom_block with def uninstall is missing from .goreleaser.yaml; caveats block still present despite plan to remove it"
severity: major

### 11. README Homebrew Section Has No Manual Symlink Instructions
expected: The Homebrew section in README.md no longer contains manual symlink instructions (no `mkdir -p ~/.docker/cli-plugins` or `ln -s` commands under the Homebrew heading).
result: pass

## Summary

total: 11
passed: 8
issues: 3
pending: 0
skipped: 0
blocked: 0

## Gaps

- truth: "make fmt runs goimports across the codebase without error"
  status: failed
  reason: "User reported: stat ./...: no such file or directory / make: *** [fmt] Error 2"
  severity: major
  test: 7
  root_cause: "goimports does not support the ./... pattern — that is a go tool convention. The Makefile fmt target passes ./... directly to goimports which treats it as a literal path."
  artifacts:
    - path: "Makefile"
      issue: "fmt target uses `goimports -w ... ./...` which fails; should use find or shell expansion"
  missing:
    - "Change fmt target to: find . -name '*.go' | xargs goimports -w -local github.com/webcane/docker-deploy"
  debug_session: ""

- truth: "brew install creates ~/.docker/cli-plugins/docker-deploy symlink (or prints actionable opoo fallback on Tahoe)"
  status: failed
  reason: "User reported: symlink is missing after brew install. and no fallback message to make manually"
  severity: major
  test: 9
  root_cause: "post_install EPERM fallback fires on macOS Tahoe (expected), but the opoo warning message in the brew output was not prominent enough for the user. The plugin is unusable without manual ln -sf step."
  artifacts:
    - path: ".goreleaser.yaml"
      issue: "post_install has correct EPERM fallback but caveats: block still present (should be removed per plan)"
  missing:
    - "Remove caveats: block from .goreleaser.yaml brews section"
  debug_session: ""

- truth: "brew uninstall removes ~/.docker/cli-plugins/docker-deploy symlink via custom_block def uninstall"
  status: failed
  reason: "User reported: custom_block with def uninstall is missing from .goreleaser.yaml; caveats block still present despite plan to remove it"
  severity: major
  test: 10
  root_cause: "custom_block with def uninstall was not added to .goreleaser.yaml. The uninstall hook is completely absent."
  artifacts:
    - path: ".goreleaser.yaml"
      issue: "brews block missing custom_block with def uninstall; caveats: block not removed"
  missing:
    - "Add custom_block to brews section with def sandbox_allowlist? and def uninstall using File.delete/File.exist?"
    - "Remove caveats: block"
  debug_session: ""
