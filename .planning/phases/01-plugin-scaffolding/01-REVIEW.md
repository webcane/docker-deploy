---
phase: 01-plugin-scaffolding
reviewed: 2026-05-14T00:00:00Z
depth: standard
files_reviewed: 7
files_reviewed_list:
  - .github/workflows/ci.yml
  - .github/workflows/release.yml
  - .gitignore
  - .goreleaser.yaml
  - Makefile
  - cmd/docker-deploy/main.go
  - go.mod
findings:
  critical: 1
  warning: 5
  info: 3
  total: 9
status: issues_found
---

# Phase 1: Code Review Report

**Reviewed:** 2026-05-14T00:00:00Z
**Depth:** standard
**Files Reviewed:** 7
**Status:** issues_found

## Summary

Phase 1 is a walking skeleton: Go module init, Docker CLI plugin wiring via `plugin.Run()`, Makefile, `.gitignore`, GoReleaser config, and GitHub Actions CI/release workflows. There is no business logic yet, and no security-sensitive code paths (no SSH, no InsecureIgnoreHostKey). The overall structure is sound for a scaffold, but there are several issues that will cause real failures before the project matures: a goreleaser v2 schema error that will break every release build, a floating goreleaser version pin that risks non-reproducible releases, CI double-execution on PRs, missing `.env` exclusion in `.gitignore` (critical given this tool copies `.env` files), and a no-op `RunE` that silently succeeds with no user feedback.

---

## Critical Issues

### CR-01: goreleaser v2 `snapshot.name_template` key is invalid — breaks snapshot builds

**File:** `.goreleaser.yaml:30-31`
**Issue:** The file declares `version: 2` (goreleaser v2 schema) at line 1, but uses the goreleaser v1 key `snapshot.name_template`. In goreleaser v2, the key was renamed to `snapshot.version_template`. This will cause a schema validation error or silently ignored config when running `goreleaser release --snapshot` or `goreleaser build --snapshot`. Any local snapshot build or CI snapshot job will fail or produce an incorrectly named artifact.

Additionally, `archives[].format` (line 17) and `archives[].format_overrides[].format` (line 21) are deprecated in goreleaser v2; the replacement is `archives[].formats` (a YAML list). The `format_overrides` entry is also redundant because `tar.gz` is already the default for Linux.

**Fix:**
```yaml
snapshot:
  version_template: "{{ .Tag }}-next"

archives:
  - formats: ["tar.gz"]
    name_template: "{{ .ProjectName }}_{{ .Os }}_{{ .Arch }}"
    # format_overrides for linux/tar.gz can be removed — it's the default
```

---

## Warnings

### WR-01: `.gitignore` does not exclude `.env` files — high risk given project purpose

**File:** `.gitignore:1-7`
**Issue:** This tool's core job is copying `.env` files to a remote VPS. The `.gitignore` has no exclusion for `.env`, `*.env`, `deploy.yaml`, or similar local configuration files. A developer who adds a `deploy.yaml` or `.env` to the repo root for testing could accidentally commit credentials. Given that CLAUDE.md flags MITM on `.env` as catastrophic, preventing `.env` from ever being committed is a required defence.

**Fix:**
```gitignore
bin/
dist/
*.exe
.DS_Store
*.log
docker-deploy

# Never commit secrets or local config
.env
*.env
deploy.yaml
```

### WR-02: `goreleaser-action` pinned to `version: latest` — non-reproducible releases

**File:** `.github/workflows/release.yml:24`
**Issue:** `version: latest` means goreleaser itself floats to whatever is current at release time. A goreleaser major version bump (e.g., v2 → v3) could silently break the schema at the next tag push with no change to source. This is especially risky because the project already has a v1→v2 schema issue (CR-01) that was introduced without noticing. Pin to an explicit goreleaser semver.

**Fix:**
```yaml
- uses: goreleaser/goreleaser-action@v6
  with:
    version: "~> v2"   # or a specific v2.x.y
    args: release --clean
```

### WR-03: CI triggers on both `push` and `pull_request` with no branch filter — double execution

**File:** `.github/workflows/ci.yml:3-5`
**Issue:** The current trigger runs the CI job twice for every pull request: once for the `push` event (when the branch is pushed) and once for the `pull_request` event. This wastes CI minutes and creates confusing duplicate status checks on PRs. The conventional fix is to restrict the `push` trigger to the main branch only, and use `pull_request` for all feature branches.

**Fix:**
```yaml
on:
  push:
    branches: [main]
  pull_request:
```

### WR-04: No `go build` step in CI — compilation errors are not caught

**File:** `.github/workflows/ci.yml:17-21`
**Issue:** CI runs `go test ./...` and `go vet ./...` but has no explicit `go build ./...` step. If the package has no test files (as is the case for phase 1 — `cmd/docker-deploy` has no `_test.go`), `go test ./...` will not compile the main binary. A build failure in `main.go` would not be caught. `go vet` does compile code but only reports vet diagnostics, not build errors. An explicit build step is the standard guard.

**Fix:**
```yaml
- name: Build
  run: go build ./...

- name: Run tests
  run: go test ./...

- name: Run vet
  run: go vet ./...
```

### WR-05: `cmd/docker-deploy/main.go` `RunE` is a silent no-op with no user feedback

**File:** `cmd/docker-deploy/main.go:17-19`
**Issue:** The skeleton `RunE` returns `nil` without printing anything. When a user runs `docker deploy` against this binary, the command exits 0 with no output — no "not implemented yet" message, no usage hint. This is acceptable for a private scaffold but will create confusion when the binary is installed and invoked. A placeholder message prevents silent surprise, and the pattern should be established now so implementors don't accidentally ship the no-op.

**Fix:**
```go
RunE: func(cmd *cobra.Command, args []string) error {
    return fmt.Errorf("not implemented yet")
},
```

---

## Info

### IN-01: `goreleaser.yaml` archives `format_overrides` for `linux/tar.gz` is redundant

**File:** `.goreleaser.yaml:19-21`
**Issue:** The `format_overrides` block sets `format: tar.gz` for `goos: linux`, which is already the default format. The block adds noise without effect and will still need to be updated when fixing the `format` → `formats` schema issue (WR-01/CR-01).

**Fix:** Remove the `format_overrides` block entirely:
```yaml
archives:
  - formats: ["tar.gz"]
    name_template: "{{ .ProjectName }}_{{ .Os }}_{{ .Arch }}"
```

### IN-02: `release.yml` uses `actions/checkout@v4` without `fetch-depth: 0` awareness note

**File:** `.github/workflows/release.yml:14-16`
**Issue:** `fetch-depth: 0` is correctly set for GoReleaser (it needs full tag history to compute changelogs and version). This is good. No action needed — flagged only to acknowledge it was checked.

### IN-03: `Makefile` `build` target does not embed `CGO_ENABLED=0` — binary may have libc dependency

**File:** `Makefile:4-5`
**Issue:** The `go build` invocation does not set `CGO_ENABLED=0`. For a binary intended to run on arbitrary Linux VPS targets, a CGO-linked binary may fail if the target glibc version differs. The goreleaser config similarly omits `CGO_ENABLED=0`. This is not critical for phase 1 (no CGO code exists yet), but the default should be established before any dependency that can pull in CGO is added.

**Fix:**
```makefile
build:
	mkdir -p bin
	CGO_ENABLED=0 go build -ldflags "-X main.version=dev" -o bin/docker-deploy ./cmd/docker-deploy/
```

And in `.goreleaser.yaml`:
```yaml
builds:
  - main: ./cmd/docker-deploy
    binary: docker-deploy
    env:
      - CGO_ENABLED=0
    ldflags:
      - -s -w -X main.version={{.Version}}
```

---

_Reviewed: 2026-05-14T00:00:00Z_
_Reviewer: Claude (gsd-code-reviewer)_
_Depth: standard_
