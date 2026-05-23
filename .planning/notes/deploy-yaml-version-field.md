---
title: deploy.yaml version field — intentionally dormant
date: 2026-05-23
context: explore session — deploy config versioning
---

# deploy.yaml version field — intentionally dormant

## Decision

The `version` field in `deploy.yaml` is parsed into the config struct but never checked.
It is **not documented**, **not emitted by `init`**, and **not visible to users**.

## Rationale

There is no current need to branch on config schema version. Exposing a field
that does nothing would confuse users ("what version should I set?").

## Forward-compat contract

If the config schema ever requires a breaking change:

- Absence of the field **implicitly means version 1**
- The parser is already set up to read the value — no struct change needed
- At that point, document the field and emit it from `init`

## What this is NOT

This has nothing to do with `SchemaVersion` in `docker-cli-plugin-metadata`.
That field is the Docker CLI plugin protocol version (fixed at `"0.1.0"`) and
is unrelated to `deploy.yaml` config schema versioning.
