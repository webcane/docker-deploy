# Phase 10: Shell Completion Rework - Discussion Log

> **Audit trail only.** Do not use as input to planning, research, or execution agents.
> Decisions are captured in CONTEXT.md — this log preserves the alternatives considered.

**Date:** 2026-06-02
**Phase:** 10-add-phase-autosuggestion
**Session:** Rework discussion (supersedes 2026-06-01 log)
**Areas discussed:** Dynamic completions fate, Package structure, contrib/ files in git, Manual install method

---

## Dynamic Completions Fate

| Option | Description | Selected |
|--------|-------------|----------|
| Remove them | Static script completes only flag names; no value suggestions. Simpler, truly static. | ✓ |
| Keep them | Static script embeds __complete callbacks; runtime reads of deploy.yaml + ~/.ssh/config at Tab-press. | |
| Keep, but only --host | Remove --path and --compose-file; keep --host only. | |

**User's choice:** Remove them

**Notes:** Followed the "no runtime reads" principle from the rework design note. The `internal/completion/completion.go` file is deleted in full; the package is kept (bash.go + zsh.go) for future extension.

---

## Package Structure (follow-up)

| Option | Description | Selected |
|--------|-------------|----------|
| Inline into main.go | Delete internal/completion/ entirely; call cobra funcs directly in main.go. | |
| Keep the package | Preserve internal/completion/ for future extension (fish, PowerShell). | ✓ |

**User's choice:** Keep the package

---

## contrib/ Files in Git

| Option | Description | Selected |
|--------|-------------|----------|
| Committed to repo | Generated locally, committed before tagging; always browsable in git. | ✓ |
| Release tarball only | Only in .tar.gz archives; simpler CI but not in git. | |

**User's choice:** Committed to repo

**Notes (follow-up on trigger):** User specified: add `make completions` target; run locally as part of `/gsd:release-tag` skill only — no automated CI commit step. Update homebrew-docker-deploy formula in `.goreleaser.yaml` if necessary.

---

## Manual Install Method

| Option | Description | Selected |
|--------|-------------|----------|
| INSTALL.md copy-paste commands | Documented section with download + symlink commands. | |
| contrib/install-completions.sh script | Shell script that downloads and places the correct file. | ✓ |

**User's choice:** contrib/install-completions.sh script

---

## Claude's Discretion

- Script logic in `contrib/install-completions.sh` (auto-detect shell, pick correct file, handle homebrew vs manual path)
- Exact Makefile target implementation
- goreleaser `extra_files` configuration

## Deferred Ideas

None — discussion stayed within phase scope
