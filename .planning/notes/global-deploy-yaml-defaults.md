---
title: "Global deploy.yaml ships pre-populated"
date: 2026-05-26
context: global-defaults-config
---

The global `~/.docker/cli-plugins/deploy.yaml` should be written during install/init with
all keys present and commented out — so users can see every available option and uncomment
what they want to customise.

## Rationale

Without a pre-populated file, users have no way to discover what the global config supports
("illuminate used settings" goal). The file itself is the documentation.

## Suggested template

```yaml
# docker-deploy global defaults
# Values here apply to all projects unless overridden by a project-level deploy.yaml.
# Resolution order: --flag > project deploy.yaml > this file > built-in defaults

version: 1

target:
  # exclude:
  #   - .git/
  #   - node_modules/
  #   - vendor/
  #   - "*.log"
  #   - .DS_Store
  #   - __pycache__/
  #   - .claude/
  #   - .github/
  #   - .planning/
  #   - .idea/
  #   - .vscode/
  #   - "*.swp"
  #   - "*.swo"
  #   - coverage/
  #   - dist/
  #   - .terraform/

  # health_timeout: 60    # seconds to wait for health checks
  # health_interval: 5    # seconds between health check polls
  # ssh_dial_timeout: 10  # seconds to wait for SSH TCP connection
```

## When to write it

- The `docker deploy init` wizard (Phase 6) should offer to write this file
- Install instructions should mention its existence
- `docker deploy validate` (Phase 13) should load and display it as part of merged config output
