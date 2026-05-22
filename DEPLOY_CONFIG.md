# deploy.yaml Configuration Reference

Place `deploy.yaml` in your project root. All fields are optional — docker-deploy works with the `--host` flag alone.

## File structure

```yaml
version: 1
target:
  host: ssh://sshuser@vps.example.com
  path: /opt/myapp
  compose_file: docker-compose.prod.yml
  force: false
  skip_env: false
  health_timeout: 60
  health_interval: 5
  exclude:
    - "tests/"
    - "*.md"
```

## Field reference

| Field | Type | Default | Description |
|-------|------|---------|-------------|
| `version` | integer | `1` | Schema version. Currently must be `1`. |
| `target.host` | string | *(required if no `--host` flag)* | SSH connection URI: `ssh://[user@]host[:port]`. Default port is 22. Example: `ssh://sshuser@vps.example.com`. |
| `target.path` | string | `/opt/<project-name>` | Remote directory where files are copied. Must be an absolute path. The project name is derived from the local directory name. |
| `target.compose_file` | string | auto-detect | Compose file basename in project root. Auto-detects `compose.yaml` then `docker-compose.yml` if not set. |
| `target.force` | boolean | `false` | Skip the "target exists — replace?" confirmation prompt on repeat deploys. Equivalent to `--force` flag. |
| `target.skip_env` | boolean | `false` | Exclude `.env` from the upload, leaving the remote `.env` unchanged. Useful when the remote has production secrets not present locally. Equivalent to `--skip-env` flag. |
| `target.health_timeout` | integer | `60` | Seconds to wait for all containers to reach healthy status after `compose up`. Value `0` is treated as not set and falls back to the default of 60. Negative values are rejected with an error. |
| `target.health_interval` | integer | `5` | Seconds between health status polls during the health wait. |
| `target.exclude` | list of strings | `[]` | Additional glob patterns to exclude from the upload. These **extend** (never replace) the built-in defaults listed below. |

## Built-in default excludes

These patterns are always excluded and cannot be removed by `target.exclude` or `--exclude`:

```
.git/
node_modules/
vendor/
*.log
.DS_Store
__pycache__/
.claude/
.github/
.planning/
.idea/
.vscode/
*.swp
*.swo
coverage/
dist/
.terraform/
```

User-supplied excludes (via `target.exclude` in `deploy.yaml` or `--exclude` flags) are **appended** to this list, deduplicated, with insertion order preserved.

## Config precedence

Configuration is resolved in three tiers, from highest to lowest priority:

1. **CLI flags** (`--host`, `--path`, `--force`, `--skip-env`, `--exclude`, `--compose-file`) — highest priority; always override deploy.yaml and defaults.
2. **deploy.yaml values** — override built-in defaults when set.
3. **Built-in defaults** — lowest priority; used when neither a flag nor a deploy.yaml value is provided.

**Note on exclude lists:** The exclude list is additive across all three tiers. Built-in defaults, `target.exclude` values, and `--exclude` flag values are all merged and deduplicated — no tier can remove entries from a higher-priority tier.

## Flags without deploy.yaml equivalent

These flags control behaviour that has no deploy.yaml representation:

| Flag | Description |
|------|-------------|
| `--dry-run` | Verify SSH connectivity and print the resolved config; do not copy any files or run `docker compose up`. |
| `--verbose` | Print each file transferred, each SSH command executed, and the pre-flight checklist to stderr. |
