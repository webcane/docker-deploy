---
created: 2026-06-02T19:19:08Z
title: "Add flag to do clean-up on remote"
area: general
files: []
---

## Problem

After a deploy, stale files may remain on the remote (old images, orphaned compose services,
leftover `.deploy-tmp-*` staging directories). There's currently no way to trigger a
clean-up from the deploy command — users have to SSH in manually.

## Solution

Add a `--cleanup` (or `--prune`) flag to the `deploy` command that, when set, runs
clean-up steps on the remote after a successful deploy. Likely steps:

- Remove the staging directory if it somehow survived an earlier run (belt-and-suspenders)
- Run `docker compose down --remove-orphans` for services no longer in the compose file
- Optionally run `docker image prune -f` to reclaim disk space

Flag should be opt-in (off by default) since pruning images is destructive and
can break rollback scenarios.
