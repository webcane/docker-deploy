# Phase 1: Plugin Scaffolding - Context

**Gathered:** 2026-05-13
**Status:** Ready for planning

<domain>
## Phase Boundary

The binary skeleton that makes `docker deploy --help` work inside the Docker CLI, responds correctly to `docker-cli-plugin-metadata`, and produces a CI-tested cross-platform binary. No SSH, no file copy, no business logic — the plugin interface contract is locked and the build/release pipeline is operational.

</domain>

<decisions>
## Implementation Decisions

### Module Path & Directory Layout
- **D-01:** Go module path is `github.com/mniedre/docker-deploy`
- **D-02:** Directory layout: `cmd/docker-deploy/` (contains `main.go`) + `internal/` for future packages
- **D-03:** Makefile with `make build`, `make install` (copies to `~/.docker/cli-plugins/`), `make test` targets
- **D-04:** Plugin metadata (name, version, description constants) lives inline in `cmd/docker-deploy/main.go` — no separate `internal/plugin/` package in Phase 1

### Command Structure
- **D-05:** Single root cobra command with flags only — no subcommands. All behavior via flags (`--host`, `--path`, `--init`, etc.)
- **D-06:** Cobra root command defined inline in `cmd/docker-deploy/main.go` — no `internal/cmd/` indirection in Phase 1
- **D-07:** Phase 1 stubs NO user-facing flags. Strict skeleton: only `plugin.Run()` wiring (which handles `docker-cli-plugin-metadata` automatically) and the cobra root for `--help`. Flags are added in the phases that implement them.
- **D-08:** Version reported via ldflags at build time: `-ldflags "-X main.version={{.Version}}"`. Local dev builds show `dev`.

### Platform Targets
- **D-09:** GoReleaser ships linux/amd64 and linux/arm64 only. These are the VPS targets.
- **D-10:** macOS developer machines: not in GoReleaser. Developers build locally via `go install` or `make install`.
- **D-11:** Release archives: `.tar.gz` per platform + `checksums.txt`. Binary name inside the archive: `docker-deploy` (not platform-suffixed).

### CI & Release Triggers
- **D-12:** Test workflow: runs on every push to any branch and on every pull request.
- **D-13:** Release workflow: triggered by a manual git tag push matching `v*` (e.g., `v0.1.0`). Full GitHub Release with published artifacts.
- **D-14:** Go version: latest stable (1.22+). Single version — no matrix.

### Claude's Discretion
- GitHub Actions workflow file naming and structure (e.g., `.github/workflows/ci.yml`, `release.yml`)
- GoReleaser config format details (`.goreleaser.yaml`)
- `.gitignore` contents
- `go.sum` and initial dependency pinning (beyond locking `docker/cli` version first — see canonical refs)

</decisions>

<canonical_refs>
## Canonical References

**Downstream agents MUST read these before planning or implementing.**

### Phase Scope
- `.planning/ROADMAP.md` §Phase 1 — goal, success criteria, and requirements list (PLUG-01, PLUG-02, PLUG-03)
- `.planning/REQUIREMENTS.md` §Plugin — PLUG-01, PLUG-02, PLUG-03 requirement definitions

### Project Decisions
- `.planning/PROJECT.md` §Key Decisions — locked architectural decisions (plugin.Run(), cobra, Go, config resolution strategy)
- `CLAUDE.md` §Critical Implementation Rules — implementation constraints that apply across all phases (rule 4: lock `docker/cli` version first is Phase 1 critical)
- `CLAUDE.md` §Key Technical Decisions — technology choices locked at project level

No external ADRs or specs — requirements and decisions fully captured above.

</canonical_refs>

<code_context>
## Existing Code Insights

### Reusable Assets
- None — greenfield project. No existing code.

### Established Patterns
- None yet — Phase 1 establishes the patterns.

### Integration Points
- `~/.docker/cli-plugins/docker-deploy` — the binary install path the Docker CLI discovers at startup
- `docker-cli-plugin-metadata` argv[1] — the discovery handshake handled by `plugin.Run()` from `github.com/docker/cli`

</code_context>

<specifics>
## Specific Ideas

- Strict Phase 1 scope: skeleton only. No stub flags. `plugin.Run()` wiring + cobra root = done.
- `docker/cli` version MUST be locked first (before any other dependency) per project rule — this is the first Go module dependency to pin.
- Binary name `docker-deploy` must be preserved exactly (Docker CLI plugin naming convention).

</specifics>

<deferred>
## Deferred Ideas

None — discussion stayed within phase scope.

</deferred>

---

*Phase: 1-Plugin Scaffolding*
*Context gathered: 2026-05-13*
