---
title: Consolidate remote sudo calls into one SSH session
date: 2026-05-25
priority: high
---

Currently the deploy loop opens a separate SSH session per remote command, causing sudo to prompt once per session — resulting in 3+ prompts during a typical deploy.

## Fix

Batch all privileged operations (move from staging tmp → `/opt/<project>`, chown, cleanup) into a single SSH session running one `sudo bash -c "..."` command. This limits sudo to exactly one prompt per deploy regardless of file count.

## Affected path

Only applies when the target directory requires elevation (e.g. `/opt/<project>`). If the user targets a user-writable path (e.g. `~/<project>`), sudo is not invoked at all — no change needed for that case.

## Implementation notes

- SFTP upload to user-writable staging tmp stays as-is (no sudo)
- After upload, open one SSH session: `sudo bash -c "mv /tmp/staging-xxx/* /opt/project/ && rm -rf /tmp/staging-xxx"`
- Detect at deploy start whether target path needs sudo and set a flag for the session
