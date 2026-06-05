---
phase: 1
phase_name: "plugin-scaffolding"
project: "docker-deploy"
generated: "2026-06-05"
counts:
  decisions: 6
  lessons: 4
  patterns: 4
  surprises: 3
missing_artifacts: []
---

# Phase 1 Learnings: Plugin Scaffolding

## Decisions

### docker/cli dependency pinned first before all other dependencies
Lock `github.com/docker/cli` via `go get` as the very first step before adding any other module. The plugin protocol contract (import paths, metadata types, `plugin.Run()` signature) must be established before business logic is layered on.

**Rationale:** Transitive dependency conflicts from `docker/cli` are difficult to resolve after the fact. Locking it first prevents version drift with downstream packages (cobra, moby, containerd, etc.).
**Source:** 01-PLAN-01.md, 01-01-SUMMARY.md

---

### Import path uses double cli/ prefix: github.com/docker/cli/cli/command
The `command.Cli` interface lives at `github.com/docker/cli/cli/command` (double `cli/`), not `github.com/docker/cli/command` as one might expect from the module name.

**Rationale:** The module path and internal package layout differ; this is a quirk of the docker/cli repo structure. Confirmed by inspecting the module cache after `go get`.
**Source:** 01-01-SUMMARY.md

---

### Metadata type lives in github.com/docker/cli/cli-plugins/metadata (not manager)
In docker/cli v29.4.3, the plugin `Metadata` struct is in the `metadata` sub-package, not `manager` as older documentation suggests.

**Rationale:** Package layout changed across docker/cli major versions. Must verify actual import paths against the installed version.
**Source:** 01-01-SUMMARY.md

---

### GoReleaser targets linux/amd64 and linux/arm64 only — no darwin/windows
Release artifacts are for VPS deployment targets only. macOS and Windows binaries are explicitly excluded.

**Rationale:** This is a server-side deploy tool; developers build from source (`make install`). Including darwin/windows would bloat releases without benefit.
**Source:** 01-PLAN-02.md, 01-02-SUMMARY.md

---

### go-version-file instead of hardcoded Go version in CI
GitHub Actions workflows use `go-version-file: go.mod` in the `actions/setup-go` step so the Go version is read from the `go` directive in go.mod rather than duplicated in workflow YAML.

**Rationale:** Single source of truth for the Go version; updating go.mod automatically propagates to CI.
**Source:** 01-PLAN-02.md, 01-02-SUMMARY.md

---

### GITHUB_TOKEN scoped to contents:write for release job only
The release workflow grants `permissions: contents: write` to the job, not the entire workflow. The CI workflow has no elevated permissions.

**Rationale:** Principle of least privilege; limits blast radius if the release workflow is ever exploited.
**Source:** 01-PLAN-02.md, 01-02-SUMMARY.md

---

## Lessons

### go mod tidy resolves 40+ transitive dependencies for docker/cli +incompatible automatically
`github.com/docker/cli v29.4.3+incompatible` is a pre-modules package that does not ship its own go.mod. Creating the first source file that imports it causes `go build` to fail with 30+ missing package errors. Running `go mod tidy` after the first import resolves all transitive deps automatically.

**Context:** The `+incompatible` suffix indicates a pre-modules release. These require an extra `go mod tidy` step that ordinary modules do not.
**Source:** 01-01-SUMMARY.md

---

### go build ./cmd/... produces a root-level binary unless -o is specified
Running `go build ./cmd/docker-deploy/` without `-o bin/docker-deploy` places the binary at the repo root. This untracked binary must be explicitly added to .gitignore to avoid accidental commits.

**Context:** Discovered when Task 3 noticed a root-level `docker-deploy` binary after Task 2's verification step.
**Source:** 01-01-SUMMARY.md

---

### Go toolchain must be installed as an explicit prerequisite
The developer machine did not have Go installed. The plan assumed Go was available. Go had to be installed via Homebrew before any plan tasks could execute.

**Context:** Bootstrapping constraint: Go is not pre-installed on all macOS developer machines. Future plans or onboarding docs should call this out.
**Source:** 01-01-SUMMARY.md

---

### fetch-depth: 0 is required for GoReleaser changelog generation
GoReleaser needs the full git history to generate changelogs and determine version tags. The default `actions/checkout` shallow clone (`fetch-depth: 1`) will cause GoReleaser to fail or produce incomplete output.

**Context:** This is a GoReleaser requirement, not a general GitHub Actions concern. Must be set explicitly on the checkout step in the release workflow.
**Source:** 01-PLAN-02.md

---

## Patterns

### Docker CLI plugin registration via plugin.Run()
Wire the cobra root command into `plugin.Run()`. The framework handles `docker-cli-plugin-metadata` argv automatically — no manual JSON printing needed. Binary naming (`docker-deploy`) and install location (`~/.docker/cli-plugins/`) are the only other requirements for Docker CLI discovery.

**When to use:** Any Docker CLI plugin built with `github.com/docker/cli/cli-plugins/plugin`.
**Source:** 01-PLAN-01.md, 01-01-SUMMARY.md

---

### ldflags version injection via -X main.version
Declare `var version = "dev"` at package scope and override it at build time with `-ldflags "-X main.version=<tag>"`. GoReleaser uses `-X main.version={{.Version}}` to inject the git tag automatically.

**When to use:** Any Go binary that needs a build-time version string visible at runtime.
**Source:** 01-PLAN-01.md, 01-PLAN-02.md

---

### Makefile with build/install/test as the developer UX contract
Three targets cover the full local development loop: `build` (produces `bin/docker-deploy`), `install` (copies to `~/.docker/cli-plugins/`), `test` (runs `go test ./...`). Using `install -m 755` and creating the plugin dir if absent makes the install target idempotent.

**When to use:** Go CLI tools that need a local install step to integrate with a host tool (Docker, git, etc.).
**Source:** 01-PLAN-01.md, 01-01-SUMMARY.md

---

### Separate CI and release workflows with distinct triggers
CI fires on every push and pull_request (all branches). Release fires only on `v*` tag pushes. Keeping them in separate workflow files prevents accidental cross-triggering and makes permissions easier to scope.

**When to use:** Any project where build/test frequency differs from release frequency.
**Source:** 01-PLAN-02.md, 01-02-SUMMARY.md

---

## Surprises

### docker/cli v29.4.3 is a +incompatible module despite being a recent release
Despite being a 2024+ release, `github.com/docker/cli v29.4.3` carries the `+incompatible` suffix because the repo has not adopted Go modules for its own go.mod. This is unexpected for a current major-version release.

**Impact:** Requires an extra `go mod tidy` after first import to pull in transitive deps; cannot rely on the module graph alone.
**Source:** 01-01-SUMMARY.md

---

### Plan specified wrong import path for command.Cli
The plan document specified `github.com/docker/cli/command` for the `command.Cli` interface but the actual import path is `github.com/docker/cli/cli/command` (with the extra `cli/` path component). Build failed until the correct path was found by inspecting the module cache.

**Impact:** Required a deviation and module cache inspection during execution. Future interface blocks in plans should be validated against the installed module version, not assumed from docs.
**Source:** 01-01-SUMMARY.md

---

### make test exits 0 with no test output on a skeleton
`go test ./...` reports no test files on the Phase 1 skeleton (only main.go exists, no `_test.go` files). This is expected but could be mistaken for a test infrastructure failure. The exit code 0 is the correct signal.

**Impact:** No functional impact; important to document as expected behavior so future plans do not incorrectly flag the CI run as broken.
**Source:** 01-VERIFICATION.md
