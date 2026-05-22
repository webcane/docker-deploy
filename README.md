# docker-deploy

[![CI](https://github.com/webcane/docker-deploy/actions/workflows/ci.yml/badge.svg)](https://github.com/webcane/docker-deploy/actions/workflows/ci.yml) [![Latest Release](https://img.shields.io/github/v/release/webcane/docker-deploy)](https://github.com/webcane/docker-deploy/releases) [![Go Report Card](https://goreportcard.com/badge/github.com/webcane/docker-deploy)](https://goreportcard.com/report/github.com/webcane/docker-deploy) [![License](https://img.shields.io/github/license/webcane/docker-deploy)](LICENSE) [![Open Issues](https://img.shields.io/github/issues/webcane/docker-deploy)](https://github.com/webcane/docker-deploy/issues) [![Codecov](https://codecov.io/gh/webcane/docker-deploy/branch/main/graph/badge.svg)](https://codecov.io/gh/webcane/docker-deploy)

Deploy your docker-compose project to any SSH-accessible VPS with a single command — no git required on the remote.

## What is docker-deploy?

docker-deploy is a Docker CLI plugin (`docker deploy`) that solves the problem of deploying a local docker-compose project to a remote VPS without needing git, a container registry, or a CI/CD pipeline on the server. It copies your project files (compose.yaml, .env, Makefile, and more) to the target directory on the remote host via SFTP, runs `docker compose up -d`, and reports deployment health — all without exposing the Docker daemon socket. It is built for developers who want the simplicity of `scp + compose up` without the operational overhead of Kamal, Ansible, or a full CI/CD system.

## Installation

### Option 1: Homebrew (macOS / Linux)

```bash
brew tap webcane/docker-deploy
brew install docker-deploy
```

Homebrew installs the binary to the Homebrew prefix. Add a symlink to the Docker CLI plugin directory so Docker discovers it:

```bash
mkdir -p ~/.docker/cli-plugins
ln -sf $(brew --prefix)/bin/docker-deploy ~/.docker/cli-plugins/docker-deploy
```

### Option 2: Install script (macOS / Linux)

```bash
curl https://raw.githubusercontent.com/webcane/docker-deploy/main/install.sh | sh
```

Installs the latest release. To pin a specific version:

```bash
INSTALL_VERSION=v1.0.0 curl https://raw.githubusercontent.com/webcane/docker-deploy/main/install.sh | sh
```

### Option 3: Manual binary (GitHub Releases)

1. Go to the [releases page](https://github.com/webcane/docker-deploy/releases) and download the archive for your OS and architecture (e.g. `docker-deploy_linux_amd64.tar.gz`).
2. Extract the archive, move the binary to `~/.docker/cli-plugins/`, and make it executable:

```bash
tar -xzf docker-deploy_linux_amd64.tar.gz
mkdir -p ~/.docker/cli-plugins
mv docker-deploy ~/.docker/cli-plugins/docker-deploy
chmod +x ~/.docker/cli-plugins/docker-deploy
```

Replace `linux_amd64` with your OS and architecture (e.g. `darwin_arm64` for Apple Silicon).

### Option 4: go install

```bash
GOBIN=~/.docker/cli-plugins go install github.com/webcane/docker-deploy/cmd/docker-deploy@latest
```

`GOBIN` must be set to `~/.docker/cli-plugins` — Docker CLI plugins must live in that directory, not in the standard `$GOPATH/bin`. Without `GOBIN`, `docker deploy` will not be discoverable by the Docker CLI.

## Usage

### Quick Start

```bash
docker deploy --host ssh://sshuser@vps.example.com
```

This copies your project files to `/opt/<project-dir-name>` on the remote host, runs `docker compose up -d`, and reports the deployment health status.

### Scenario 1: Recommended setup — non-root SSH user (sshuser)

Deploying as a non-root user is the recommended approach. Root deploys trigger a warning and violate least-privilege principles. Create a dedicated SSH user on your VPS (`sshuser`) with Docker group membership, then deploy:

```bash
docker deploy --host ssh://sshuser@vps.example.com
```

Add a `deploy.yaml` to your project directory to avoid typing the host on every run:

```yaml
version: 1
target:
  host: ssh://sshuser@vps.example.com
  path: /opt/myapp          # defaults to /opt/<project-dir-name>
```

Once `deploy.yaml` is present, just run:

```bash
docker deploy
```

See [PREREQUISITES.md](PREREQUISITES.md) for SSH key setup and passwordless sudo configuration.

### Scenario 2: Flags-only (no deploy.yaml)

Use flags for one-off deploys, CI environments, or when testing a new host without committing a config file:

```bash
docker deploy \
  --host ssh://sshuser@vps.example.com \
  --path /opt/myapp \
  --force \
  --compose-file docker-compose.prod.yml
```

`--force` skips the "target exists, replace?" confirmation prompt on repeat deploys. All flags take precedence over `deploy.yaml` values when both are present.

### Scenario 3: Config-driven deploy (deploy.yaml)

Use `deploy.yaml` for repeat deploys from the same directory. This example shows all supported fields:

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

Then deploy:

```bash
docker deploy
```

Use `--verbose` to see each file transferred and each SSH command executed during the deploy.

See [DEPLOY_CONFIG.md](DEPLOY_CONFIG.md) for the full configuration reference.

## Learn More

- [PREREQUISITES.md](PREREQUISITES.md) — SSH key setup, passwordless sudo for sshuser
- [DEPLOY_CONFIG.md](DEPLOY_CONFIG.md) — Full deploy.yaml field reference
- [TROUBLESHOOTING.md](TROUBLESHOOTING.md) — Common failures and fixes
- [COMPARISON.md](COMPARISON.md) — How docker-deploy compares to Kamal, Ansible, and other tools

## Feedback

Bug reports and feature requests are welcome. Open an issue at [github.com/webcane/docker-deploy/issues](https://github.com/webcane/docker-deploy/issues) — every report helps improve the tool.

[![TON](https://img.shields.io/badge/Donate-TON-blue)](https://tonviewer.com/UQCB7Y1q3cMl2wxfE1DDHr-VzJ-EeaJIUykx_CUkUdMrbtLG)
