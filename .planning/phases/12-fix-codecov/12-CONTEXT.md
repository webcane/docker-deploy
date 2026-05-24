# Phase 12: Docs Polish - Context

**Gathered:** 2026-05-24
**Status:** Ready for planning

<domain>
## Phase Boundary

Four independent documentation edits delivered in one pass:
1. **Help text** — Refine the plugin's Short description (cobra + metadata); no Long description added
2. **README** — Sharpen value prop; restructure install section to show install script as primary method + link to INSTALL.md; simplify all install headers
3. **INSTALL.md** — Create from the existing README install section; use simplified flat headers; cover all 4 methods
4. **COMPARISON.md** — Add a feedback/contribution section at the bottom linking to GitHub Issues

This phase adds NO new plugin features or flags.

</domain>

<decisions>
## Implementation Decisions

### Help Text (Plan 12-01)

- **D-01:** Refine only the `Short` string in both `cmd.Short` (cobra) and `metadata.Metadata.ShortDescription` — keep them identical
- **D-02:** No `Long` description added — current `docker deploy --help` output is considered fine; only the one-liner needs tuning
- **D-03:** Direction for the new Short: "Deploy a docker-compose project to a remote host" — drop "VPS", use "remote host" to be more generic

### README Restructure (Plans 12-02, 12-03)

- **D-04:** Keep the install script as the primary install method shown in README; the other three methods (Homebrew, Manual binary, `go install`) move entirely to INSTALL.md
- **D-05:** README install section becomes: install script command block → brief note linking to INSTALL.md for all methods
- **D-06:** Simplify all install section headers — drop the "Option N:" prefix. Format: `## Homebrew` not `## Option 1: Homebrew (macOS / Linux)`; add platform hint as a subtitle/paragraph if needed
- **D-07:** Apply the same simplified header style throughout INSTALL.md — consistent with README

### COMPARISON.md (Plan 12-04)

- **D-08:** Add a `## Missing a tool?` (or equivalent) section at the **bottom** of COMPARISON.md — dedicated section, not an inline note
- **D-09:** Section links to the repo's GitHub Issues page for users to suggest tool additions
- **D-10:** No changes to the comparison table itself — content is current

</decisions>

<canonical_refs>
## Canonical References

**Downstream agents MUST read these before planning or implementing.**

### Source Files to Modify

- `cmd/docker-deploy/main.go` — contains `buildDeployCmd()` (cobra `Short`) and `metadata.Metadata{ShortDescription:}` — both must be updated to D-03 wording
- `README.md` — current install section to be restructured per D-04–D-07; value prop paragraph at top to be reviewed
- `COMPARISON.md` — feedback section to be added at bottom per D-08–D-09

### File to Create

- `INSTALL.md` — new file; created from README install section content; simplified headers per D-06–D-07

### No external specs

Requirements fully captured in decisions above.

</canonical_refs>

<code_context>
## Existing Code Insights

### Reusable Assets
- `cmd/docker-deploy/main.go` line 37: `ShortDescription: "Deploy a docker-compose project to a remote VPS"` — update to D-03 wording
- `cmd/docker-deploy/main.go` line 57: `Short: "Deploy a docker-compose project to a remote VPS"` — update to same D-03 wording

### Established Patterns
- README already uses `## Installation` with 4 subsections — the restructure changes content under that header, not the header itself
- COMPARISON.md ends with `## When NOT to use docker-deploy` — new feedback section goes after that

### Integration Points
- INSTALL.md must be linked from README (a relative markdown link `[INSTALL.md](INSTALL.md)`)
- GitHub Issues URL for COMPARISON.md feedback: `https://github.com/webcane/docker-deploy/issues`

</code_context>

<specifics>
## Specific Ideas

- Install section headers: flat format without numbering — `## Homebrew`, `## Install script`, `## Manual binary`, `## go install`
- macOS platform note for Homebrew can stay as a subtitle line or brief paragraph, not a header-level qualifier
- The feedback section heading the user suggested will be in the `## Missing a tool?` style — welcoming, not formal

</specifics>

<deferred>
## Deferred Ideas

None — discussion stayed within phase scope.

</deferred>

---

*Phase: 12-Docs-Polish*
*Context gathered: 2026-05-24*
