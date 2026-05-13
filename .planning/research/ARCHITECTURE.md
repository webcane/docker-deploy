# Architecture Patterns

**Domain:** Docker CLI plugin — SSH-based docker-compose deployment tool
**Researched:** 2026-05-13
**Confidence:** HIGH (Docker plugin protocol verified against docker/cli source; SSH/SFTP patterns verified against golang.org/x/crypto/ssh and pkg/sftp docs)

---

## Recommended Architecture

```
cmd/docker-deploy/
  main.go                  ← thin: plugin.Run() + metadata only

internal/
  cli/
    root.go                ← cobra root command, flag definitions
    deploy.go              ← deploy subcommand wiring
    init.go                ← --init wizard subcommand (isolated)
  config/
    config.go              ← deploy.yaml schema + loader
    resolve.go             ← flag > deploy.yaml > defaults resolution
  ssh/
    client.go              ← SSH connection lifecycle (Dial, Close)
    exec.go                ← remote command execution (NewSession per command)
    auth.go                ← key parsing, ssh-agent integration, knownhosts
  sftp/
    copy.go                ← file copy orchestration
    filter.go              ← include/exclude logic
    upload.go              ← single-file SFTP upload
  preflight/
    checks.go              ← check interface + pipeline runner
    docker.go              ← is Docker installed? user in docker group?
    root_warning.go        ← warn if deploying as root
  deploy/
    deploy.go              ← orchestrates: preflight → copy → compose up
    health.go              ← poll docker inspect health status
  wizard/
    wizard.go              ← --init flow: detect config, create deploy user
    remote.go              ← wizard-specific remote commands (adduser, usermod)
```

---

## Component Boundaries

| Component | Responsibility | Inputs | Outputs / Side Effects |
|-----------|---------------|--------|----------------------|
| `cmd/docker-deploy/main.go` | Docker plugin entry point only | `os.Args` | calls `plugin.Run()` |
| `internal/cli` | Cobra command tree, flag parsing | CLI args | resolved `Config` struct |
| `internal/config` | Load + merge configuration | flags, deploy.yaml, cwd | `Config` struct |
| `internal/ssh` | SSH connection lifecycle | host URI, auth | `*ssh.Client` (shared) |
| `internal/sftp` | File copy to remote | `*ssh.Client`, file list | remote files written |
| `internal/preflight` | Pre-deploy validation pipeline | `*ssh.Client` | pass/fail per check |
| `internal/deploy` | Deploy orchestration | `Config`, `*ssh.Client` | compose running, health result |
| `internal/wizard` | --init interactive setup | `*ssh.Client` (root), prompts | remote user created, deploy.yaml written |

**Hard rule:** `internal/wizard` does NOT call anything in `internal/deploy`. The wizard and the deploy path are siblings that share `internal/ssh` but have no other coupling.

---

## Data Flow

```
CLI args + flags
      |
      v
internal/config.Resolve()          (flags > deploy.yaml > defaults)
      |
      v
Config{Host, RemotePath, Files, Detach, ...}
      |
      +--[--init flag]---> internal/wizard  (interactive only, exits after)
      |
      v
internal/ssh.Dial(Config.Host)     (single *ssh.Client for entire deploy)
      |
      +---> internal/preflight.Run(checks...)
      |           |
      |           v
      |       []CheckResult  (fail fast on critical, warn on advisory)
      |
      +---> internal/sftp.Copy(fileList, remotePath)
      |           |
      |           v
      |       files written to remote via SFTP subsystem
      |
      +---> internal/ssh/exec.Run("docker compose up -d")
      |
      +--[detach=false]--> internal/deploy/health.Poll(containers)
      |                         |
      |                         v
      |                   health results streamed to stdout
      |
      v
ssh.Client.Close()
```

---

## Docker CLI Plugin Protocol

**How the protocol works (verified against docker/cli source):**

1. Plugin binary is named `docker-deploy` and lives in `~/.docker/cli-plugins/`.
2. Docker CLI discovers plugins by scanning that directory.
3. During discovery, Docker CLI invokes the binary with the first arg `docker-cli-plugin-metadata`. The plugin must write a JSON metadata document to stdout and exit 0.
4. Normal invocation: `docker deploy [args]` — Docker CLI invokes the binary without the metadata arg.

**main.go pattern:**

```go
package main

import (
    "github.com/docker/cli/cli-plugins/manager"
    "github.com/docker/cli/cli-plugins/plugin"
    "github.com/docker/cli/cli/command"
    "github.com/spf13/cobra"
    "github.com/yourorg/docker-deploy/internal/cli"
)

func main() {
    plugin.Run(func(dockerCli command.Cli) *cobra.Command {
        return cli.NewRootCmd(dockerCli)
    }, manager.Metadata{
        SchemaVersion: "0.1.0",
        Vendor:        "yourorg",
        Version:       "0.1.0",
        ShortDescription: "Deploy docker-compose projects to remote VPS via SSH",
    })
}
```

`plugin.Run()` handles the `docker-cli-plugin-metadata` argument internally — the plugin author does not need to check `os.Args[1]` manually. The `plugin.RunningStandalone()` function can be used to detect when the binary is invoked directly (not through the Docker CLI), which is useful for standalone mode or debugging.

**PersistentPreRunE note:** If the root command or any subcommand uses cobra's `PersistentPreRun*`, wrap it with `plugin.PersistentPreRunE` to preserve Docker CLI initialization.

---

## SSH Client Lifecycle

**Design decision: one `*ssh.Client`, many sessions.**

`golang.org/x/crypto/ssh` multiplexes multiple logical sessions over a single TCP connection. Creating a new session (via `client.NewSession()`) is cheap — it does not open a new TCP connection or TLS handshake. This is the correct pattern for executing multiple remote commands sequentially.

```
Dial once
  |
  +--> NewSession() for each preflight check command
  +--> sftp.NewClient(sshClient) for file copy (SFTP subsystem, same conn)
  +--> NewSession() for "docker compose up -d"
  +--> NewSession() per poll interval for "docker inspect ..."
  |
Close once
```

**Session rule:** Each `*ssh.Session` can run exactly one command. Create a new session per command execution. Do not reuse sessions.

**Auth resolution order (within `internal/ssh/auth.go`):**

1. Explicit `--identity` flag (path to private key file)
2. `SSH_AUTH_SOCK` env var → connect to ssh-agent (`golang.org/x/crypto/ssh/agent`)
3. Default key files in order: `~/.ssh/id_ed25519`, `~/.ssh/id_rsa`, `~/.ssh/id_ecdsa`
4. If none found → error with actionable message

**Host key verification:** Use `golang.org/x/crypto/ssh/knownhosts` to build a `HostKeyCallback` from `~/.ssh/known_hosts`. Do not use `ssh.InsecureIgnoreHostKey()`. If the host is unknown, present an interactive prompt (first-connect UX like OpenSSH), then append to known_hosts. If the key mismatches, hard-fail with a MITM warning.

---

## Config Resolution Order

**Implemented in `internal/config/resolve.go`:**

Priority (highest to lowest):
1. CLI flags (`--host`, `--remote-path`, `--detach`, `--identity`)
2. `deploy.yaml` in the current working directory
3. Compiled-in defaults (`RemotePath = /opt/<project_name>`, `Files = ["compose.yaml", ".env", "Makefile", "README.md"]`)

**`deploy.yaml` schema (minimal v1):**

```yaml
host: ssh://deploy@myserver.example.com:22
remote_path: /opt/myapp
identity: ~/.ssh/id_deploy
files:
  include:
    - compose.yaml
    - .env
    - config/
  exclude:
    - "*.log"
    - ".DS_Store"
```

**Implementation approach:** Use `gopkg.in/yaml.v3` to parse deploy.yaml into a typed struct. Do NOT use Viper — it is heavyweight and has known flag-override ordering bugs when defaults are set. Manual resolution in a dedicated `Resolve(flags, file, defaults)` function is simpler, testable, and avoids the `spf13/viper` pitfalls documented in the community.

---

## Pre-Flight Check Pipeline

**`internal/preflight/checks.go`** defines a simple interface:

```go
type Check interface {
    Name() string
    Run(ctx context.Context, exec RemoteExec) CheckResult
}

type CheckResult struct {
    Name    string
    Passed  bool
    Fatal   bool   // if true, abort deploy on failure
    Message string
}
```

**Checks implemented:**

| Check | Command | Fatal? | Notes |
|-------|---------|--------|-------|
| Docker installed | `docker --version` | YES | cannot proceed without Docker |
| Docker daemon running | `docker info` | YES | daemon may be installed but not started |
| User in docker group | `id -nG` (grep `docker`) | YES | compose up will fail with permission error |
| Root user warning | `id -u` == 0 | NO | warn but allow; document security implications |

**Pipeline runner:** Execute checks sequentially. On first fatal failure, surface error and exit. Advisory failures (non-fatal) are collected and displayed as warnings after the fatal check pass.

**Extensibility:** The `Check` interface allows future checks (disk space, compose version) without touching orchestration code.

---

## File Copy Strategy

**`internal/sftp/`** owns all file copy logic.

**Library:** `github.com/pkg/sftp` — wraps `golang.org/x/crypto/ssh`, well-maintained, supports `MkdirAll`, concurrent Goroutine use, and the `ReadFrom` interface for maximum throughput.

**Pattern:**

```
sftp.NewClient(sshClient)          ← reuses existing TCP connection
  |
  +--> client.MkdirAll(remotePath)
  |
  +--> for each file in filtered list:
         localStat → remoteDir.MkdirAll (for nested paths)
         client.Create(remotePath)
         io.Copy(remoteFile, localFile)   ← or f.ReadFrom(localFile) for speed
```

**Filter logic (`internal/sftp/filter.go`):**

1. Start with default includes: `["compose.yaml", "docker-compose.yaml", ".env", "Makefile", "README.md"]`
2. Merge user `files.include` from deploy.yaml (additive; patterns resolved relative to project root)
3. Apply `files.exclude` patterns (glob matching via `path.Match`)
4. Walk directories in the include list recursively, applying exclude at each entry

**Never copy:** `.git/`, `node_modules/`, `vendor/` — these are in a hardcoded base exclude list that the user cannot override (safety rail).

**Security note:** `.env` is copied as-is. Add a warning to stdout before copy: "Copying .env to remote host [host]. Ensure this is intended."

---

## Remote Command Execution

**`internal/ssh/exec.go`** wraps session management:

```go
func (c *Client) Run(ctx context.Context, cmd string) (stdout, stderr []byte, err error) {
    sess, _ := c.sshClient.NewSession()
    defer sess.Close()
    // CombinedOutput or separate Stdout/Stderr pipes
    // Context cancellation via sess.Signal(ssh.SIGINT)
}

func (c *Client) RunStreaming(ctx context.Context, cmd string, out io.Writer) error {
    // For compose up in attached mode — pipe remote stdout/stderr to local out
}
```

**Compose up invocation:**

- Detached mode (`--detach`): `docker compose -f compose.yaml up -d` → `Run()`, wait for exit code
- Attached mode (default): `docker compose -f compose.yaml up -d` then follow logs, or `up` without `-d` and stream. Given health polling is the v1 design, use `-d` always and poll separately.

**Health polling (`internal/deploy/health.go`):**

```
After compose up exits 0:
  containers = parse "docker compose ps --format json" on remote
  for each container:
    if has healthcheck:
      poll "docker inspect --format={{json .State.Health}} <id>"
      until Status == "healthy" OR timeout OR Status == "unhealthy"
    else:
      warn "Container <name> has no HEALTHCHECK defined"
```

**Poll interval:** 3 seconds, configurable. **Timeout:** 120 seconds, configurable. Use exponential backoff only if polls are expensive; for `docker inspect` they are cheap — fixed interval is fine.

---

## --init Wizard Isolation

**`internal/wizard/wizard.go`** is entered when `--init` flag is passed or auto-detected (no deploy.yaml + first deploy attempt).

**Isolation rules:**
- The wizard calls `internal/ssh` (for a root-privileged connection) but NOT `internal/deploy`, `internal/sftp`, or `internal/preflight`.
- The wizard exits (`os.Exit(0)`) after completing. It does not hand off to the deploy flow.
- To deploy after `--init`, the user re-runs `docker deploy`.

**Wizard flow:**

```
1. Prompt: root SSH host (may differ from deploy host if key-based root access is separate)
2. Connect via root SSH credentials (password auth allowed here only)
3. Check if deploy user already exists
4. Create user if not: adduser --disabled-password --gecos "" deployuser
5. Add to docker group: usermod -aG docker deployuser
6. Copy current user's public key to /home/deployuser/.ssh/authorized_keys
7. Write deploy.yaml to cwd with host: ssh://deployuser@<host>
8. Print: "Init complete. Run `docker deploy` to deploy."
```

**Interactive prompts:** Use `github.com/charmbracelet/huh` for the wizard (modern, maintained; `AlecAivazis/survey` is archived). The huh library supports accessible mode and has no runtime dependencies.

---

## Scalability Considerations

This is a CLI tool, not a server. "Scale" means handling edge cases, not load.

| Concern | v1 Approach | Future Extension Point |
|---------|-------------|----------------------|
| Multi-host targets | Single `host` in config | Named targets map in deploy.yaml: `targets: {prod: ..., staging: ...}` |
| Large file trees | Sequential SFTP upload | Concurrent uploads with worker pool (bounded goroutines) |
| Slow health polling | Fixed 3s poll | configurable `health_timeout`, `health_interval` in deploy.yaml |
| Plugin distribution | Manual binary install | `docker plugin install` or Homebrew tap |

---

## Suggested Build Order

Dependencies flow upward. Build bottom-up to enable testing at each layer.

```
Phase 1 — Foundation
  internal/config       (no external deps beyond yaml.v3)
  internal/ssh/auth     (key parsing, known_hosts)
  internal/ssh/client   (Dial + session management)

Phase 2 — Transport
  internal/sftp/filter  (pure logic, no I/O)
  internal/sftp/upload  (depends on ssh/client)
  internal/sftp/copy    (orchestrates filter + upload)

Phase 3 — Remote Operations
  internal/ssh/exec     (session-per-command wrapper)
  internal/preflight    (depends on ssh/exec)
  internal/deploy/health (depends on ssh/exec)

Phase 4 — Orchestration
  internal/deploy/deploy (ties preflight + sftp + exec + health together)

Phase 5 — CLI Layer
  internal/cli/root     (cobra commands, flag binding)
  internal/cli/deploy   (wires deploy command to internal/deploy)
  cmd/docker-deploy/main.go (plugin.Run wrapper)

Phase 6 — Wizard (independent of 4 and 5's deploy path)
  internal/wizard
  internal/cli/init     (--init subcommand)
```

---

## Anti-Patterns to Avoid

### Anti-Pattern 1: Global SSH client
**What:** Storing `*ssh.Client` in a package-level variable
**Why bad:** Untestable, non-reentrant, complicates future multi-host support
**Instead:** Pass `*ssh.Client` explicitly through function parameters or a `Deployer` struct

### Anti-Pattern 2: Wizard logic in deploy path
**What:** Auto-detecting unconfigured VPS during a normal `docker deploy` run
**Why bad:** Mixing wizard state machine into deploy orchestration causes complex conditional logic
**Instead:** If deploy.yaml is missing, print a clear message: "Run `docker deploy --init` to configure this host." Do not silently enter wizard mode.

### Anti-Pattern 3: Using Viper for config
**What:** `viper.BindPFlag()` + `viper.ReadInConfig()`
**Why bad:** Viper has documented flag-default override ordering bugs; it is heavyweight for a simple three-layer config
**Instead:** Manual `Resolve(flags, fileConfig, defaults)` function — 30 lines, fully testable, no surprises

### Anti-Pattern 4: `ssh.InsecureIgnoreHostKey()`
**What:** Skipping host key verification during development and shipping it
**Why bad:** MITM-vulnerable; unacceptable for a deployment tool copying `.env` files
**Instead:** `knownhosts.New(knownHostsPath)` from `golang.org/x/crypto/ssh/knownhosts`

### Anti-Pattern 5: One session for all commands
**What:** Reusing a single `*ssh.Session` for multiple `Run()` calls
**Why bad:** Each `*ssh.Session` supports exactly one command execution — calling `Run()` twice panics
**Instead:** Create a new session per command; the TCP connection is reused automatically

---

## Key Dependencies

| Library | Version Pin | Purpose |
|---------|-------------|---------|
| `github.com/docker/cli` | match installed Docker CLI version | plugin.Run(), command.Cli |
| `github.com/spf13/cobra` | v1.x (pulled transitively by docker/cli) | command tree |
| `golang.org/x/crypto` | latest | ssh client, ssh/agent, ssh/knownhosts |
| `github.com/pkg/sftp` | v1.x | SFTP file transfer over existing SSH conn |
| `gopkg.in/yaml.v3` | v3 | deploy.yaml parsing |
| `github.com/charmbracelet/huh` | v0.x | --init wizard interactive prompts |

---

## Sources

- Docker CLI plugin package: https://pkg.go.dev/github.com/docker/cli/cli-plugins/plugin
- Docker CLI plugin design spec: https://github.com/docker/cli/issues/1534
- docker/cli plugin.go source: https://github.com/docker/cli/blob/master/cli-plugins/plugin/plugin.go
- golang.org/x/crypto/ssh: https://pkg.go.dev/golang.org/x/crypto/ssh
- golang.org/x/crypto/ssh/knownhosts: https://pkg.go.dev/golang.org/x/crypto/ssh/knownhosts
- github.com/pkg/sftp: https://pkg.go.dev/github.com/pkg/sftp
- charmbracelet/huh: https://github.com/charmbracelet/huh
- Go SSH host key verification pattern: https://skarlso.github.io/2019/02/17/go-ssh-with-host-key-verification/
- Go project layout: https://go.dev/doc/modules/layout
