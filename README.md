# docker-deploy

[![CI](https://github.com/webcane/docker-deploy/actions/workflows/ci.yml/badge.svg)](https://github.com/webcane/docker-deploy/actions/workflows/ci.yml) [![Latest Release](https://img.shields.io/github/v/release/webcane/docker-deploy)](https://github.com/webcane/docker-deploy/releases) [![Go Report Card](https://goreportcard.com/badge/github.com/webcane/docker-deploy)](https://goreportcard.com/report/github.com/webcane/docker-deploy) [![License](https://img.shields.io/github/license/webcane/docker-deploy)](LICENSE) [![Open Issues](https://img.shields.io/github/issues/webcane/docker-deploy)](https://github.com/webcane/docker-deploy/issues) [![Codecov](https://codecov.io/gh/webcane/docker-deploy/branch/master/graph/badge.svg)](https://codecov.io/gh/webcane/docker-deploy)

Deploy your docker-compose project to any SSH-accessible remote host with a single command — no git required on the remote.

## What is docker-deploy?

A Docker CLI plugin for solo developers and small teams with a single VPS. One command copies your compose project files to the remote host via SFTP and runs `docker compose up -d` — no git on the remote, no container registry, no CI/CD pipeline required. It handles `.env` and secrets automatically, verifies SSH host keys, and reports deployment health.

## Installation

### Install script

Pin to a release tag (recommended — avoids fetching from an unpinned `master` branch):

```bash
curl -fsSL https://raw.githubusercontent.com/webcane/docker-deploy/v0.8.4/install.sh | sh
```

For Homebrew, manual binary download, and `go install`, see [INSTALL.md](INSTALL.md).

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
