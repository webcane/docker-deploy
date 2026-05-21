---
name: verbose-mode-file-diff
description: In --verbose mode, list remote files and local files before the replace confirmation so the operator can see what will change before confirming
type: todo
priority: medium
date: 2026-05-21
---

## Task

Enhance `--verbose` mode: before prompting "Replace all contents?", show remote files currently on the host and local files about to be copied, so the operator sees a before/after diff and can make an informed decision.

### Current verbose flow

```
Target /opt/test-deploy exists on 192.168.1.99. Replace all contents? [y/N] y
Uploading 7 files...
  -> .env
  -> README.md
  ...
```

### Desired verbose flow

```
Remote files on 192.168.1.99:/opt/test-deploy (5 files):
  .env
  README.md
  compose.yaml
  deploy.yaml
  docker-compose.yml.bk

Local files to upload (7 files):
  .env
  README.md
  compose.yaml
  deploy.yaml
  docker-compose.yml.bk
  entrypoint.sh
  test-compose.yaml

Target /opt/test-deploy exists on 192.168.1.99. Replace all contents? [y/N]
```

### Implementation notes

- Use SFTP `ReadDir` on remote target dir to list existing files (only when `--verbose` is set and target dir exists)
- Reuse existing `WalkFiles` result — just display it before upload starts instead of inline during upload
- The confirm prompt itself is unchanged; only the preamble in verbose mode changes
- No new config fields needed

### Files likely affected

- `internal/filetransfer/` — SFTP ReadDir call + verbose preamble output
- `internal/deploy/main.go` or wire-up layer — pass verbose flag through to pre-confirm display
