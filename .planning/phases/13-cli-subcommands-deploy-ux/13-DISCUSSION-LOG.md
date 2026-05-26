# Phase 13: CLI Subcommands & Deploy UX - Discussion Log

> **Audit trail only.** Do not use as input to planning, research, or execution agents.
> Decisions are captured in CONTEXT.md — this log preserves the alternatives considered.

**Date:** 2026-05-26
**Phase:** 13-CLI-Subcommands-Deploy-UX
**Areas discussed:** version output format, validate scope, verbose pre-confirm diff, sudo consolidation rollback

---

## version output format

| Option | Description | Selected |
|--------|-------------|----------|
| Bare version: `v0.6.3` | Single line, version tag only | |
| Prefixed: `docker-deploy v0.6.3` | Program name + version | |
| Docker CLI style (full) | Version, git commit, build time, OS/arch | ✓ |

**User's choice:** Full Docker CLI-style output (provided exact format):
```
Docker Deploy Version v1.0.0
  Git commit:        de40ad0
 Built:             Mon Apr 20 14:57:44 2026
 OS/Arch:           darwin/arm64
```

**Follow-up — ldflags fields:**
| Option | Selected |
|--------|----------|
| Version + Git commit + Build time (all three via ldflags) | ✓ |
| Version + Git commit only | |
| Version only | |

**Follow-up — dev build fallback:**
| Option | Selected |
|--------|----------|
| `dev` version + short commit hash | ✓ |
| Commit hash as the version string | |

**Follow-up — subcommand vs flag:**
| Option | Selected |
|--------|----------|
| Subcommand only (`docker deploy version`) | ✓ |
| Both subcommand and `--version` flag | |

**Notes:** User had a clear vision from the start — provided the exact output format without prompting. Docker CLI-style is the intent.

---

## validate scope

| Option | Description | Selected |
|--------|-------------|----------|
| YAML parse only | Just check if deploy.yaml can be unmarshalled | (initial selection) |
| Run config.Resolve() — all existing validations | ParseHost, absolute path, health values | ✓ (clarified) |
| Field semantics | Host URL format, path validity | (covered by Resolve) |
| Compose file exists locally | Check referenced compose file exists | |
| No deploy.yaml = error | Fail if no deploy.yaml found | ✓ |

**User's choice:** Run `config.Resolve()` (all existing validations). Initially selected "YAML parse only" but on follow-up clarified the intent is to reuse what Resolve() already validates — no new logic. Missing deploy.yaml → error (not valid).

**Notes:** The "field errors" in SC-5 means whatever Resolve() currently rejects — no new validation needed. Zero-config is not valid for `validate`; the command is only useful when a deploy.yaml exists.

---

## verbose pre-confirm diff

| Option | Description | Selected |
|--------|-------------|----------|
| Two plain lists: Local / Remote | Simple filenames under each header | ✓ |
| Diff-style (+/-) | Lines prefixed with + or − showing changes | |

**User's choice:** Two plain lists.

**Follow-up — truncation:**
| Option | Selected |
|--------|----------|
| Truncate at 20 with count + suggest extending excludes | ✓ |
| Show all files | |

**Follow-up — first deploy (no remote):**
| Option | Selected |
|--------|----------|
| Show `Remote files: (none)` | ✓ |
| Skip remote section | |

**Notes:** Truncation message verbatim from user: "truncate to 20. show warn. ask to extend exclude list if necessary" — captured as `... and N more — consider adding patterns to deploy.yaml exclude list`.

---

## sudo consolidation rollback

Discussion was interrupted by user pointing to `.planning/todos/pending/2026-05-26-sudo-exec-refactor.md`, which specifies a different (and better) approach than the compound-command option being discussed.

**User's direction:** "keep rollback, but use ssh password differently"

Final decisions from todo + follow-up:

| Option | Description | Selected |
|--------|-------------|----------|
| One compound command: `sudo bash -c 'mv && mv && rm'` | Single sudo auth, loses granular rollback | (asked, superseded by todo) |
| SudoExec refactor: exported function, `SudoCreds` type | Keep rollback, fix password storage | ✓ |

**Follow-up — cached password step order:**
| Option | Selected |
|--------|----------|
| direct → cached pw → sudo -n → interactive | ✓ |
| direct → sudo -n → cached pw → interactive | |

**Follow-up — plan dependency:**
| Option | Selected |
|--------|----------|
| 13-04 first (SudoExec), then 13-06 uses it | ✓ |
| Independent / parallel | |

**Notes:** The todo (`2026-05-26-sudo-exec-refactor.md`) fully specifies the refactor. The key insight: keep granular rollback (N separate SudoExec calls), just fix the password storage (`[]byte` + `SudoCreds.Zero()`) and make the function exported/testable. This also structurally fixes the `feedback_sudo_rollback.md` issue — error/rollback paths now use `SudoExec` by default.

---

## Claude's Discretion

None — all decisions were made explicitly by the user.

---

## Deferred Ideas

- **`ssh_dial_timeout` config field** — `.planning/todos/pending/2026-05-26-ssh-dial-timeout-config-field.md` — add `SSHDialTimeout` to `TargetConfig`. Reviewed, not folded. Out of scope for Phase 13.
