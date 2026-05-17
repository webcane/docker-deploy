---
title: Verify deploy.yaml loading with fresh binary after Phase 5
date: 2026-05-17
priority: high
---

Before starting Phase 6, confirm that `deploy.yaml` is correctly loaded and used by
the current installed binary.

**Steps:**
1. Run `make install` from repo root to ensure binary reflects all Phase 5 changes
2. Create a minimal `deploy.yaml` in a test directory:
   ```yaml
   version: 1
   target:
     host: ssh://user@host:port
     path: /opt/myproject
   ```
3. Run `docker deploy --dry-run` from that directory (no `--host` flag)
4. Confirm it connects using the host from `deploy.yaml`, not the "no host configured" error

**Context:** User reported `no host configured` error despite having a correctly structured
`deploy.yaml`. Most likely cause is a stale binary installed before Phase 5 completed.
Resolve() signature changed in 05-01 — a pre-Phase-5 binary may mishandle config loading.
