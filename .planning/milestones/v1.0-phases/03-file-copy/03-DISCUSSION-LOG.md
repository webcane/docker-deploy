# Phase 3: File Copy - Discussion Log

> **Audit trail only.** Do not use as input to planning, research, or execution agents.
> Decisions are captured in CONTEXT.md — this log preserves the alternatives considered.

**Date:** 2026-05-14
**Phase:** 3-File Copy
**Areas discussed:** Filter semantics, Project name resolution, Repeat deploy strategy, deploy.yaml schema placement

---

## Filter Semantics

| Option | Description | Selected |
|--------|-------------|----------|
| Replace defaults | If include: set, use ONLY those files | |
| Extend defaults | User files added on top of defaults | |
| All files minus excludes | Copy everything in cwd, only excludes filter | ✓ |

**User's choice:** "include all files in the current directory by default. list only excludes."

**Follow-up — include: field:**

| Option | Description | Selected |
|--------|-------------|----------|
| Yes — whitelist mode | include: exists as explicit pin | |
| No — exclude-only model | Only exclude: exists | ✓ |

**Follow-up — exclude: extends or replaces defaults:**

| Option | Description | Selected |
|--------|-------------|----------|
| Extend defaults | User excludes added on top of built-ins | ✓ |
| Replace defaults | User list is the only exclude list | |

**Follow-up — glob patterns:**

| Option | Description | Selected |
|--------|-------------|----------|
| Glob patterns | Supports *, **, *.log | ✓ |
| Exact names only | Simpler, more rigid | |

**Follow-up — .gitignore integration:**

User raised: "docker deploy exclude should extend .gitignore but how to copy .env in such case?"

| Option | Description | Selected |
|--------|-------------|----------|
| Ignore .gitignore, own exclude list | Tool has its own defaults; .env copies normally | ✓ |
| Read .gitignore, but always copy .env | .gitignore as base, hardcoded .env exception | |
| Read .gitignore, user manages .env | Full .gitignore respect; high friction | |

**Notes:** .env is in .gitignore in most projects, making .gitignore integration incompatible with the core value proposition. Exclude-only model with tool-owned defaults was chosen. .gitignore integration deferred.

---

## Project Name Resolution

| Option | Description | Selected |
|--------|-------------|----------|
| Current working directory name | filepath.Base(cwd) | ✓ |
| compose.yaml 'name:' field | Matches Docker's naming convention | |
| Explicit 'name:' in deploy.yaml | Portable, explicit, but adds required config step | |

**User's choice:** Current working directory name.
**Notes:** Predictable, no extra parsing needed, CLI-native behavior.

---

## Repeat Deploy Strategy

| Option | Description | Selected |
|--------|-------------|----------|
| Replace only uploaded files | Overwrite counterparts, leave other remote files | |
| Clean slate — wipe and replace | Entire target replaced, maximum reproducibility | ✓ (with confirmation) |

**User's choice:** "cli should ask to replace target contents. or use optional flag --force to replace it automatically"

**Follow-up — what does the confirmation cover:**

| Option | Description | Selected |
|--------|-------------|----------|
| Confirm overwriting individual files | Show N files to update, proceed? | |
| Confirm wiping the entire directory | Target exists, replace all contents? | ✓ |

**Follow-up — when does the prompt appear:**

| Option | Description | Selected |
|--------|-------------|----------|
| Only when target already exists | First deploy silent, repeat deploys prompt | ✓ |
| Every deploy | Always ask regardless | |

**User addition:** "it's possible to define force flag in config deploy.yaml to simplify cli command"
**Notes:** `force: true` in deploy.yaml permanently skips the replace-confirmation prompt.

---

## deploy.yaml Schema Placement

| Option | Description | Selected |
|--------|-------------|----------|
| Under 'target:' alongside host/path | All per-target config grouped; multi-target compatible | ✓ |
| Top-level (global) | Flatter; conflicts with D-10 forward-compat | |

**User's choice:** Under `target:`.

**Follow-up — --exclude CLI flag:**

| Option | Description | Selected |
|--------|-------------|----------|
| deploy.yaml only | Excludes are persistent config | |
| --exclude flag too | One-off exclusions, follows established precedence | ✓ |

---

## Claude's Discretion

- Upload output format (per-file log vs. summary count vs. progress bar)
- SSH `mv`/`rename` strategy for atomic directory swap
- Whether to show file count before the replace confirmation prompt

## Deferred Ideas

- `.gitignore` integration — explored and rejected; conflicts with .env copy requirement
- `include:` whitelist mode — rejected in favor of exclude-only; could return for monorepo use cases in v2
