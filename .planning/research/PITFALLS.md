# Domain Pitfalls

**Domain:** Docker CLI plugin — SSH/SCP deployment of docker-compose projects to remote VPS
**Researched:** 2026-05-13

---

## Critical Pitfalls

Mistakes that cause security incidents, data loss, or forced rewrites.

---

### Pitfall 1: InsecureIgnoreHostKey — Silent MITM Vulnerability

**What goes wrong:** Developer uses `ssh.InsecureIgnoreHostKey()` as the `HostKeyCallback` to "get it working quickly." The code ships and deploys production .env files (secrets) to whatever host answers the SSH connection — no verification that the remote is who it claims to be.

**Why it happens:** The Go `crypto/ssh` library requires a `HostKeyCallback` at construction time. The zero value causes all calls to fail with an opaque error, so the first fix found is `InsecureIgnoreHostKey`. It works, so it stays.

**Consequences:** A compromised DNS record, BGP hijack, or same-network ARP spoofing routes deploys to an attacker's machine. Every secret in `.env` is exfiltrated silently. No warning is shown to the user.

**Prevention:**
- Use `golang.org/x/crypto/ssh/knownhosts` to build a `HostKeyCallback` from the user's `~/.ssh/known_hosts`.
- On first connect to an unknown host, prompt the user: "Fingerprint: SHA256:xxx — trust and add to known_hosts? [y/N]" (mirrors OpenSSH `StrictHostKeyChecking=ask` UX).
- Never default to accept-all, even behind a flag. If needed, expose an explicit `--insecure-skip-host-verify` flag that prints a loud warning.
- Known pitfall: the `knownhosts` package returns a `*knownhosts.KeyError` when the key is unknown (not in file) vs. mismatched (changed key). Parse the error type to distinguish "add new host" from "host key changed — STOP."

**Warning signs:** Any code path containing `ssh.InsecureIgnoreHostKey`. Any `HostKeyCallback: nil` that somehow doesn't error.

**Phase:** Address in the SSH transport layer implementation phase. Do not defer.

---

### Pitfall 2: ssh.Dial Hangs Indefinitely — ClientConfig.Timeout Does Not Cover SSH Handshake

**What goes wrong:** The tool appears to hang on deploy with no output. `ClientConfig.Timeout` only applies to the TCP `net.Dial` call. Once the TCP connection is established, the SSH key exchange (`kexLoop`) and client authentication (`clientAuthenticate`) can block forever if the remote is slow, misconfigured, or deliberately stalling.

**Why it happens:** Multiple tracked issues in the Go issue tracker confirm this behavior (issues #15113, #51926, #50046). The `Timeout` field in `ClientConfig` is a documented footgun — developers assume it covers the full handshake.

**Consequences:** `docker deploy` hangs with no output. CI pipelines time out externally. The user has no feedback or way to cancel cleanly.

**Prevention:**
- Wrap the entire dial sequence in a `context.WithTimeout` and use a goroutine: dial in a goroutine, select on `ctx.Done()` and a result channel.
- Pattern:
  ```go
  ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
  defer cancel()
  type result struct { client *ssh.Client; err error }
  ch := make(chan result, 1)
  go func() {
      c, err := ssh.Dial("tcp", addr, config)
      ch <- result{c, err}
  }()
  select {
  case r := <-ch:
      // handle r.client, r.err
  case <-ctx.Done():
      return fmt.Errorf("SSH connection timed out after 30s")
  }
  ```
- Expose `--timeout` flag with a sensible default (30s).
- Implement periodic SSH keep-alive requests (send `keepalive@openssh.com` global requests on an interval) to detect dead connections during long-running operations like streaming logs.

**Warning signs:** No explicit context-based timeout wrapping the SSH dial. Tests pass on fast local VMs but hang on real VPS deployments with packet loss.

**Phase:** SSH transport implementation phase. Keep-alive can be a follow-up milestone.

---

### Pitfall 3: SCP to /opt as Non-Root Fails — Directory Ownership Not Pre-Created

**What goes wrong:** The deploy user (non-root, created by `--init`) cannot write to `/opt/<project>` because `/opt` is owned by root and the subdirectory does not yet exist. SCP fails with "permission denied." Alternatively, the `--init` flow creates the directory owned by root, and subsequent non-root SCP still fails.

**Why it happens:** `/opt` is a system directory (owned `root:root`, mode `755`). Creating a subdirectory requires root. Developers test as root and miss this. The deploy user only gets docker group membership, not write access to `/opt`.

**Consequences:** First deploy fails. User sees an opaque SCP error. If the `--init` flow does not `chown` the created directory to the deploy user, every deploy fails even after init succeeds.

**Prevention:**
- During `--init` (root SSH session): create `/opt/<project>`, then `chown deployuser:deployuser /opt/<project>`.
- During pre-flight checks on non-init deploy: run `ssh exec "test -w /opt/<project>"`. If it fails, exit with a clear message: "Remote directory not writable. Run `docker deploy --init` first."
- Do not assume the directory exists. Create it atomically: `mkdir -p /opt/<project>` owned by deploy user during init.
- Consider defaulting the deploy path to the deploy user's home directory (`~/deploy/<project>`) instead of `/opt` to avoid the permission issue entirely for the common case.

**Warning signs:** Tests only run as root on the test VPS. `/opt/<project>` pre-created manually in test setup.

**Phase:** `--init` flow and pre-flight check implementation.

---

### Pitfall 4: Partial Deploy — Files Copied, docker compose up Fails, Remote Left in Inconsistent State

**What goes wrong:** SCP completes (new `compose.yaml`, `.env` copied), but `docker compose up -d` fails (image pull error, compose syntax error, etc.). The remote now has new config files but containers are running from the previous config, or stopped entirely. Re-running the deploy does not automatically recover — the user must manually reconcile.

**Why it happens:** SCP and SSH command execution are separate operations with no transaction semantics. There is no atomic "apply" at the remote end.

**Consequences:** Production containers are stopped or in a degraded state with mismatched config files. The user may not realize the running containers reflect the old config. Rolling back requires manual SSH access.

**Prevention:**
- Use a staging approach: SCP files to a temp directory (`/opt/<project>/.deploy-tmp-<timestamp>`), then run a remote script that atomically moves files into place and executes `docker compose up -d`. If compose fails, the old files are still in place (move is fast and the containers were not touched before compose succeeded).
- At minimum: before SCP, snapshot the current directory (`cp -r /opt/<project> /opt/<project>.previous`), and on failure print "Previous deploy saved at /opt/<project>.previous — restore with: mv /opt/<project>.previous /opt/<project>".
- Report compose exit code explicitly. Do not treat compose stderr output as success. Check the return code from the remote command.
- In the health check phase: if polling times out, report the compose logs, not just "deploy failed."

**Warning signs:** SCP and `docker compose up` run sequentially with no intermediary state check. No cleanup or rollback path in the code.

**Phase:** Core deploy flow. Staging directory approach should be implemented from the start, not retrofitted.

---

### Pitfall 5: docker-compose (v1) vs docker compose (v2) — Remote Has the Wrong One

**What goes wrong:** The tool runs `docker compose up -d` on the remote, but the VPS is running an older Ubuntu/Debian with only `docker-compose` (Python v1, installed via apt) and no `docker compose` plugin. The command silently fails or produces "docker: 'compose' is not a docker command."

**Why it happens:** Docker Compose v1 (the standalone `docker-compose` binary) reached end-of-life in June 2023 but remains installed on many VPS instances that haven't been updated, especially Ubuntu 20.04 systems using the default apt package.

**Consequences:** Deploy fails with a confusing Docker CLI error rather than a compose error. User assumes the tool is broken rather than the remote Docker installation being outdated.

**Additional breaking differences:**
- Container naming: v1 uses underscores (`project_service_1`), v2 uses hyphens (`project-service-1`). Running v2 on a v1-launched project recreates all containers.
- The top-level `version:` key in `compose.yaml` generates a warning in v2 but was required in some v1 versions.
- Environment variable interpolation edge cases differ between v1 and v2.

**Prevention:**
- Pre-flight check: SSH to remote and run `docker compose version`. If that fails, try `docker-compose version`. Report clearly which is present and whether it meets minimum requirements.
- Require `docker compose` (v2) as a hard requirement. If only v1 is found, fail with: "Remote has docker-compose v1 (EOL). Upgrade: https://docs.docker.com/compose/migrate/"
- Do not attempt to support both. The compatibility matrix is too large for v1.

**Warning signs:** Pre-flight checks only verify Docker daemon is running, not compose plugin version.

**Phase:** Pre-flight check implementation phase.

---

### Pitfall 6: go.mod Dependency Conflicts with github.com/docker/cli

**What goes wrong:** The plugin uses `github.com/docker/cli` for the plugin framework (`cli-plugins/plugin`). The same module is a transitive dependency of other Docker-related libraries (`github.com/moby/moby`, `github.com/docker/docker`). These libraries pin different versions of `docker/cli`. The build fails with version conflicts, or subtle runtime mismatches occur if `replace` directives are used to paper over them.

**Why it happens:** The Docker ecosystem's Go modules have historically had messy versioning. `github.com/docker/docker` is deprecated in favor of `github.com/moby/moby/client` (as of v29). Replace directives are non-transitive in Go modules — they only apply to the main module, so a `replace` in your `go.mod` doesn't affect how the dependency resolves for consumers. The Docker CLI itself vendors all dependencies, which hides this in upstream but exposes it downstream.

**Consequences:**
- Build fails with "ambiguous import" or "incompatible versions."
- If using `replace` directives to force version alignment, the directive does not propagate, and the build breaks when `go mod tidy` is run without the replacement.
- Subtle API incompatibilities if mismatched versions of the plugin framework are used (plugin metadata format changes between major Docker CLI versions).

**Prevention:**
- Pin `github.com/docker/cli` to a specific version matching the Docker CLI version you target (e.g., v27 or v28). Document this constraint explicitly.
- Minimize Docker SDK surface area: do not import `github.com/moby/moby` unless required. The plugin only needs `github.com/docker/cli/cli-plugins/plugin` and `github.com/docker/cli/cli-plugins/manager`.
- Run `go mod tidy` and `go build ./...` as CI steps without any replace directives to catch conflicts early.
- If a `replace` directive is required during development (e.g., local testing against a forked CLI), add a CI check that asserts the directive is not present in the release branch.
- Track the Docker CLI module path rename: `github.com/docker/docker` → `github.com/moby/moby` (v29+). A `replace` cannot bridge a module path rename.

**Warning signs:** `go.mod` contains `replace` directives checked into the main branch. Multiple versions of `github.com/docker/cli` appear in `go.sum`.

**Phase:** Project scaffolding phase. Lock versions before writing business logic.

---

## Moderate Pitfalls

Issues that cause functional problems or user confusion but are recoverable.

---

### Pitfall 7: .env File Copied Verbatim — Secrets Exposed in Remote Filesystem Permissions

**What goes wrong:** The `.env` file containing database passwords, API keys, and other secrets is SCP'd to the remote VPS. The file lands with world-readable permissions (`644` by default from most SCP implementations), making secrets readable by any user on the VPS.

**Why it happens:** SCP preserves source file permissions when using the `-p` flag, but without it, remote umask applies. Default umask `022` produces `644`. Any other user who can log into the VPS can read the secrets.

**Prevention:**
- After SCP, immediately `chmod 600 /opt/<project>/.env` via SSH command.
- Consider issuing a warning when `.env` is in the file list: "Warning: .env file contains secrets — ensure your VPS restricts login to trusted users."
- Document clearly that secrets management (e.g., Vault, cloud secrets manager) is out of scope for v1 but the warning is by design.
- Do not implement any logging of `.env` file contents — even partial logging for debug purposes risks secret exfiltration into log files.

**Warning signs:** No `chmod` call after file copy. Debug logging that prints transferred file contents.

**Phase:** Core deploy flow (file copy phase).

---

### Pitfall 8: Health Check Polling Race — "Healthy" Before Container Is Actually Ready

**What goes wrong:** The tool polls `docker inspect` for health status and reports "healthy" as soon as the status field returns `healthy`. However, some containers mark themselves healthy before their application is actually accepting connections (e.g., the health check pings a `/health` endpoint that responds before the database migrations complete).

**Inverse problem:** The tool polls too aggressively and times out before `start_period` elapses, reporting "unhealthy" for a container that would have been healthy in 30 more seconds.

**Why it happens:** Docker's health check `start_period` delays counting failures but does not delay status transitions. A container enters the `starting` state during `start_period`, and the first successful check transitions it to `healthy` immediately — even if `start_period` has not elapsed. Polling code that doesn't handle the `starting` state correctly may loop forever or exit early.

**Prevention:**
- Poll on a configurable interval (default 5s) with a configurable timeout (default 2 minutes). Expose both as flags (`--health-interval`, `--health-timeout`).
- Treat `starting` as "not yet determined, keep polling" — do not treat it as failure.
- When no healthcheck is configured (`docker inspect` returns `Health: null`), emit a warning: "No HEALTHCHECK defined — deploy succeeded but health cannot be verified" and exit 0.
- Do not use the container's `Status` field (`running`) as a proxy for health — a container can be `running` and have a failing health check.

**Warning signs:** Polling logic that exits immediately on first non-`healthy` state rather than retrying. No handling for `null` health state.

**Phase:** Health check polling implementation.

---

### Pitfall 9: Docker CLI Plugin Binary Naming and Installation Pitfalls

**What goes wrong:** The plugin binary is named incorrectly, placed in the wrong directory, or is not executable. Docker CLI silently ignores plugins that fail validation rather than reporting the specific error.

**Specific failure modes:**
- Binary named `deploy` instead of `docker-deploy` — Docker CLI plugin convention requires the `docker-` prefix.
- Installed to `/usr/local/bin/` instead of `~/.docker/cli-plugins/` or `/usr/lib/docker/cli-plugins/` — Docker only searches specific directories.
- Binary not executable — Docker silently skips it.
- `docker-cli-plugin-metadata` subcommand not implemented or returns invalid JSON — Docker cannot discover the plugin.
- Plugin metadata `SchemaVersion` mismatch — Docker CLI expects `"0.1.0"` and will reject other values.

**Why it happens:** The plugin discovery mechanism is silent about individual plugin failures. Running `docker info` or `docker plugin ls` does not show why a plugin was skipped. The only debugging path is `docker system info --format json | jq .ClientInfo.Plugins`.

**Prevention:**
- Follow the exact naming convention: binary = `docker-deploy`, location = `~/.docker/cli-plugins/`.
- Implement `docker-cli-plugin-metadata` as the first subcommand, returning valid JSON with `SchemaVersion: "0.1.0"`.
- Write an installation script (or Makefile target) that places the binary and sets `chmod +x`.
- Test plugin discovery explicitly: `docker deploy --help` should show the plugin's help, not Docker's default help.
- Use `github.com/docker/cli/cli-plugins/plugin.Run()` as the entry point — it handles the metadata subcommand automatically.

**Warning signs:** Plugin not appearing in `docker --help` output after install. No integration test that actually invokes the plugin through Docker CLI.

**Phase:** Project scaffolding. Validate plugin discovery before writing any deploy logic.

---

### Pitfall 10: SSH Key Algorithm Mismatch in known_hosts

**What goes wrong:** The remote server presents an `ecdsa-sha2-nistp256` host key, but the entry in the user's `~/.ssh/known_hosts` was recorded when the server used `ssh-rsa`. The `knownhosts` package returns a `KeyError` that looks like a mismatch (potential MITM) rather than a "different algorithm" issue. The tool either refuses to connect (correct but confusing) or the developer adds a bypass that accepts any key.

**Why it happens:** Servers can have multiple host keys of different types. OpenSSH's `known_hosts` stores them separately. The Go `knownhosts` package (as of Go issue #49631) selects which key to verify based on what's in `known_hosts`, not what the server advertises, which can lead to algorithm mismatches when `known_hosts` has a stale key type.

**Prevention:**
- When a `KeyError` is returned and `knownhosts.KeyError.Want` is non-empty (key mismatch), display the stored fingerprint and the presented fingerprint and exit with a clear message: "Host key mismatch — possible key rotation or MITM. If the server's key was legitimately rotated, run: `ssh-keygen -R <host>` then re-deploy."
- When `Want` is empty (host not in known_hosts), offer to add the key interactively.
- Do not set `HostKeyAlgorithms` to force a specific algorithm type — let the server negotiate and use `known_hosts` as the source of truth.

**Warning signs:** Code that catches all `KeyError` types and prompts "add to known_hosts?" without distinguishing new host from changed host.

**Phase:** SSH transport implementation.

---

### Pitfall 11: docker group Membership — Effective Root on the Remote Host

**What goes wrong:** The `--init` flow adds the deploy user to the `docker` group to allow running `docker compose` without sudo. This is correct and required. However, the tool or documentation should make clear that `docker` group membership is equivalent to root on that host. If the deploy user's SSH key is compromised, the attacker has full root access via container escape.

**Why it happens:** `docker` group members can mount the host filesystem inside a container: `docker run -v /:/host --rm alpine cat /host/etc/shadow`. This is a documented, intentional capability. The threat model for a deploy-only user is different from the threat model for an interactive shell user, but the surface is the same.

**Prevention:**
- During `--init`, print a one-time warning: "Note: The deploy user will be added to the docker group, which grants root-equivalent access to this host. Ensure this user's SSH key is protected."
- Do not suppress this warning with a `--quiet` flag in the `--init` flow.
- This is a known, accepted risk for single-user VPS deployments. Document it explicitly rather than hiding it.
- Future consideration: rootless Docker mode eliminates this risk but requires more complex setup — flag as a v2 investigation item.

**Warning signs:** `--init` adds docker group membership with no user-facing warning.

**Phase:** `--init` implementation.

---

## Minor Pitfalls

Small issues that cause confusion or minor bugs.

---

### Pitfall 12: SSH URI Parsing Edge Cases

**What goes wrong:** The tool accepts `--host "ssh://user@host:port"` but the URI parsing is done manually (string split) rather than using `net/url`. Edge cases: IPv6 addresses (`ssh://user@[::1]:22`), non-standard ports, usernames with special characters, hosts without explicit port (defaults to 22).

**Prevention:** Use `net/url.Parse()` to parse the SSH URI. Handle the IPv6 bracket notation. Default port to 22 if absent. Validate that the scheme is `ssh://` before proceeding.

**Phase:** SSH transport implementation.

---

### Pitfall 13: Compose Project Name Inference Differs Between v1 and v2

**What goes wrong:** Docker Compose infers the project name from the directory name. If the tool copies files to `/opt/myapp/` and runs `docker compose up -d` from that directory, the project name is `myapp`. But if the user also has a `compose.yaml` with `name: myproject` defined, v2 uses the explicit name. Subsequent runs may see containers from both names and treat them as different projects.

**Prevention:** Explicitly pass `--project-name <name>` to all `docker compose` commands. Derive the project name from the compose file's `name:` field if present, otherwise from the target directory name. Store the resolved name in `deploy.yaml` after first deploy.

**Phase:** Core deploy flow.

---

### Pitfall 14: Streaming Remote Logs Over SSH — Session Lifecycle

**What goes wrong:** The tool streams `docker compose logs -f` over SSH in non-detached mode. When the user hits Ctrl-C, the local process receives SIGINT but the remote SSH session may not be terminated, leaving a dangling `docker compose logs` process on the remote.

**Prevention:** Use SSH session's `Signal()` method to forward SIGINT to the remote process. Alternatively, use `cmd.Wait()` in combination with a context that is cancelled on local signal, and call `session.Close()` to terminate the remote process.

**Phase:** Log streaming / non-detached mode implementation.

---

## Phase-Specific Warnings

| Phase Topic | Likely Pitfall | Mitigation |
|-------------|---------------|------------|
| Project scaffolding / go.mod | Docker CLI module version conflicts (Pitfall 6) | Lock `docker/cli` version immediately; CI without replace directives |
| Plugin binary installation | Binary naming / metadata format (Pitfall 9) | Validate plugin discovery in first integration test |
| SSH transport layer | Dial hangs (Pitfall 2) + known_hosts (Pitfall 1) + key algorithm mismatch (Pitfall 10) | Context-based timeout; knownhosts callback before any other code |
| `--init` flow | Directory ownership (Pitfall 3) + docker group warning (Pitfall 11) | chown in init; warning printed unconditionally |
| File copy (SCP) | .env permissions (Pitfall 7) + partial deploy state (Pitfall 4) | chmod 600 post-copy; staging directory pattern |
| Remote compose execution | v1 vs v2 compose (Pitfall 5) + project name (Pitfall 13) | Pre-flight compose version check; explicit --project-name |
| Health polling | Race conditions (Pitfall 8) | Handle `starting` state; configurable timeout |
| Log streaming | SSH session teardown (Pitfall 14) | Signal forwarding on local SIGINT |

---

## Sources

- Go crypto/ssh hangs: [Issue #15113](https://github.com/golang/go/issues/15113), [Issue #51926](https://github.com/golang/go/issues/51926), [Issue #50046](https://github.com/golang/go/issues/50046)
- SSH keep-alive gap: [Issue #21478](https://github.com/golang/go/issues/21478)
- Go SSH host key verification: [knownhosts package](https://pkg.go.dev/golang.org/x/crypto/ssh/knownhosts), [Key algorithm mismatch issue #49631](https://github.com/golang/go/issues/49631)
- Go SSH security overview: [Golang SSH Security](https://bridge.grumpy-troll.org/2017/04/golang-ssh-security/)
- Docker Compose v1 EOL and v2 migration: [Docker Docs migrate](https://docs.docker.com/compose/releases/migrate/)
- Docker CLI plugin architecture: [cli-plugins/manager](https://pkg.go.dev/github.com/docker/cli/cli-plugins/manager), [plugin.go](https://github.com/docker/cli/blob/master/cli-plugins/plugin/plugin.go)
- Docker SDK dependency collision: [moby/moby issue #41191](https://github.com/moby/moby/issues/41191)
- moby module path rename: [docker/buildx issue #3792](https://github.com/docker/buildx/issues/3792)
- Docker group security: [Docker Engine security](https://docs.docker.com/engine/security/), [Docker socket risk analysis](https://rome-rohani.medium.com/is-docker-group-access-putting-your-system-at-risk-4dc9978bd18b)
- StrictHostKeyChecking: [Linux Audit guide](https://linux-audit.com/ssh/config/client/option-stricthostkeychecking/)
- .env file security on VPS: [DCHost blog](https://www.dchost.com/blog/en/managing-env-files-and-secrets-on-a-vps-safely/)
- Docker health check timing: [Last9 guide](https://last9.io/blog/docker-compose-health-checks/)
