# Phase 1: Plugin Scaffolding - Discussion Log

> **Audit trail only.** Do not use as input to planning, research, or execution agents.
> Decisions are captured in CONTEXT.md — this log preserves the alternatives considered.

**Date:** 2026-05-13
**Phase:** 1-Plugin Scaffolding
**Areas discussed:** Module path & layout, Command structure, Platform targets, CI & release triggers

---

## Module Path & Layout

| Option | Description | Selected |
|--------|-------------|----------|
| github.com/mniedre/docker-deploy | Matches git user and repo name. Standard for personal/OSS projects. | ✓ |
| github.com/docker-deploy/docker-deploy | Org-style path — useful if planning a GitHub org. | |
| You decide | Claude picks a reasonable default. | |

**User's choice:** `github.com/mniedre/docker-deploy`

---

| Option | Description | Selected |
|--------|-------------|----------|
| cmd/docker-deploy/ + internal/ | main.go in cmd/docker-deploy/, business logic in internal/. Standard Go layout for CLI tools. | ✓ |
| Flat (main.go at root) | Single-package binary. Fine for small tools. | |
| cmd/ + pkg/ (public packages) | Use pkg/ instead of internal/ for externally importable packages. | |

**User's choice:** `cmd/docker-deploy/ + internal/`

---

| Option | Description | Selected |
|--------|-------------|----------|
| Yes — make install, make build, make test | `make install` copies binary to ~/.docker/cli-plugins/ for local testing. | ✓ |
| No — go build / go test only | Keep it minimal. | |

**User's choice:** Makefile with `make install`, `make build`, `make test`

---

| Option | Description | Selected |
|--------|-------------|----------|
| Inline in cmd/docker-deploy/main.go | Simpler for Phase 1 — can be extracted later if it grows. | ✓ |
| internal/plugin/ package | Separate package from day one. | |

**User's choice:** Inline in `cmd/docker-deploy/main.go`

---

## Command Structure

| Option | Description | Selected |
|--------|-------------|----------|
| Single root command, flags only | All behavior via flags. PROJECT.md examples all use this pattern. | ✓ |
| Subcommands (deploy, init, status) | More extensible but adds cobra nesting now. | |

**User's choice:** Single root command with flags only

---

| Option | Description | Selected |
|--------|-------------|----------|
| cmd/docker-deploy/main.go directly | Cobra root inline in main. Idiomatic for small CLI tools. | ✓ |
| internal/cmd/ package from day one | Separate cmd package. Cleaner for testing but adds indirection in Phase 1. | |

**User's choice:** Cobra root inline in `cmd/docker-deploy/main.go`

---

| Option | Description | Selected |
|--------|-------------|----------|
| --host (stub) | SSH target URI — declare now for --help discoverability. | |
| --path (stub) | Remote deploy directory override. | |
| --init (stub) | Init wizard flag. | |
| None — only --help and version | Strict Phase 1 scope: skeleton only, no stub flags. | ✓ |

**User's choice:** No stub flags for Phase 1. Strict skeleton. Also noted: `docker-cli-plugin-metadata` is handled by `plugin.Run()` (framework), not cobra.

---

| Option | Description | Selected |
|--------|-------------|----------|
| ldflags at build time | GoReleaser injects `-ldflags "-X main.version={{.Version}}"`. Local builds show `dev`. | ✓ |
| Hardcoded v0.1.0 | Simple but requires manual bumps. | |

**User's choice:** ldflags at build time

---

## Platform Targets

| Option | Description | Selected |
|--------|-------------|----------|
| linux/amd64 | Most VPS targets. Essential. | ✓ |
| linux/arm64 | ARM VPS (Graviton, Oracle Free Tier). Increasingly common. | ✓ |
| darwin/amd64 + darwin/arm64 | macOS developer machines. | |
| windows/amd64 | Windows dev machines. | |

**User's choice:** linux/amd64 + linux/arm64 only

---

| Option | Description | Selected |
|--------|-------------|----------|
| Checksums + tarballs | .tar.gz per platform + checksums.txt. | ✓ |
| Raw binaries only | No archives, just the binary files. | |

**User's choice:** Checksums + tarballs

---

| Option | Description | Selected |
|--------|-------------|----------|
| go install + make install | Developers build locally from source. | ✓ |
| Add darwin to GoReleaser too | Ship darwin binaries in releases. | |
| Homebrew tap (future) | Note as a v2 concern. | |

**User's choice:** macOS users use `go install` or `make install`

---

| Option | Description | Selected |
|--------|-------------|----------|
| docker-deploy (no platform suffix) | Dropped directly into ~/.docker/cli-plugins/. | ✓ |
| docker-deploy_{{ .Os }}_{{ .Arch }} | Platform-suffixed. Users rename on install. | |

**User's choice:** Binary name `docker-deploy` (no suffix)

---

## CI & Release Triggers

| Option | Description | Selected |
|--------|-------------|----------|
| Push to any branch + PRs | Tests run on every push and every pull request. | ✓ |
| PRs only | Tests only run when a PR is opened/updated. | |
| Push to main only | Tests only run on merges to main. | |

**User's choice:** Push to any branch + PRs

---

| Option | Description | Selected |
|--------|-------------|----------|
| Git tag push (v*) | Release only on semver tags. Standard GoReleaser pattern. | |
| Push to main | Every merge to main produces a release. | (initial answer) |
| Manual workflow dispatch | Release triggered manually in GitHub Actions UI. | |

**User's choice:** Initially chose "Push to main", then reconsidered → **Manual semver tags (v*)** after exploring the version strategy implications.

---

| Option | Description | Selected |
|--------|-------------|----------|
| Full release with GitHub Release | Each push creates a tagged GitHub Release with binaries. | ✓ |
| Snapshot build only | Builds binaries but doesn't publish a GitHub Release. | |

**User's choice:** Full release with published GitHub Release

---

| Option | Description | Selected |
|--------|-------------|----------|
| Latest stable (1.22+) | Single version, always current. | ✓ |
| Matrix: latest + previous stable | Test on two Go versions. | |

**User's choice:** Latest stable (1.22+)

---

| Option | Description | Selected |
|--------|-------------|----------|
| Date-based tags (v0.0.YYYYMMDD) | CI auto-creates tag on each push. | |
| Commit SHA as pre-release | Base version + git SHA. | |
| Manual tags only — switch to tag-based | Release only when manually pushing a semver tag. | ✓ |
| You decide | Claude picks. | |

**User's choice:** Manual semver tags only (switched release trigger to tag-based)

---

## Claude's Discretion

- GitHub Actions workflow file naming and structure
- GoReleaser config format details (`.goreleaser.yaml`)
- `.gitignore` contents
- `go.sum` and initial dependency pinning details (beyond locking `docker/cli` version first)

## Deferred Ideas

None — discussion stayed within phase scope.
