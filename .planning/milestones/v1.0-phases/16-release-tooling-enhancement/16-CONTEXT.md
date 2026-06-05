# Phase 16: Release Tooling Enhancement - Context

**Gathered:** 2026-05-27
**Status:** Ready for planning

<domain>
## Phase Boundary

Extend the `/gsd:release-tag` skill (`.claude/commands/gsd/release-tag.md`) with:

1. **Wave 0 — Pre-release checks**: run `go test`, `golangci-lint`, and optionally `test-ci` (integration tests, Docker auto-detected) before the user is prompted for a version bump. Hard abort on failure — no files touched.
2. **Wave 1 — STATE.md update**: update `last_updated` and `last_activity` in `STATE.md` as part of the release commit. Commit message body derived from `git log $PREV_TAG..HEAD` (feat/fix/refactor commits, chore excluded).
3. **Wave 3 — Extended linter coverage**: already shipped to master (`b5b1c17`, `1af48a0`). Planner should verify current `.golangci.yml` state and add only what's missing.
4. **Wave 2 — Terminal demo**: SKIPPED.

This phase touches only one production Go file indirectly (`.golangci.yml` for lint config, already done) and one GSD skill file (`.claude/commands/gsd/release-tag.md`). No new Go packages or tests.

</domain>

<decisions>
## Implementation Decisions

### Pre-release checks (Wave 0)
- **D-01:** Checks run BEFORE the version bump question — fail fast, no interactive steps wasted
- **D-02:** `go test ./...` → hard abort on any failure; print which tests failed
- **D-03:** `golangci-lint run ./...` → if issues found, run `make lint-fix` automatically; re-run lint; if still issues → hard abort; only abort on non-auto-fixable issues (matches existing Wave 0 spec)
- **D-04:** `test-ci` → auto-detect Docker socket (`[ -S /var/run/docker.sock ] || [ -S $HOME/.colima/default/docker.sock ]`); if Docker absent, print warning and skip with non-blocking note; if Docker present, run and hard-abort on failure
- **D-05:** Each check prints its name before running (e.g., `▶ go test ./...`); result is PASS or FAIL inline

### Commit message body (Wave 1)
- **D-06:** Body source: `git log $PREV_TAG..HEAD --oneline`
- **D-07:** Filter: exclude lines starting with `chore:` or `chore(` — keep feat/fix/refactor/docs/test/ci/perf
- **D-08:** If no filtered commits exist (all were chores), body is omitted — subject line only
- **D-09:** Format:
  ```
  chore: bump version to v0.9.4

  Changes since v0.9.3:
  - feat: add validate subcommand
  - fix: path-aware sudo detection
  ```

### STATE.md update (Wave 1)
- **D-10:** Update `last_updated:` field to ISO 8601 timestamp of release (e.g., `"2026-05-27T14:30:00Z"`)
- **D-11:** Update `last_activity:` field to `{YYYY-MM-DD} -- Released {NEXT_TAG}`
- **D-12:** Do NOT update `milestone:` — it tracks planning milestones (v1.0), not semver releases
- **D-13:** STATE.md staged and committed in the SAME release commit as README.md + INSTALL.md

### Terminal demo (Wave 2)
- **D-14:** SKIPPED — Wave 2 deferred; no terminal demo in this phase

### Wave 3 status
- **D-15:** Extended linter coverage (gosec, ineffassign, unused, bodyclose, noctx, gocritic, revive, errorlint, wrapcheck, gocognit, nestif, prealloc) already committed to master. Planner must verify `.golangci.yml` against the Wave 3 criteria list and confirm all 16 success criteria pass.

</decisions>

<canonical_refs>
## Canonical References

**Downstream agents MUST read these before planning or implementing.**

### Release skill (primary target)
- `.claude/commands/gsd/release-tag.md` — the skill being extended; read current step structure before adding Wave 0 and Wave 1 steps

### Linter config (Wave 3 verification)
- `.golangci.yml` — current linter config; verify Wave 3 linters (gosec, ineffassign, etc.) are present

### Build system
- `Makefile` — `test`, `test-ci`, `lint`, `lint-fix` targets used by Wave 0 checks

### Planning state
- `.planning/STATE.md` — fields being updated (last_updated, last_activity); read schema before editing
- `.planning/ROADMAP.md` — Phase 16 success criteria (criteria 1-16) define acceptance

</canonical_refs>

<code_context>
## Existing Code Insights

### Reusable Assets
- `Makefile` targets: `test`, `test-ci`, `lint`, `lint-fix` — Wave 0 should invoke these directly rather than raw `go test` calls, keeping behaviour consistent with CI
- Docker detection logic in `Makefile`'s `test-ci` target — reuse the same socket detection pattern for the release skill's test-ci auto-skip logic

### Established Patterns
- Release commit convention: `chore: bump version to $TAG` — preserve as subject line; body is additive
- STATE.md uses YAML frontmatter for structured fields + Markdown body for narrative; update only frontmatter fields `last_updated` and `last_activity`

### Integration Points
- `.claude/commands/gsd/release-tag.md` Steps 1-9 — new Wave 0 check steps insert before Step 1 (version detection); Wave 1 STATE.md update inserts inside Step 5 (file updates) alongside README.md/INSTALL.md

</code_context>

<specifics>
## Specific Ideas

- **Commit message body pattern**: follow gsd-ship's PR body approach (read from artifacts) but simplified to git log for a version bump commit — no PR, direct commit to main
- **test-ci Docker detection**: reuse the exact socket-detection condition from `Makefile`'s `test-ci` target to stay consistent with how CI and local dev detect Docker

</specifics>

<deferred>
## Deferred Ideas

- **Terminal demo (Wave 2)**: vhs tape file producing a GIF embedded in README.md — deferred by user decision; note for a future phase if README engagement metrics suggest it's worth the maintenance cost

</deferred>

---

*Phase: 16-release-tooling-enhancement*
*Context gathered: 2026-05-27*
