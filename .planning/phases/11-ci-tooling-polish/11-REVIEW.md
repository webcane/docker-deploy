# Phase 11 Code Review

**Reviewed:** 2026-05-23
**Depth:** standard
**Files Reviewed:** 8

## Summary

10 findings: 5 critical, 3 warning, 2 info

---

## Findings

### [CRITICAL] Non-existent GitHub Actions versions break all CI and release jobs

**File:** `.github/workflows/ci.yml` (lines 13, 15, 28, 30, 54, 56), `.github/workflows/release.yml` (lines 15, 19, 25)

**Issue:** Every workflow references action versions that do not exist:
- `actions/checkout@v6` — latest stable is `v4`
- `actions/setup-go@v6` — latest stable is `v5`
- `codecov/codecov-action@v6` — latest stable is `v5`
- `goreleaser/goreleaser-action@v7` — latest stable is `v6`

GitHub Actions resolves these by tag. Non-existent tags cause the runner to fail at the `uses:` step before any code runs. Every lint, test, integration, and release job will fail immediately. No code change can ship.

**Fix:** Downgrade to known-good versions:
```yaml
- uses: actions/checkout@v4
- uses: actions/setup-go@v5
  with:
    go-version-file: go.mod
- uses: codecov/codecov-action@v5
- uses: goreleaser/goreleaser-action@v6
```

---

### [CRITICAL] golangci-lint version `v1.64.8` does not exist

**File:** `.github/workflows/ci.yml` (line 20)

**Issue:**
```yaml
run: go install github.com/golangci/golangci-lint/cmd/golangci-lint@v1.64.8
```
The v1 release series peaked around `v1.61.x` before golangci-lint v2.0.0 was released in early 2025. Version `v1.64.8` was never published. `go install` with a non-existent pseudo-version will return `unknown revision v1.64.8` and the lint job will fail on every run.

**Fix:** Use the last known good v1 release or migrate to v2:
```yaml
# Option A — last v1 release
run: go install github.com/golangci/golangci-lint/cmd/golangci-lint@v1.61.0

# Option B — v2 (requires updating .golangci.yml config format)
run: go install github.com/golangci/golangci-lint/cmd/golangci-lint@v2.1.0
```

---

### [CRITICAL] CI push trigger targets `main` but the repository default branch is `master`

**File:** `.github/workflows/ci.yml` (line 5)

**Issue:**
```yaml
on:
  push:
    branches: [main]
```
The repository's default branch is `master` (confirmed by `git branch --show-current` and the git status). This trigger never fires. Direct pushes to `master` — including merge commits and hotfixes — bypass all CI checks entirely. Only `pull_request` events (which have no branch filter) will trigger jobs. There is no CI gate on the main integration branch.

**Fix:**
```yaml
on:
  push:
    branches: [master]
  pull_request:
```

---

### [CRITICAL] `def uninstall` in Homebrew formula `custom_block` is silently ignored

**File:** `.goreleaser.yaml` (lines 68-71)

**Issue:** `def uninstall` is a method on Homebrew **Cask** objects, not **Formula** objects. A generated `.rb` formula that defines `def uninstall` will either fail `brew audit` or, worse, silently define an unreachable method that Homebrew's formula runner never calls. The symlink at `~/.docker/cli-plugins/docker-deploy` will not be removed on `brew uninstall docker-deploy`.

The goreleaser `custom_block` key inserts its content verbatim into the formula class body. Homebrew's formula lifecycle hooks are `def install`, `def post_install`, `def caveats`, `test do` — there is no `def uninstall` for formulas.

**Fix:** Remove the `def uninstall` block. Document the symlink persistence as a known limitation in caveats, or use a `keg_only` workaround. If automated removal is required, the correct solution is to publish a Cask instead of a formula, which does support `uninstall`.

Alternatively, document the manual removal in caveats:
```ruby
def caveats
  <<~EOS
    To remove the Docker CLI plugin symlink on uninstall:
      rm -f ~/.docker/cli-plugins/docker-deploy
  EOS
end
```

---

### [CRITICAL] `uninstall` block uses `File.exist?` instead of `File.symlink?`

**File:** `.goreleaser.yaml` (line 70)

**Issue:** Even if `def uninstall` were valid (see prior finding), the condition is wrong:
```ruby
File.delete(symlink) if File.exist?(symlink)
```
`File.exist?` returns `true` for regular files, directories, and symlinks. If a user has manually placed a real binary at `~/.docker/cli-plugins/docker-deploy` (via `make install` or `go install`), running `brew uninstall` would silently delete that binary.

The `post_install` block correctly uses `File.symlink?` before deleting — the `uninstall` block should match:
```ruby
File.delete(symlink) if File.symlink?(symlink)
```

---

### [WARNING] `sandbox_allowlist?` is not a Homebrew Formula DSL method

**File:** `.goreleaser.yaml` (line 64)

**Issue:**
```ruby
def sandbox_allowlist?
  true
end
```
This method does not exist in Homebrew's Formula API. It will be inserted verbatim into the formula class via `custom_block`, where it becomes an unreachable dead method. `brew audit --strict` may flag this as an unknown override or unexpected method definition. The intent (opting out of macOS sandbox restrictions during `post_install`) is not achievable through this mechanism in modern Homebrew.

**Fix:** Remove `sandbox_allowlist?`. If sandbox restrictions are causing `post_install` to fail silently on macOS Ventura/Sequoia, the correct approach is to wrap the symlink creation in a `rescue` (already done for `Errno::EPERM`) and use caveats to guide users. The `sandbox_allowlist?` stub does nothing.

---

### [WARNING] `integration` job does not depend on `lint`; lint failures do not block integration

**File:** `.github/workflows/ci.yml` (line 50)

**Issue:**
```yaml
integration:
  needs: [test]
```
The `integration` job depends only on `test`. If the `lint` job fails, integration tests still execute and — if they pass — the overall workflow can succeed with lint errors. This undermines the lint gate: a PR with lint failures could still show a green CI badge if `test` and `integration` both pass.

**Fix:**
```yaml
integration:
  needs: [test, lint]
```

---

### [WARNING] `go vet` runs after coverage upload; a vet failure does not prevent partial results

**File:** `.github/workflows/ci.yml` (lines 40-47)

**Issue:** The step order in the `test` job is:
1. Build
2. Run tests (produces `coverage.out`)
3. Upload coverage to Codecov
4. Run vet

If `go vet` finds a real issue, coverage data from potentially-broken code is already uploaded to Codecov. More importantly, vet is a correctness check and should gate whether the test results are trustworthy. Running it after the fact reduces its value as a gate.

**Fix:** Move `go vet` before `go test`:
```yaml
- name: Run vet
  run: go vet ./...

- name: Run tests
  run: go test -coverprofile=coverage.out ./...

- name: Upload coverage to Codecov
  uses: codecov/codecov-action@v5
  ...
```

---

### [INFO] `dependabot.yml` does not cover Go module dependencies

**File:** `.github/dependabot.yml`

**Issue:** Dependabot is configured only for `github-actions`. The project has Go module dependencies (Docker CLI, cobra, testcontainers, etc.) that will not receive automated update PRs.

**Fix:** Add the `gomod` ecosystem:
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

### [INFO] Codecov badge in README points to `branch/master` but CI uploads from the `main` trigger

**File:** `README.md` (line 3)

**Issue:**
```markdown
[![Codecov](https://codecov.io/gh/webcane/docker-deploy/branch/master/graph/badge.svg)]
```
The CI workflow's `push` trigger is `branches: [main]` (see CRITICAL finding above). Until the branch mismatch is fixed, coverage is only uploaded from `pull_request` events — never from a named-branch push. Once the branch trigger is corrected to `master`, the badge URL will resolve correctly. This is a secondary symptom of the branch name mismatch, not an independent fix.

**Fix:** Resolve the CRITICAL branch mismatch finding first. The badge URL referencing `master` is actually correct for this repo — it is the ci.yml trigger that needs to change.
