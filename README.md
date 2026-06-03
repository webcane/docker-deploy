# docker-deploy

[![CI](https://github.com/webcane/docker-deploy/actions/workflows/ci.yml/badge.svg)](https://github.com/webcane/docker-deploy/actions/workflows/ci.yml) [![Latest Release](https://img.shields.io/github/v/release/webcane/docker-deploy)](https://github.com/webcane/docker-deploy/releases) [![Go Report Card](https://goreportcard.com/badge/github.com/webcane/docker-deploy)](https://goreportcard.com/report/github.com/webcane/docker-deploy) [![License](https://img.shields.io/github/license/webcane/docker-deploy)](LICENSE) [![Open Issues](https://img.shields.io/github/issues/webcane/docker-deploy)](https://github.com/webcane/docker-deploy/issues) [![Coverage](https://raw.githubusercontent.com/webcane/docker-deploy/master/coverage.svg)](https://github.com/webcane/docker-deploy/actions/workflows/ci.yml)

Deploy your docker-compose project to any SSH-accessible remote host with a single command — no git required on the remote.

## What is docker-deploy?

A Docker CLI plugin for solo developers and small teams with a single VPS. One command copies your compose project files to the remote host via SFTP and runs `docker compose up -d` — no git on the remote, no container registry, no CI/CD pipeline required. It handles `.env` and secrets automatically, verifies SSH host keys, and reports deployment health.

## Installation

### Install script

Pin to a release tag (recommended — avoids fetching from an unpinned `master` branch):

```bash
curl -fsSL https://raw.githubusercontent.com/webcane/docker-deploy/v0.13.1/install.sh | sh
```

For Homebrew, manual binary download, and `go install`, see [INSTALL.md](INSTALL.md).

## Usage

### Quick Start

```bash
# Using an SSH URL
docker deploy --host ssh://sshuser@vps.example.com

# Using an SSH config alias (host defined in ~/.ssh/config)
docker deploy --host minipc
```

This copies your project files to `/opt/<project-dir-name>` on the remote host, runs `docker compose up -d`, and reports the deployment health status.

See [PREREQUISITES.md](PREREQUISITES.md) for SSH key setup and passwordless sudo configuration. For all usage scenarios — `deploy.yaml` config, custom paths, excludes, the `validate` subcommand, and more — see [USAGE.md](USAGE.md).

## Learn More

- [PREREQUISITES.md](PREREQUISITES.md) — SSH key setup, passwordless sudo for sshuser
- [DEPLOY_CONFIG.md](DEPLOY_CONFIG.md) — Full deploy.yaml field reference
- [TROUBLESHOOTING.md](TROUBLESHOOTING.md) — Common failures and fixes
- [COMPARISON.md](COMPARISON.md) — How docker-deploy compares to Kamal, Ansible, and other tools

## Feedback

Bug reports and feature requests are welcome. Open an issue at [github.com/webcane/docker-deploy/issues](https://github.com/webcane/docker-deploy/issues) — every report helps improve the tool.

[![TON](https://img.shields.io/badge/Donate-TON-blue)](https://tonviewer.com/UQCB7Y1q3cMl2wxfE1DDHr-VzJ-EeaJIUykx_CUkUdMrbtLG)
