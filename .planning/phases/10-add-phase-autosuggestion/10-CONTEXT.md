# Phase 10: Add Phase Autosuggestion - Context

**Gathered:** 2026-06-01
**Status:** Ready for planning

<domain>
## Phase Boundary

Add shell tab completion to the `docker deploy` CLI plugin. Users press Tab to get suggestions for subcommands, flags, and contextually-aware flag values (`--host` from deploy.yaml/~/.ssh/config, `--path` from cwd name, `--compose-file` from cwd scan). Delivered via a `docker deploy completion <shell>` subcommand.

</domain>

<decisions>
## Implementation Decisions

### Shell Coverage
- **D-01:** Support bash and zsh only. No fish or PowerShell in this phase.

### Dynamic --host Completions
- **D-02:** `--host` Tab-completes by reading the project `deploy.yaml` host value and `~/.ssh/config` host aliases at completion time. Both parsers exist already from Phase 2 and Phase 14 — reuse them.
- **D-03:** If reading either file fails (missing, parse error), silently return an empty suggestion list. No errors shown during Tab completion.

### Completion Install UX
- **D-04:** Expose a visible `docker deploy completion <shell>` subcommand at the root level (not hidden). Users pipe its output to their shell's completions directory (e.g., `docker deploy completion bash > ~/.bash_completion.d/docker-deploy`).
- **D-05:** Subcommand is discoverable via `docker deploy --help`. Consistent with kubectl, gh, and other cobra-based CLIs.

### Flag Value Hints
- **D-06:** `--path` suggests `/opt/<cwd-basename>` as a completion candidate (e.g., in `~/projects/myapp`, suggest `/opt/myapp`). Matches the built-in default path resolution logic.
- **D-07:** `--compose-file` scans cwd with a lightweight `os.ReadDir(".")` and suggests `compose.yaml` and/or `docker-compose.yml` if they exist. Matches the auto-detect logic in config resolution.

### Package Structure
- **D-08:** Completion logic lives in its own `internal/completion/` package, not in `cmd/docker-deploy/main.go`. Structure:
  ```
  internal/completion/
  ├── completion.go   — RegisterFlagCompletionFunc calls + dynamic completion functions (--host, --path, --compose-file)
  ├── bash.go         — bash script generation (wraps cmd.Root().GenBashCompletionV2)
  └── zsh.go          — zsh script generation (wraps cmd.Root().GenZshCompletion)
  ```
  `cmd/docker-deploy/main.go` only wires the `completion` subcommand via `buildCompletionCmd()` and calls `completion.Register(cmd)` to attach flag completions. No completion logic in `main.go`.

</decisions>

<canonical_refs>
## Canonical References

**Downstream agents MUST read these before planning or implementing.**

### Existing Parsers to Reuse
- `internal/config/config.go` — `LoadFile()` and `Resolve()` — read `deploy.yaml`; extract `host` value for `--host` completion
- `internal/sshconfig/` — SSH config host alias resolution from Phase 14; produces the alias list for `--host` completion
- `cmd/docker-deploy/main.go` — `buildDeployCmd()` and subcommand registration; this is where the `completion` subcommand is added

### Cobra Completion API
- cobra's `RegisterFlagCompletionFunc()` is the mechanism for dynamic flag value completions
- cobra's `GenBashCompletion()` / `GenZshCompletion()` generate the shell scripts

### Project Conventions
- `CLAUDE.md` — key technical decisions (cobra, plugin.Run pattern)

</canonical_refs>

<code_context>
## Existing Code Insights

### Reusable Assets
- `internal/config.LoadFile()`: reads `deploy.yaml` from cwd; returns `FileConfig` with `Host` field — reuse for `--host` completion
- `internal/sshconfig/`: parses `~/.ssh/config` and returns host alias list — reuse for `--host` completion
- `cmd/docker-deploy/main.go` `buildDeployCmd()`: where all subcommands are registered with `cmd.AddCommand()`

### Established Patterns
- Subcommands follow the `build<Name>Cmd() *cobra.Command` factory pattern (see `buildVersionCmd`, `buildValidateCmd`)
- Errors in completion functions must be swallowed silently — completion crashes are invisible but break UX

### Integration Points
- New `buildCompletionCmd()` registered in `buildDeployCmd()` via `cmd.AddCommand()`
- `RegisterFlagCompletionFunc` calls sit alongside flag definitions in `buildDeployCmd()`

</code_context>

<specifics>
## Specific Ideas

- The `completion` subcommand takes the shell name as a positional arg (`bash` or `zsh`) and writes the script to stdout — standard cobra pattern
- `--host` completion should merge deduplicated suggestions from both deploy.yaml and ~/.ssh/config host list
- All dynamic completion functions must recover from panics/errors and return `(nil, cobra.ShellCompDirectiveNoFileComp)` on failure

</specifics>

<deferred>
## Deferred Ideas

None — discussion stayed within phase scope

</deferred>

---

*Phase: 10-add-phase-autosuggestion*
*Context gathered: 2026-06-01*
