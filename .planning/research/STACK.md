# Technology Stack

**Project:** docker-deploy (Docker CLI plugin)
**Researched:** 2026-05-13

---

## Recommended Stack

### Language Runtime

| Technology | Version | Purpose | Why |
|------------|---------|---------|-----|
| Go | 1.25.x | Implementation language | Mandated by project. Single static binary, no runtime on target VPS, standard for Docker ecosystem tooling. Go 1.25 (Aug 2025) adds DWARF5 for smaller binaries. |

Use `go 1.24` as the minimum in `go.mod` (current LTS pair is 1.24/1.25). Set `GOFLAGS=-trimpath` and `CGO_ENABLED=0` to guarantee a fully static binary — critical for distributing a Docker plugin.

---

### Docker CLI Plugin Framework

| Library | Version | Purpose | Why |
|---------|---------|---------|-----|
| `github.com/docker/cli` | v29.4.3 (track latest) | Plugin protocol, metadata, DockerCli instance | Required to integrate as a proper `docker` subcommand. Provides `plugin.Run()`, `plugin.RunPlugin()`, and `metadata.Metadata`. The docker/compose plugin is the canonical reference. |
| `github.com/spf13/cobra` | v1.10.2 | Command/flag parsing | docker/cli already depends on cobra. Every real Docker CLI plugin uses it. Required by the `plugin.Run(makeCmd, meta)` signature — makeCmd returns a `*cobra.Command`. |

**Plugin protocol details (HIGH confidence — verified against docker/cli source):**

The binary must be named `docker-deploy` and placed in `~/.docker/cli-plugins/` (user) or `/usr/lib/docker/cli-plugins/` (system). Plugin names must match `^[a-z][a-z0-9]*$`.

When Docker invokes the plugin binary with the first argument `docker-cli-plugin-metadata`, the binary must print a JSON metadata blob and exit. The `plugin.Run()` helper handles this automatically — it inspects `os.Args[1]` and short-circuits to print the metadata when that sentinel is present.

Minimum `main.go` shape:

```go
func main() {
    plugin.Run(func(dockerCli command.Cli) *cobra.Command {
        return newDeployCommand(dockerCli)
    }, manager.Metadata{
        SchemaVersion:    "0.1.0",
        Vendor:           "docker-deploy",
        Version:          version,
        ShortDescription: "Deploy docker-compose projects to a remote VPS over SSH",
    })
}
```

Plugins that use cobra's `PersistentPreRun*` hooks **must** call `plugin.PersistentPreRunE` — failing to do so breaks context propagation in newer Docker CLI versions.

**`RunningStandalone()`** returns true when the binary is invoked directly (not through `docker`). Use this to provide a helpful error message instead of a cryptic failure.

---

### SSH Transport

| Library | Version | Purpose | Why |
|---------|---------|---------|-----|
| `golang.org/x/crypto` | v0.51.0 | SSH client, known_hosts verification | The canonical, stdlib-adjacent SSH implementation used by all serious Go SSH tooling. Provides `ssh.Dial`, auth methods, and the `ssh/knownhosts` subpackage for host key verification. No alternative is more trusted. |
| `github.com/pkg/sftp` | v1.13.10 | Remote file transfer (SFTP subsystem) | Stable, actively maintained (Oct 2025 release), mirrors the `os` package API. Used for copying compose files to the remote. Prefer SFTP over SCP — SFTP is a proper protocol with directory listing, resume-from-break, and error propagation. SCP is deprecated in OpenSSH 9+. |

**Why `golang.org/x/crypto/ssh` directly, not a wrapper like `goph`:**

`goph` (v1.5.0, last meaningful tag in 2023 though commits continue into 2026) is a thin convenience wrapper. It's fine for simple use cases but this plugin needs fine-grained control over:
- SSH agent forwarding (`golang.org/x/crypto/ssh/agent`)
- Known_hosts handling with interactive "first-connect" prompts
- Session multiplexing (single TCP connection for multiple exec + SFTP)
- Connection timeout and keepalive tuning

Using `golang.org/x/crypto/ssh` directly avoids fighting a wrapper's abstractions for these requirements. `pkg/sftp` is then layered on top via `sftp.NewClient(sshConn)`.

**Auth method priority (what to implement):**

1. SSH agent via `$SSH_AUTH_SOCK` — zero-config for users who already have `ssh-add`'d their key
2. `~/.ssh/id_ed25519` / `~/.ssh/id_rsa` — explicit key file fallback
3. `--identity`/`-i` flag — explicit key path
4. Password fallback (only for `--init` root access, never for normal deploys)

**Host key verification:** Use `golang.org/x/crypto/ssh/knownhosts` to build a `HostKeyCallback` from `~/.ssh/known_hosts`. On first connect to an unknown host, prompt the user interactively (like OpenSSH does) rather than auto-accepting. **Never ship with `InsecureIgnoreHostKey()`** — even in dev mode this is a supply-chain risk for a deployment tool.

Security note: CVE-2024-45337 and CVE-2025-22869 were patched in recent `golang.org/x/crypto` releases. Pin to v0.38.0+ and keep updated.

---

### Configuration

| Library | Version | Purpose | Why |
|---------|---------|---------|-----|
| `github.com/spf13/viper` | v1.21.0 | Config file parsing (`deploy.yaml`) | Already in-ecosystem with cobra (same author). Handles YAML + environment variable overlay + flag binding in ~10 lines. Supports `SetConfigFile("deploy.yaml")` for project-local config. |

**Config precedence order** (flags win over file, file wins over defaults):

```
CLI flags > deploy.yaml > environment variables > built-in defaults
```

Viper's `BindPFlags(cmd.Flags())` after cobra flag definition wires this automatically.

**`deploy.yaml` shape (v1):**

```yaml
host: ssh://deploy@vps.example.com:22
remote_dir: /opt/myapp
include:
  - compose.yaml
  - .env
  - Makefile
```

---

### Testing

| Library | Version | Purpose | Why |
|---------|---------|---------|-----|
| `github.com/testcontainers/testcontainers-go` | latest stable | Integration tests with real SSH daemon | Spins up an `openssh-server` container, runs actual SSH/SFTP transfers. Eliminates mock drift — tests the real protocol, not a fake. Essential for a tool where the entire value is SSH correctness. |
| stdlib `testing` | — | Unit tests | No test framework needed. Table-driven tests with `t.Run` cover pure logic (path building, config parsing, file filter matching). |

**Testing strategy:**

- Unit tests (no network): config parsing, file include/exclude logic, SSH URI parsing, remote path construction.
- Integration tests (Docker required): use testcontainers to spin up `lscr.io/linuxserver/openssh-server` or `rastasheep/ubuntu-sshd`, run actual deploys, assert remote file state via a second SSH exec.
- Integration tests are opt-in behind `//go:build integration` build tag so `go test ./...` stays fast.

Mock SSH servers (`crypto/ssh` in-process server) are useful for error-path testing (auth failure, permission denied, disk full) without Docker.

---

### Build and Distribution

| Tool | Version | Purpose | Why |
|------|---------|---------|-----|
| GoReleaser | latest | Cross-compile, release, checksum | Standard Go release tooling. Produces `linux/amd64`, `linux/arm64`, `darwin/amd64`, `darwin/arm64` binaries. Integrates with GitHub Releases. Plugin users `curl | tar | mv` the binary into `~/.docker/cli-plugins/`. |
| `Makefile` | — | Developer workflow | `make build`, `make install` (copies to `~/.docker/cli-plugins/docker-deploy`), `make test`, `make lint`. Keeps CI and local dev identical. |

**Binary naming in GoReleaser:**

```yaml
builds:
  - binary: docker-deploy
    goos: [linux, darwin]
    goarch: [amd64, arm64]
    env:
      - CGO_ENABLED=0
    ldflags:
      - -s -w -X main.version={{.Version}}
```

The `-s -w` strips symbol table and DWARF debug info from release builds, cutting binary size by ~30%. Users who need debug symbols build from source.

---

## Alternatives Considered

| Category | Recommended | Alternative | Why Not |
|----------|-------------|-------------|---------|
| SSH library | `golang.org/x/crypto/ssh` | `goph` | Goph is a convenience wrapper that loses control over agent forwarding, known_hosts UX, and session reuse. Fine for simple scripts, not for a production deploy tool. |
| File transfer | `pkg/sftp` (SFTP) | `go-scp` (bramvdbogaerde) | SCP is deprecated in OpenSSH 9+ (2022). SFTP is the modern replacement. `pkg/sftp` has MkdirAll, Walk, and stable API. SCP wrappers require spawning `scp` subprocess or re-implementing the protocol. |
| File transfer | `pkg/sftp` v1.13.10 | `pkg/sftp` v2 alpha | v2 is in alpha (v2.0.0-alpha2, May 2024). Not production-ready. Use v1.13.10 stable. |
| Config | `viper` | Plain `gopkg.in/yaml.v3` | Viper adds flag binding and env overlay for free. For a CLI tool the flag+file+env merge is exactly what's needed. Pure yaml.v3 requires hand-rolling precedence. |
| CLI framework | `cobra` | `urfave/cli` | docker/cli already imports cobra. Mixing frameworks adds dependency noise and breaks the plugin framework's assumption that `makeCmd` returns `*cobra.Command`. |
| Plugin integration | `github.com/docker/cli` plugin framework | Standalone binary (no plugin) | Project requirement is `docker deploy` UX. Without the plugin framework, users run `docker-deploy` separately, losing shell completion, docker config inheritance, and the integrated help system. |
| Testing | testcontainers-go | Mock SSH server | Mock SSH eliminates the real protocol. A deploy tool's correctness is entirely in the SSH/SFTP interactions. testcontainers validates what actually matters. |

---

## Module Setup

```bash
# Initialize module
go mod init github.com/<user>/docker-deploy

# Core dependencies
go get github.com/docker/cli@latest
go get github.com/spf13/cobra@v1.10.2
go get github.com/spf13/viper@v1.21.0
go get golang.org/x/crypto@latest
go get github.com/pkg/sftp@v1.13.10

# Dev/test dependencies
go get github.com/testcontainers/testcontainers-go@latest
```

**`go.mod` minimum Go version:**

```
go 1.24
```

Set to 1.24 (not 1.25) so users on stable distros can build from source. CI runs on 1.25.

---

## Directory Layout

```
docker-deploy/
  cmd/
    docker-deploy/
      main.go           # plugin.Run() entry point
  internal/
    deploy/             # core deploy orchestration
    ssh/                # SSH/SFTP client wrapper
    config/             # viper config loading
    files/              # include/exclude filter, local file walk
  deploy.yaml           # example project config (not committed by default)
  .goreleaser.yaml
  Makefile
  go.mod
  go.sum
```

---

## Sources

- Docker CLI plugin API: https://pkg.go.dev/github.com/docker/cli/cli-plugins/plugin
- Docker CLI plugin architecture: https://deepwiki.com/docker/cli/3-plugin-architecture
- Docker CLI plugin naming/install: https://github.com/docker/cli/issues/1534
- golang.org/x/crypto/ssh: https://pkg.go.dev/golang.org/x/crypto/ssh
- golang.org/x/crypto/ssh/knownhosts: https://pkg.go.dev/golang.org/x/crypto/ssh/knownhosts
- CVE-2024-45337, CVE-2025-22869: https://copyprogramming.com/howto/golang-ssh-getting-must-specify-hoskeycallback-error-despite-setting-it-to-nil
- pkg/sftp v1.13.10: https://pkg.go.dev/github.com/pkg/sftp
- pkg/sftp releases: https://github.com/pkg/sftp/releases
- goph SSH client: https://github.com/melbahja/goph
- cobra v1.10.2: https://github.com/spf13/cobra/releases
- viper v1.21.0: https://github.com/spf13/viper
- testcontainers-go: https://golang.testcontainers.org/
- GoReleaser: https://goreleaser.com/customization/builds/go/
- Go 1.25 release notes: https://go.dev/doc/go1.25
