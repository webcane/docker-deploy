# Usage

## SSH URL format

Specify the remote host as an `ssh://` URL:

```bash
docker deploy --host ssh://sshuser@vps.example.com
```

The port defaults to 22 if omitted. To use a non-standard port:

```bash
docker deploy --host ssh://sshuser@vps.example.com:2222
```

## SSH config alias

Any host defined in `~/.ssh/config` can be passed directly to `--host`:

```
# ~/.ssh/config
Host minipc
  HostName 192.168.1.50
  User sshuser
  IdentityFile ~/.ssh/id_ed25519
```

```bash
docker deploy --host minipc
```

SSH config `User`, `HostName`, `Port`, `IdentityFile`, and `ProxyJump` are all honoured. Flags (`--host`, `--path`, etc.) and `deploy.yaml` values take precedence over SSH config defaults when both specify the same field.

## deploy.yaml config-driven deploy

Add a `deploy.yaml` to your project directory to avoid typing flags on every run:

```yaml
version: 1
target:
  host: ssh://sshuser@vps.example.com
  path: /opt/myapp
  compose_file: docker-compose.prod.yml
  force: true
  skip_env: false           # set true to preserve remote .env unchanged
  health_timeout: 90        # seconds to wait for healthy status (default: 60)
  health_interval: 10       # seconds between health polls (default: 5)
  exclude:
    - "tests/"
    - "*.md"
```

Once `deploy.yaml` is present, just run:

```bash
docker deploy
```

An SSH config alias works in `deploy.yaml` too:

```yaml
version: 1
target:
  host: minipc
```

See [DEPLOY_CONFIG.md](DEPLOY_CONFIG.md) for the full configuration reference.

## Custom remote path

Override the default `/opt/<project-dir-name>` target directory:

```bash
docker deploy --host ssh://sshuser@vps.example.com --path /srv/myapp
```

## Excluding files

Extend the built-in exclude list with `--exclude` (repeatable):

```bash
docker deploy --host ssh://sshuser@vps.example.com \
  --exclude "tests/" \
  --exclude "*.md"
```

The built-in defaults (`.git/`, `.gitignore`, etc.) always apply; `--exclude` adds on top of them.

## Recommended setup — non-root SSH user

Deploying as a non-root user is the recommended approach. Root deploys trigger a warning and violate least-privilege principles. Create a dedicated SSH user on your VPS (`sshuser`) with Docker group membership:

```bash
docker deploy --host ssh://sshuser@vps.example.com
```

See [PREREQUISITES.md](PREREQUISITES.md) for SSH key setup and passwordless sudo configuration.

## Flags-only deploy (no deploy.yaml)

Use flags for one-off deploys, CI environments, or when testing a new host:

```bash
docker deploy \
  --host ssh://sshuser@vps.example.com \
  --path /opt/myapp \
  --force \
  --compose-file docker-compose.prod.yml
```

`--force` skips the "target exists, replace?" confirmation prompt on repeat deploys. All flags take precedence over `deploy.yaml` values when both are present.

Use `--verbose` to print each file transferred and each SSH command executed:

```bash
docker deploy --host ssh://sshuser@vps.example.com --verbose
```

## validate subcommand

Check that all requirements for a successful deploy are met before pushing any files:

```bash
docker deploy validate --host ssh://sshuser@vps.example.com
```

Or with an SSH config alias:

```bash
docker deploy validate --host minipc
```

`validate` verifies SSH connectivity, known-hosts trust, Docker and Docker Compose availability on the remote, and that the resolved config is consistent. It exits non-zero if any check fails.

## Precedence

Flag values override `deploy.yaml`, which overrides built-in defaults:

```
--flag  >  deploy.yaml  >  default
```
