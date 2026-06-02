---
name: amend-phase-10-roadmap
description: Update ROADMAP.md Phase 10 to reflect the static cobra + CI-generated + brew-install completion approach; mark phase as not complete
metadata:
  type: project
---

## Amend Phase 10 in ROADMAP.md

Phase 10 is currently marked complete with a dynamic-completion goal. Reopen it to match
the new design direction decided in [[completion-rework-design]].

### Changes needed

1. **Update the phase goal** — from dynamic completions (deploy.yaml + ssh/config reads)
   to static cobra-generated completion files installed via homebrew formula

2. **Update success criteria** — remove dynamic `--host`, `--path`, `--compose-file`
   suggestion criteria; replace with:
   - Tab completion works in zsh after homebrew install (no user fpath config)
   - Tab completion works in bash after homebrew install
   - `_docker-deploy` and `docker-deploy.bash` are committed to the repo under `contrib/`
   - Homebrew formula installs via `share/zsh/site-functions/`
   - Manual install script documented in INSTALL.md

3. **Remove or revise plan references** — the existing plans (10-01, 10-02) used the
   dynamic approach; mark them superseded or remove

4. **Uncheck the phase completion checkbox** — set phase 10 back to `[ ]` (not done)

### Follow-up

After this todo is done, run `/gsd-plan-phase 10` to create fresh plans.
See [[replan-phase-10]].
