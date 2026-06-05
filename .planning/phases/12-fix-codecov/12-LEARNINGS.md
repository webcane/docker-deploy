---
phase: 12
phase_name: "fix-codecov"
project: "docker-deploy"
generated: "2026-06-05"
counts:
  decisions: 5
  lessons: 3
  patterns: 4
  surprises: 1
missing_artifacts:
  - "12-SECURITY.md (no separate security artifact for this docs-only phase)"
  - "12-HUMAN-UAT.md (no human UAT required; all checks were automated)"
---

# Phase 12 Learnings: Docs Polish

## Decisions

### "remote host" Over "remote VPS" in Plugin Description
The plugin's one-liner description was updated from "Deploy a docker-compose project to a remote VPS" to "Deploy a docker-compose project to a remote host" in both the cobra `Short` field and the Docker CLI `ShortDescription` metadata field.

**Rationale:** "remote VPS" is too narrow — the plugin works with any SSH-accessible host, not only VPS instances; "remote host" is more accurate and inclusive.
**Source:** 12-01-PLAN.md

---

### Install Script as Primary Installation Method in README
The README Installation section was restructured to show only the install script (the two curl commands) as the primary method, with a single one-line link to INSTALL.md for the three other methods.

**Rationale:** Four equally-weighted options in a README overwhelm first-time readers; the install script is the simplest path that works on all platforms.
**Source:** 12-02-PLAN.md

---

### INSTALL.md Extracts Full Install Documentation from README
A dedicated INSTALL.md file was created at the project root, containing all four install methods (Install script, Homebrew, Manual binary, go install) under flat `##` headers without "Option N:" prefixes.

**Rationale:** Concentrating all install methods in one referenced file keeps the README focused while preserving the information for users who need alternative paths.
**Source:** 12-03-PLAN.md

---

### Flat Headers in INSTALL.md (No "Option N:" Prefix)
The four install method sections in INSTALL.md use `## Install script`, `## Homebrew`, `## Manual binary`, `## go install` without any "Option 1:", "Option 2:", etc. numbering.

**Rationale:** Numeric prefixes imply ranking and become brittle when methods are reordered; plain names are more direct and scannable.
**Source:** 12-03-PLAN.md

---

### README Value Proposition Capped at 100 Words
The "What is docker-deploy?" section was rewritten to 60 words (under the 100-word limit), stating who the tool is for, the core value, and the simplicity advantage over CI/CD pipelines.

**Rationale:** A short, focused value proposition reduces cognitive load for first-time visitors; existing text was verbose and repeated information.
**Source:** 12-02-PLAN.md

---

## Lessons

### Both cobra Short and Docker CLI ShortDescription Must Be Updated Together
The plugin description appears in two places: `cmd.Short` (shown by `docker deploy --help`) and `metadata.Metadata.ShortDescription` (returned by `docker-deploy docker-cli-plugin-metadata`). Both must be updated to the same string or they diverge, creating a confusing inconsistency between help text and metadata.

**Context:** Plan 12-01 explicitly identified both locations and required character-for-character identical strings after the change.
**Source:** 12-01-PLAN.md

---

### One Occurrence of go install in README Is Acceptable as Inline Code in a Link Sentence
After removing the `go install` command block from the README, one occurrence of `` `go install` `` remained — as inline code in the INSTALL.md link sentence ("For Homebrew, manual binary download, and `go install`, see INSTALL.md"). This is a reference to the method, not a command block, and is intentional.

**Context:** Verification confirmed the grep check `grep -c "go install" README.md` returns 1 (acceptable) because the remaining occurrence is inline text in the reference link, not a code block.
**Source:** 12-VERIFICATION.md

---

### Documentation-Only Phases Can Achieve Clean 5/5 Verification Without Probes
Phase 12 was entirely documentation. All five must-have truths were verifiable programmatically (grep/wc/awk on markdown files) without any runtime probes or SSH connections. A clean 5/5 score is achievable in a single verification pass for docs-only phases.

**Context:** Verification report noted "no anti-patterns found" and "no human verification required" — automated checks were sufficient.
**Source:** 12-VERIFICATION.md

---

## Patterns

### Parallel Documentation Plans in the Same Wave
Plans 12-02 (update README) and 12-03 (create INSTALL.md) ran in parallel in Wave 1 because they affect different files. Both were independent: README restructuring removes the three non-primary install methods; INSTALL.md creation adds them to the dedicated file. Wave parallelism applies to documentation as readily as code.

**When to use:** When two documentation tasks write to different files and neither depends on the other's output.
**Source:** 12-02-PLAN.md, 12-03-PLAN.md

---

### Feedback/Contribution Section Pattern for Evolving Reference Tables
The COMPARISON.md file has a `## Missing a tool?` section appended at the bottom with a GitHub Issues link. This pattern invites users to suggest additions to a table that may become stale, acknowledging that it is not exhaustive.

**When to use:** Any comparison table, feature matrix, or reference list that is expected to grow over time; a visible feedback link converts passive readers into contributors.
**Source:** 12-04-PLAN.md

---

### Extract Install Options to INSTALL.md to Keep README Focused
When a README Installation section grows beyond two or three options, extract the alternatives to a dedicated INSTALL.md and link to it from the README. The README retains only the primary (recommended) path.

**When to use:** Projects with multiple install methods (package manager, manual binary, language toolchain install, etc.) where the README is becoming a long reference document.
**Source:** 12-02-PLAN.md, 12-03-PLAN.md

---

### Verify Both Plugin Metadata and CLI Help Text After Description Changes
For Docker CLI plugins, a description change must be verified in two ways: `docker deploy --help` (cobra output) and `docker-deploy docker-cli-plugin-metadata` (JSON metadata consumed by `docker plugin`). Both surfaces need to match.

**When to use:** Any change to plugin description strings in main.go that touches `cmd.Short` or `metadata.Metadata.ShortDescription`.
**Source:** 12-01-PLAN.md, 12-VERIFICATION.md

---

## Surprises

### The Description Change Was Already Applied From a Prior Commit
When Plan 12-01 ran, the "remote host" string was already present in `cmd/docker-deploy/main.go` from a prior goreleaser-related commit (`d2c702c`). No code change was needed — the task was complete as-is. The plan's verification grep confirmed the desired state, and no additional commit was required.

**Impact:** The plan executed in near-zero time for Task 1; the value was the explicit test of both locations rather than the code change itself. Pre-existing fixes can be fully credited and documented without redundant commits.
**Source:** 12-01-SUMMARY.md
