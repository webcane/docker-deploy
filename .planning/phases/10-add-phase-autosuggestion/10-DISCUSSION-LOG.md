# Phase 10: Add Phase Autosuggestion - Discussion Log

> **Audit trail only.** Do not use as input to planning, research, or execution agents.
> Decisions are captured in CONTEXT.md — this log preserves the alternatives considered.

**Date:** 2026-06-01
**Phase:** 10-add-phase-autosuggestion
**Areas discussed:** Feature clarification, Shell coverage, Dynamic --host completions, Completion install UX, Flag value hints

---

## Feature Clarification

| Option | Description | Selected |
|--------|-------------|----------|
| Shell tab completion | cobra's completion subcommand — suggests flags, subcommands, and values when user presses Tab | ✓ |
| Post-deploy next-step hints | After a deploy run, print actionable suggestions | |
| Preflight fix suggestions | When a pre-flight check fails, output the exact fix command | |

**User's choice:** Shell tab completion
**Notes:** Phase description "Add Phase Autosuggestion" was ambiguous; user clarified it means shell completions.

---

## Shell Coverage

| Option | Description | Selected |
|--------|-------------|----------|
| Bash + zsh | The vast majority of Linux/macOS Docker users | ✓ |
| Bash + zsh + fish | Adds fish shell support | |
| All four (+ PowerShell) | Adds PowerShell for Windows users | |

**User's choice:** Bash + zsh
**Notes:** Standard for this user base; install.sh/Homebrew users will have these shells.

---

## Dynamic --host Completions

| Option | Description | Selected |
|--------|-------------|----------|
| Yes — read deploy.yaml + ~/.ssh/config | Dynamic host suggestions reusing Phase 2 and 14 parsers | ✓ |
| Static prefix hint only | Just show ssh://user@host:port as a label | |
| No --host completion | Leave --host as a plain string | |

**User's choice:** Yes — read deploy.yaml + ~/.ssh/config
**Notes:** Both parsers already exist; reuse without additional complexity.

**Error handling follow-up:**

| Option | Description | Selected |
|--------|-------------|----------|
| Silent fallback — no suggestions | Return empty list on error | ✓ |
| Show error in completion output | Return error string as completion entry | |

**User's choice:** Silent fallback
**Notes:** Completion errors crashing Tab is confusing UX; swallow silently.

---

## Completion Install UX

| Option | Description | Selected |
|--------|-------------|----------|
| docker deploy completion <shell> subcommand | Standard piped output pattern | ✓ |
| Documented eval snippet only | No subcommand needed | |
| Homebrew auto-installs completions | Zero user steps but Homebrew-only | |

**User's choice:** docker deploy completion <shell> subcommand

**Visibility follow-up:**

| Option | Description | Selected |
|--------|-------------|----------|
| Visible subcommand at root level | Discoverable via docker deploy --help | ✓ |
| Hidden subcommand | Not shown in --help | |

**User's choice:** Visible at root level
**Notes:** Consistent with kubectl, gh, and standard cobra CLIs.

---

## Flag Value Hints

| Option | Description | Selected |
|--------|-------------|----------|
| Yes — suggest /opt/<cwd-basename> | Smart default matching built-in default logic | ✓ |
| No path suggestions | Leave --path as a plain string | |

**User's choice:** Yes — suggest /opt/<cwd-basename>

**--compose-file follow-up:**

| Option | Description | Selected |
|--------|-------------|----------|
| Yes — scan cwd for compose files | Quick os.ReadDir scan; suggest if files exist | ✓ |
| No file suggestions | Shell's default file completion handles this | |

**User's choice:** Yes — scan cwd
**Notes:** Matches the auto-detect logic already in config resolution.

---

## Claude's Discretion

None — user made all decisions explicitly.

## Deferred Ideas

None — discussion stayed within phase scope.
