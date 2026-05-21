# Phase 6: Init Wizard - Discussion Log

> **Audit trail only.** Do not use as input to planning, research, or execution agents.
> Decisions are captured in CONTEXT.md — this log preserves the alternatives considered.

**Date:** 2026-05-19
**Phase:** 06-init-wizard
**Areas discussed:** Root credential input, Auto-trigger mechanics, deploy.yaml write behavior, Wizard question sequence
**Note:** Area 1 was completed in a prior session (interrupted); Areas 2–4 completed in resume session today.

---

## Root credential input

| Option | Description | Selected |
|--------|-------------|----------|
| Deploy user + sudo | Connect once as deploy user, use sudo for privileged ops | |
| Separate root SSH dial | Prompt root credentials separately, two connections | |
| Either — user decides at runtime | Wizard asks and branches | |
| Free-form | User described the full flow in their own words | ✓ |

**User's choice:** Free-form description of the connection model.
**Notes:** Wizard connects as whatever SSH user is provided. On connect, runs `id -u` to detect root. If root → warning + partial-assist deploy user setup. Checks docker/sudo group membership. Uses sudo only when the target path requires it (error-driven, not path-prefix heuristic). If SSH key not in agent → prompt password. If sudo needed and passwordless sudo not configured → prompt sudo password interactively (wizard has TTY).

---

## Auto-trigger mechanics

| Option | Description | Selected |
|--------|-------------|----------|
| Interrupt deploy, offer to run init | Stop deploy, ask [y/N] to run wizard inline | |
| Fail with clear --init hint | Exit with error + "run --init" message | |
| Auto-run wizard, then continue deploy | Trigger wizard without prompt, then deploy | |
| Free-form | User described --init's dual responsibilities | ✓ |

**User's choice:** Free-form.
**Notes:** `--init` has two responsibilities: (1) add missing params (write deploy.yaml), (2) allow root user to create deploy user + configure it. When `docker deploy` fails because target dir can't be created → show user-friendly error; full OS error via `--verbose`. Do NOT suggest `--init` — it won't help with directory permission failures in the deploy loop. INIT-02's auto-trigger is overridden.

Follow-up: "Should the error suggest --init?" → No. Just a user-friendly error. `--init` is not the remedy for mid-deploy dir creation failures.

---

## deploy.yaml write behavior

### Fields to write

| Option | Description | Selected |
|--------|-------------|----------|
| Minimal: host, user, path | Just enough to run docker deploy | |
| Full: host, user, path + ssh_key, compose_file, project_name | Self-contained, no runtime ~/.ssh resolution needed | ✓ |
| You decide | Let planner figure out the right set | |

**User's choice:** Full field set.

### Overwrite behavior

| Option | Description | Selected |
|--------|-------------|----------|
| Ask before overwriting | Prompt with diff preview, default No | ✓ |
| Always overwrite silently | Replace without asking | |
| Refuse and exit | Hard stop, user must delete manually | |

**User's choice:** Ask before overwriting (with diff preview).

---

## Wizard question sequence

### Root-user deploy user setup

| Option | Description | Selected |
|--------|-------------|----------|
| Full automated setup | Create user, docker group, SSH key, optionally sudoers | |
| Instructions only | Detect + print commands, user runs them manually | |
| Detect + partial assist | Warn, offer specific safe steps, skip sudoers | ✓ |

**User's choice:** Detect + partial assist — warn about root, offer to create deploy user + add to docker group, skip sudoers (print manual command instead).

### Completion behavior

| Option | Description | Selected |
|--------|-------------|----------|
| Write deploy.yaml and exit | Config written, user deploys separately | |
| Offer to run deploy immediately | "deploy.yaml written. Run docker deploy now? [y/N]" | ✓ |
| Auto-run docker deploy | Immediate deploy, no extra prompt | |

**User's choice:** Offer to run deploy immediately.

---

## Claude's Discretion

- Exact `huh` form/group layout (single-page vs multi-step pager)
- Error message copy for user-friendly target-dir failure in deploy loop
- Whether to show a summary of deploy.yaml contents before the "run now?" prompt

## Deferred Ideas

- sudoers auto-configuration — intentionally excluded from `--init` (security risk); print manual command instead
- INIT-02 auto-trigger from deploy failures — overridden; could be re-evaluated later
- SSH config entry write for the host — out of scope for v1
- Verbose deploy user setup feedback — deferred to Phase 7 (verbose flag)
