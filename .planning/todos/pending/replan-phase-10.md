---
name: replan-phase-10
description: Re-plan phase 10 after ROADMAP.md is amended — invoke /gsd-plan-phase 10 to create fresh plans for static cobra completion + CI generation + brew install
metadata:
  type: project
---

## Re-plan Phase 10

**Depends on:** [[amend-phase-10-roadmap]] must be done first (ROADMAP.md updated).

Run `/gsd-plan-phase 10` to create new plans covering:

1. **Hidden completion subcommand** — add cobra `completion [zsh|bash]` command with
   `Hidden: true`; remove dynamic completion logic from phase 10's original plans

2. **CI generation workflow** — GitHub Actions step that builds the binary and runs
   `docker-deploy completion zsh > contrib/_docker-deploy` and
   `docker-deploy completion bash > contrib/docker-deploy.bash` on release

3. **Homebrew formula update** — add `(share/"zsh/site-functions").install "_docker-deploy"`
   and bash equivalent to the formula in `webcane/homebrew-docker-deploy`

4. **Manual install script** — a shell script (or documented commands) in INSTALL.md
   for users who don't use homebrew

5. **INSTALL.md documentation** — section describing how to enable completions after
   install; no mention of the hidden subcommand

See [[completion-rework-design]] for full design rationale.
