# Phase 12: Docs Polish - Discussion Log

> **Audit trail only.** Do not use as input to planning, research, or execution agents.
> Decisions are captured in CONTEXT.md — this log preserves the alternatives considered.

**Date:** 2026-05-24
**Phase:** 12-Docs-Polish
**Areas discussed:** Help text depth, README split strategy, COMPARISON.md additions

---

## Help Text Depth

| Option | Description | Selected |
|--------|-------------|----------|
| Short only — better wording | Improve the one-liner. No Long. Clean and minimal — consistent with other docker CLI plugins. | ✓ |
| Short + Long with flags summary | Add a Long description paragraph that explains the deploy flow. | |
| Short + Long with examples | Add Long + an Examples field showing 2-3 common invocations. | |

**User's choice:** Short only — improve wording only
**Notes:** User tried "Short + Long with flags summary" initially, then revised: "output of 'docker deploy --help' looks fine. tune only short description." Direction for new Short: "Deploy a docker-compose project to a remote host" (drop "VPS", use "remote host"). Both `cmd.Short` and `metadata.Metadata.ShortDescription` kept identical.

---

## README Split Strategy

| Option | Description | Selected |
|--------|-------------|----------|
| Quick-start one-liner + link | Keep one Homebrew command as primary, then link to INSTALL.md. | |
| Just a link, no install commands | Remove all install commands from README entirely. | |
| Keep install script + link | Keep install script as primary in README, link to INSTALL.md for the rest. | ✓ |

**User's choice:** Keep install script as primary install method in README; link to INSTALL.md for all methods.
**Notes:** User also specified header simplification: remove "Option N:" prefix from install headers. New format: `## Homebrew`, `## Install script`, etc. Platform qualification (macOS/Linux) becomes a subtitle or paragraph, not a header-level qualifier. Apply same simplified header style in INSTALL.md.

---

## COMPARISON.md Additions

| Option | Description | Selected |
|--------|-------------|----------|
| Bottom of file — dedicated section | Add `## Missing a tool?` section at the end with GitHub Issues link. | ✓ |
| Just above or below the table | Inline italicized note near the table. | |
| Top of file | Brief note at very top before the table. | |

**User's choice:** Bottom of file — dedicated section
**Notes:** Link to `https://github.com/webcane/docker-deploy/issues`. No changes to comparison table content — table is current.

---

## Claude's Discretion

None — all areas had explicit user direction.

## Deferred Ideas

None — discussion stayed within phase scope.
