# Phase 10: Shell Completion Rework - Context

**Gathered:** 2026-06-02
**Status:** Ready for planning

<domain>
## Phase Boundary

Replace the original dynamic-completion implementation with static cobra-generated completion files. Deliver:
- A hidden `completion [zsh|bash]` subcommand that writes the generated script to stdout
- A `make completions` target that generates `contrib/_docker-deploy` (zsh) and `contrib/docker-deploy.bash` (bash)
- `/gsd:release-tag` integration — `make completions` runs before tagging and the `contrib/` files are committed
- Goreleaser includes the `contrib/` files in release tarballs; the homebrew formula installs the zsh file via `share/"zsh/site-functions"` (zero user fpath config)
- A `contrib/install-completions.sh` script for non-homebrew users
- An INSTALL.md section describing how to enable completions

</domain>

<decisions>
## Implementation Decisions

### Shell Coverage
- **D-01:** bash and zsh only. Fish and PowerShell are out of scope for this phase.

### Completion Subcommand Visibility
- **D-02:** The `completion [zsh|bash]` subcommand is hidden (`Hidden: true`, `DisableFlagsInUseLine: true`). It does NOT appear in `docker deploy --help`. It is used only by `make completions` and the release pipeline — not documented in README.

### Dynamic Flag Completions — Removed
- **D-03:** Remove all `RegisterFlagCompletionFunc` hooks (`--host`, `--path`, `--compose-file`). Delete `internal/completion/completion.go` entirely (HostCompletionFunc, PathCompletionFunc, ComposeFileCompletionFunc, dedupStrings). Remove the `completion.Register(cmd)` call from `cmd/docker-deploy/main.go`. The generated static script completes only flag names and subcommand names — no value suggestions.

### Package Structure
- **D-04:** Keep the `internal/completion/` package but reduce it to `bash.go` and `zsh.go` only. No `completion.go` (deleted per D-03). The package is retained for future extension (e.g., fish).

### contrib/ File Generation
- **D-05:** Add a `make completions` Makefile target. It builds the binary (or uses the one already in PATH), runs `docker-deploy completion zsh > contrib/_docker-deploy` and `docker-deploy completion bash > contrib/docker-deploy.bash`, then stages both files. The `/gsd:release-tag` skill runs this target before creating the git tag and includes the `contrib/` files in the release commit.

### Release Pipeline
- **D-06:** No automated CI step generates or commits `contrib/` files. Generation happens locally via `make completions` as part of the `/gsd:release-tag` flow. Goreleaser picks up the `contrib/` files and includes them in the release tarball via `extra_files` in `.goreleaser.yaml`.

### Homebrew Formula
- **D-07:** Update the `brews.install` block in `.goreleaser.yaml` to add:
  ```
  (share/"zsh/site-functions").install "_docker-deploy"
  (share/"bash-completion/completions").install "docker-deploy.bash"
  ```
  This installs completions automatically on `brew install` — zero user configuration required on homebrew-managed macOS.

### Manual Install Script
- **D-08:** Add `contrib/install-completions.sh` — a shell script that downloads the completion file from the latest release tarball and places it in the correct fpath location (`/opt/homebrew/share/zsh/site-functions/` for Homebrew macOS, `~/.zsh/completions/` fallback). For non-homebrew users.

### Documentation
- **D-09:** INSTALL.md gets a "Shell Completions" section. README.md does NOT mention completions. The hidden subcommand is not mentioned anywhere in user-facing docs.

</decisions>

<canonical_refs>
## Canonical References

**Downstream agents MUST read these before planning or implementing.**

### Core files to modify
- `cmd/docker-deploy/main.go` — remove `completion.Register(cmd)` call; update `buildCompletionCmd()` to set `Hidden: true`
- `internal/completion/completion.go` — DELETE this file (D-03)
- `internal/completion/bash.go` — keep, no change needed
- `internal/completion/zsh.go` — keep, no change needed
- `.goreleaser.yaml` — add `extra_files` for `contrib/` and update `brews.install` block (D-06, D-07)
- `Makefile` — add `completions` target (D-05)

### New files to create
- `contrib/_docker-deploy` — generated zsh completion script (committed, D-05)
- `contrib/docker-deploy.bash` — generated bash completion script (committed, D-05)
- `contrib/install-completions.sh` — manual install script (D-08)
- `INSTALL.md` (existing) — add "Shell Completions" section (D-09)

### Design rationale
- `.planning/notes/completion-rework-design.md` — original rework design decisions; MUST read before planning
- `CLAUDE.md` — key technical decisions (cobra, plugin.Run pattern)

### Release flow reference
- `.planning/todos/pending/replan-phase-10.md` — lists all deliverables for this rework
- `.planning/todos/pending/amend-phase-10-roadmap.md` — roadmap amendment requirements

</canonical_refs>

<code_context>
## Existing Code Insights

### Reusable Assets
- `internal/completion/bash.go` `GenerateBash()` — wraps `cmd.Root().GenBashCompletionV2()`; keep as-is
- `internal/completion/zsh.go` `GenerateZsh()` — wraps `cmd.Root().GenZshCompletion()`; keep as-is
- `cmd/docker-deploy/main.go` `buildCompletionCmd()` — already exists; just add `Hidden: true` and remove the `completion.Register(cmd)` call

### Code to Delete
- `internal/completion/completion.go` — entire file (HostCompletionFunc, PathCompletionFunc, ComposeFileCompletionFunc, dedupStrings, Register)
- The `completion.Register(cmd)` call in `main.go` imports `"github.com/webcane/docker-deploy/internal/sshconfig"` and `config.LoadFile` — those imports in main.go stay (used elsewhere), but the completion package's dependency on them disappears

### Established Patterns
- Subcommands follow `build<Name>Cmd() *cobra.Command` factory pattern
- All test files in same package directory — `internal/completion/` tests will need to drop completion_test.go (tests for deleted funcs)

### Integration Points
- `buildCompletionCmd()` in main.go: add `Hidden: true` to the returned `*cobra.Command`
- Remove `completion.Register(cmd)` line from `buildDeployCmd()` in main.go
- `.goreleaser.yaml` `brews.install` block: add the two completion install lines after `bin.install`

</code_context>

<specifics>
## Specific Ideas

- `make completions` should be runnable locally even before a release (useful for testing the generated script)
- The `contrib/install-completions.sh` script should auto-detect the shell and only install the relevant file
- The INSTALL.md completions section should mention homebrew first, then the manual script fallback

</specifics>

<deferred>
## Deferred Ideas

None — discussion stayed within phase scope

</deferred>

---

*Phase: 10-add-phase-autosuggestion*
*Context gathered: 2026-06-02*
