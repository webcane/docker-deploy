# Walking Skeleton — docker-deploy

**Phase:** 1
**Generated:** 2026-05-13

## Capability Proven End-to-End

A developer can install the `docker-deploy` binary to `~/.docker/cli-plugins/` and run `docker deploy --help` — the Docker CLI discovers the plugin, forwards the invocation, and displays usage output. The `docker-cli-plugin-metadata` handshake responds with valid JSON. The binary is cross-compiled for linux/amd64 and linux/arm64 by GoReleaser on a `v*` tag push, and CI validates every push.

## Architectural Decisions

| Decision | Choice | Rationale |
|---|---|---|
| Language | Go 1.22+ | Single binary, no runtime deps on VPS or developer machine |
| Plugin protocol | `github.com/docker/cli` `plugin.Run()` | Handles `docker-cli-plugin-metadata` handshake automatically; required by Docker CLI plugin convention |
| CLI framework | `github.com/spf13/cobra` | Required by docker/cli plugin framework integration |
| Module path | `github.com/mniedre/docker-deploy` | Canonical GitHub path for distribution |
| Directory layout | `cmd/docker-deploy/main.go` + `internal/` | Go convention; `internal/` gates future packages |
| Metadata location | Inline in `cmd/docker-deploy/main.go` | No premature abstraction in Phase 1; move to `internal/plugin/` only if it grows |
| Version injection | ldflags `-X main.version={{.Version}}`; local dev shows `dev` | Build-time injection; no runtime config file needed |
| Build/install | Makefile (`build`, `install`, `test`) | Developer muscle-memory; wraps `go build` and binary copy |
| Release targets | linux/amd64, linux/arm64 via GoReleaser | These are the VPS targets; macOS not included (devs use `make install`) |
| Release archives | `.tar.gz` per platform + `checksums.txt` | Standard GoReleaser defaults; binary named `docker-deploy` inside archive |
| CI trigger | Push to any branch + PR | Every commit tested |
| Release trigger | Manual `v*` git tag push | Explicit intent required; no accidental releases |

## Stack Touched in Phase 1

- [x] Project scaffold (Go module, `go.sum`, `.gitignore`, `Makefile`)
- [x] CLI plugin hook — `plugin.Run()` wiring + cobra root (the only "route" that matters here)
- [ ] Database — not applicable (CLI tool, no database)
- [ ] UI — not applicable (CLI tool; `--help` output is the UI)
- [x] Build pipeline — `make build` produces local binary; `make install` copies to `~/.docker/cli-plugins/`
- [x] CI/CD — GitHub Actions test workflow (every push) + release workflow (`v*` tag)

## Out of Scope (Deferred to Later Slices)

- User-facing flags (`--host`, `--path`, `--init`) — added in the phases that implement them
- SSH connectivity, config resolution — Phase 2
- File copy via SFTP — Phase 3
- `docker compose up -d` remote execution — Phase 4
- Pre-flight checks and health polling — Phase 5
- Init wizard — Phase 6
- macOS GoReleaser targets — developer machines use `make install`; VPS targets are linux only

## Subsequent Slice Plan

Each later phase adds one vertical slice on top of this skeleton without altering its architectural decisions:

- Phase 2: SSH dial (knownhosts, timeout, auth chain) + config resolution (`deploy.yaml` / flags / defaults)
- Phase 3: SFTP file copy with atomic staging-dir pattern
- Phase 4: Full deploy loop — copy + `docker compose up -d` + streaming output
- Phase 5: Pre-flight checks + post-deploy health polling
- Phase 6: `--init` wizard for first-deploy VPS setup
