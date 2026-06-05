# Phase 11: CI & Tooling Polish - Context

**Gathered:** 2026-05-23
**Status:** Ready for planning

<domain>
## Phase Boundary

Four infrastructure improvements delivered in one pass:
1. **Linting & Formatting** — `make lint` (golangci-lint) + `make fmt` (goimports) added to Makefile and enforced in CI as a gate
2. **Codecov** — coverage uploaded from unit tests on every CI run; badge in README
3. **GitHub Actions hygiene** — bump all actions to current major versions, remove the `FORCE_JAVASCRIPT_ACTIONS_TO_NODE24` workaround, add Dependabot for automated future bumps
4. **Brew symlink automation** — `post_install` creates `~/.docker/cli-plugins/docker-deploy` symlink; `def uninstall` removes it; existing `caveats:` block removed

This phase adds NO new plugin features — only CI, tooling, and distribution infrastructure.

</domain>

<decisions>
## Implementation Decisions

### Linting & Formatting

- **D-01:** Add `.golangci.yml` config file checked into the repo (not inline defaults)
- **D-02:** Linter set: `errcheck`, `govet`, `staticcheck`, `goimports` (minimal, high-value, low-noise for a small CLI project)
- **D-03:** `goimports` local prefix: `github.com/webcane/docker-deploy` (groups project imports in a third group, separate from stdlib and third-party)
- **D-04:** `make lint` and `make fmt` added to Makefile; both run as a separate `lint` job in GitHub Actions CI that must pass before merge

### Codecov

- **D-05:** Tokenless upload (public repo — no `CODECOV_TOKEN` secret required)
- **D-06:** Coverage scope: unit tests only (`go test -coverprofile=coverage.out ./...` in the `test` job)
- **D-07:** Add minimal `codecov.yml` config to control PR diff coverage comments
- **D-08:** Use `codecov/codecov-action@v4` to upload `coverage.out`

### GitHub Actions Hygiene

- **D-09:** Remove `FORCE_JAVASCRIPT_ACTIONS_TO_NODE24: "true"` env var from all workflow files after real action bumps land — it was a workaround, not a permanent fix
- **D-10:** Version pinning strategy: major version tags (`@vN`) — conventional, auto-receives non-breaking updates
- **D-11:** Add `.github/dependabot.yml` for `github-actions` ecosystem with weekly schedule — keeps action versions current automatically going forward

### Homebrew Formula Symlink Automation

- **D-12:** Add `post_install` block to `.goreleaser.yaml` `brews:` entry: `mkdir -p ~/.docker/cli-plugins && ln -sf #{bin}/docker-deploy ~/.docker/cli-plugins/docker-deploy`
- **D-13:** Add `def uninstall` via `custom_block` in GoReleaser `brews:` to remove `~/.docker/cli-plugins/docker-deploy` symlink on `brew uninstall`
- **D-14:** Remove the existing `caveats:` block entirely — `post_install` handles everything; manual instructions become misleading once automation is in place

</decisions>

<canonical_refs>
## Canonical References

**Downstream agents MUST read these before planning or implementing.**

### Workflow Files
- `.github/workflows/ci.yml` — current CI structure; `test` and `integration` jobs; `FORCE_JAVASCRIPT_ACTIONS_TO_NODE24` workaround to remove
- `.github/workflows/release.yml` — release pipeline; `goreleaser-action`, `cosign-installer` versions

### Distribution Config
- `.goreleaser.yaml` — GoReleaser config; `brews:` block with current `install:`, `caveats:`, `test:` — target for `post_install` and `custom_block` additions

### Build System
- `Makefile` — current targets (`build`, `install`, `test`, `test-ci`); `make lint` and `make fmt` to be added

### No external specs
Requirements fully captured in decisions above.

</canonical_refs>

<code_context>
## Existing Code Insights

### Reusable Assets
- `Makefile`: Current targets use `go build`, `go test`, `go vet` — `make lint` and `make fmt` follow same pattern with tool invocations
- `.github/workflows/ci.yml`: `test` job already runs `go test ./...` — extend it with `-coverprofile=coverage.out` and add `codecov-action` upload step

### Established Patterns
- Actions already use major version tags (`@v4`, `@v5`, `@v6`, `@v3`) — the bump is a version number update, not a structural change
- GoReleaser `brews:` already has `install:` and `test:` Ruby blocks — `post_install` and `custom_block` follow the same YAML key pattern

### Integration Points
- Codecov badge goes in `README.md` alongside existing build/release badges (from Phase 9 documentation)
- `post_install` and `def uninstall` Ruby blocks are injected into the Homebrew formula that GoReleaser auto-pushes to `webcane/homebrew-docker-deploy` on release

</code_context>

<specifics>
## Specific Ideas

- `make fmt` should run `goimports -w -local github.com/webcane/docker-deploy ./...`
- `make lint` should run `golangci-lint run ./...`
- `.golangci.yml` should explicitly enable: `errcheck`, `govet`, `staticcheck`, `goimports` — and set `goimports.local-prefixes: github.com/webcane/docker-deploy`
- Dependabot schedule: weekly, targeting `.github/workflows/` directory

</specifics>

<deferred>
## Deferred Ideas

None — discussion stayed within phase scope.

</deferred>

---

*Phase: 11-CI & Tooling Polish*
*Context gathered: 2026-05-23*
