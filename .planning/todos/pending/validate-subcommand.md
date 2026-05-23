---
name: validate-subcommand
description: Add `docker deploy validate` subcommand that checks deploy.yaml before attempting a deploy
type: todo
priority: medium
date: 2026-05-23
---

## Task

Implement `docker deploy validate` — a subcommand that reads and validates `deploy.yaml` and reports errors without connecting to the remote host.

## Motivation

Users encounter config mistakes only when a deploy is already in progress. A dedicated validate command surfaces errors early, locally, with no SSH required.

## Behaviour

```
$ docker deploy validate
✓ deploy.yaml is valid

$ docker deploy validate
✗ deploy.yaml is invalid:
  - host: required field is missing
  - port: must be between 1 and 65535 (got: 99999)
```

- Exit code 0 on success, non-zero on validation failure
- Reads config via the existing `Resolve(flags, file, defaults)` path
- No SSH connection — purely local

## Implementation options (decide during planning)

**Option A — rely on `Resolve()` + struct unmarshaling**
- Extend `Resolve()` to return structured validation errors
- No external dependency, single source of truth (the Go struct)

**Option B — bundle a JSON Schema in the binary**
- Schema lives in `internal/config/schema.json`, embedded via `go:embed`
- Validated against the schema using a JSON Schema library (e.g. `santhosh-tekuri/jsonschema`)
- Schema must be kept in sync with the Go struct — maintenance overhead

Either approach is acceptable; decide based on richness of error messages needed.

## Files likely affected

- `cmd/validate.go` — new cobra subcommand
- `internal/config/resolve.go` — surface structured validation errors (Option A) or embed schema (Option B)
- `main.go` — register the subcommand
