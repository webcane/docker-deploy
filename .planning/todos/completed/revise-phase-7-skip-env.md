---
name: revise-phase-7-skip-env
description: Update Phase 7 plans — remove SkipEnv fields, add remote .env existence check + --force bypass confirmation
metadata:
  type: project
---

# Revise Phase 7 Plans: Remove SkipEnv, Add Contextual Confirmation

## What

Update Phase 7 before executing it. The `--skip-env` approach was replaced (see [[skip-env-via-exclude]]).

## Changes to 07-01-PLAN.md

Remove all SkipEnv scope:

- Remove `SkipEnv bool` from `FlagOpts` struct
- Remove `SkipEnv bool` from `Config` struct
- Remove `SkipEnv bool yaml:"skip_env"` from `TargetConfig`
- Remove `cfg.SkipEnv = opts.SkipEnv || file.Target.SkipEnv` from `Resolve()`
- Remove `skipEnv bool` param from `mergeExcludes()` — no `.env` injection at config layer
- Remove related tests: `TestResolveSkipEnv`

Keep everything else: `FlagOpts` refactor, expanded `defaultExcludes`, `Verbose` field.

## Changes to 07-02-PLAN.md

Add to Wire-up plan: contextual `.env` confirmation in the deploy flow.

- After config is resolved and excludes are known, check if `.env` is in effective excludes
- If yes: SSH to remote, stat `<target_path>/.env`
- If remote `.env` exists AND `--force` is false → print warning + prompt "Remote .env will not be updated. Continue? [y/N]"
- If `--force` → skip confirmation, proceed silently
- This check runs before SFTP upload begins

## Changes to ROADMAP.md

Update Phase 7 success criteria to reflect the new design.

## Priority

High — Phase 7 is planned and ready to execute. Plans must be revised before execution starts.
