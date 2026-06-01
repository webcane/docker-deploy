# Phase 10: Add Phase Autosuggestion - Research

**Researched:** 2026-06-01
**Domain:** cobra shell completion (bash/zsh), dynamic flag value completion
**Confidence:** HIGH

## Summary

This phase adds shell tab completion to the `docker deploy` CLI plugin. The implementation has three parts: (1) a `completion` subcommand that writes a shell script to stdout, (2) `RegisterFlagCompletionFunc` registrations on `buildDeployCmd()` for `--host`, `--path`, and `--compose-file`, and (3) a new `ListHosts` function in `internal/sshconfig/` that enumerates all aliases from `~/.ssh/config` (the existing `LookupHost` only matches by alias and cannot enumerate).

The cobra API at v1.10.2 is straightforward. `Completion` is a `= string` type alias, so completion functions return `([]string, cobra.ShellCompDirective)` at the wire level. No new dependencies are needed — cobra is already in go.mod.

**Primary recommendation:** Add `sshconfig.ListHosts(configPath string) []string` to enumerate aliases, then wire `RegisterFlagCompletionFunc` for three flags inside `buildDeployCmd()`, and add `buildCompletionCmd()` registered via `cmd.AddCommand()`.

<user_constraints>
## User Constraints (from CONTEXT.md)

### Locked Decisions
- **D-01:** Support bash and zsh only. No fish or PowerShell in this phase.
- **D-02:** `--host` Tab-completes by reading the project `deploy.yaml` host value and `~/.ssh/config` host aliases at completion time. Both parsers exist already from Phase 2 and Phase 14 — reuse them.
- **D-03:** If reading either file fails (missing, parse error), silently return an empty suggestion list. No errors shown during Tab completion.
- **D-04:** Expose a visible `docker deploy completion <shell>` subcommand at the root level (not hidden). Users pipe its output to their shell's completions directory (e.g., `docker deploy completion bash > ~/.bash_completion.d/docker-deploy`).
- **D-05:** Subcommand is discoverable via `docker deploy --help`. Consistent with kubectl, gh, and other cobra-based CLIs.
- **D-06:** `--path` suggests `/opt/<cwd-basename>` as a completion candidate (e.g., in `~/projects/myapp`, suggest `/opt/myapp`). Matches the built-in default path resolution logic.
- **D-07:** `--compose-file` scans cwd with a lightweight `os.ReadDir(".")` and suggests `compose.yaml` and/or `docker-compose.yml` if they exist. Matches the auto-detect logic in config resolution.

### Claude's Discretion
None — all decisions were locked during discussion.

### Deferred Ideas (OUT OF SCOPE)
None — discussion stayed within phase scope.
</user_constraints>

<phase_requirements>
## Phase Requirements

| ID | Description | Research Support |
|----|-------------|------------------|
| D-01 | bash and zsh shells only | cobra v1.10.2 `GenBashCompletion`/`GenBashCompletionV2`/`GenZshCompletion` verified |
| D-02 | `--host` completes from deploy.yaml host + ~/.ssh/config aliases | `config.LoadFile()` returns `FileConfig.Target.Host`; new `sshconfig.ListHosts()` needed for alias enumeration |
| D-03 | Silent failure on completion errors | `cobra.ShellCompDirectiveNoFileComp` with `return nil, ...` on any error path |
| D-04 | Visible `docker deploy completion <shell>` subcommand | `buildCompletionCmd()` pattern verified in codebase; registered via `cmd.AddCommand()` |
| D-05 | Discoverable via --help | Visible (not hidden) command; cobra shows it automatically in help |
| D-06 | `--path` suggests `/opt/<cwd-basename>` | `os.Getwd()` + `filepath.Base()` pattern — same logic as `runDeploy` |
| D-07 | `--compose-file` scans cwd | `os.ReadDir(".")` + filename check — mirrors existing `Resolve()` auto-detect |
</phase_requirements>

## Architectural Responsibility Map

| Capability | Primary Tier | Secondary Tier | Rationale |
|------------|-------------|----------------|-----------|
| Shell script generation | CLI layer (`cmd/`) | — | cobra's `GenBashCompletion`/`GenZshCompletion` writes to an `io.Writer` |
| Dynamic `--host` completions | CLI layer (completion func) | `internal/sshconfig/` + `internal/config/` | Completion func calls sshconfig.ListHosts and config.LoadFile at tab-press time |
| SSH alias enumeration | `internal/sshconfig/` | — | New `ListHosts()` function — parser already owns the file, add enumeration there |
| `--path` hint generation | CLI layer (completion func) | — | Pure `os.Getwd()` + string concat — no package boundary needed |
| `--compose-file` scan | CLI layer (completion func) | — | `os.ReadDir(".")` inline — simple enough to stay in completion func |

## Standard Stack

### Core
| Library | Version | Purpose | Why Standard |
|---------|---------|---------|--------------|
| `github.com/spf13/cobra` | v1.10.2 | Shell completion script generation + flag completion registration | Already in go.mod; provides `GenBashCompletion`, `GenBashCompletionV2`, `GenZshCompletion`, `RegisterFlagCompletionFunc` |

### Supporting
| Library | Version | Purpose | When to Use |
|---------|---------|---------|-------------|
| `internal/sshconfig` | (internal) | Enumerate SSH config host aliases | Needs new `ListHosts()` function — `LookupHost` only matches, does not enumerate |
| `internal/config` | (internal) | Read deploy.yaml for host value | Existing `LoadFile()` returns `FileConfig.Target.Host` |

### Alternatives Considered
| Instead of | Could Use | Tradeoff |
|------------|-----------|----------|
| `GenBashCompletion` (V1) | `GenBashCompletionV2(w, false)` | V2 is preferred for new code; V1 is legacy but both work at v1.10.2; V2 emits `# bash completion V2` header |
| `GenZshCompletion` | `GenZshCompletionNoDesc` | With-desc version is the default and the better UX; no-desc is for environments where descriptions clutter |

**Installation:** No new packages — cobra is already in go.mod at v1.10.2.

## Architecture Patterns

### System Architecture Diagram

```
User presses Tab
       │
       ▼
Shell calls: docker deploy --host <TAB>
       │
       ▼
cobra invokes RegisterFlagCompletionFunc for "--host"
       │
       ├── config.LoadFile(cwd) ──────► FileConfig.Target.Host (deploy.yaml host value)
       │                                        │ on error: []
       ├── sshconfig.ListHosts(~/.ssh/config) ► []string of alias names
       │                                        │ on error: []
       ▼
merge + deduplicate → return ([]string, ShellCompDirectiveNoFileComp)
       │
       ▼
Shell displays candidates to user
```

```
docker deploy completion bash > ~/.bash_completion.d/docker-deploy
       │
       ▼
buildCompletionCmd().RunE called with args[0]=="bash"
       │
       ├── "bash" → cmd.Root().GenBashCompletion(os.Stdout)   [or V2]
       └── "zsh"  → cmd.Root().GenZshCompletion(os.Stdout)
```

### Recommended Project Structure
```
cmd/docker-deploy/
├── main.go              # Add buildCompletionCmd(); add RegisterFlagCompletionFunc calls
internal/sshconfig/
├── sshconfig.go         # Add ListHosts(configPath string) []string
├── sshconfig_test.go    # Add tests for ListHosts
cmd/docker-deploy/
├── main_test.go         # Add: completion cmd registered, flag completions wired
```

### Pattern 1: buildCompletionCmd factory
**What:** Returns a visible cobra.Command for generating shell completion scripts.
**When to use:** For the `docker deploy completion <shell>` subcommand.

```go
// Source: cobra docs + existing buildVersionCmd pattern in this codebase
func buildCompletionCmd() *cobra.Command {
    return &cobra.Command{
        Use:       "completion [bash|zsh]",
        Short:     "Generate shell completion script",
        ValidArgs: []string{"bash", "zsh"},
        Args:      cobra.ExactValidArgs(1),
        SilenceUsage: true,
        RunE: func(cmd *cobra.Command, args []string) error {
            switch args[0] {
            case "bash":
                return cmd.Root().GenBashCompletion(os.Stdout)
            case "zsh":
                return cmd.Root().GenZshCompletion(os.Stdout)
            }
            return nil
        },
    }
}
```

### Pattern 2: RegisterFlagCompletionFunc for dynamic --host
**What:** Registers a Go function that cobra calls at tab-press time.
**When to use:** For all three dynamic flags: `--host`, `--path`, `--compose-file`.

```go
// Source: cobra v1.10.2 completions.go — Completion = string type alias
// RegisterFlagCompletionFunc MUST be called after the flag is defined.
// Returns error if flag not found — safe to ignore with _ in main.go.
_ = cmd.RegisterFlagCompletionFunc("host", func(_ *cobra.Command, _ []string, _ string) ([]cobra.Completion, cobra.ShellCompDirective) {
    var hosts []string
    // 1. Read deploy.yaml host (silent failure on error — D-03)
    cwd, err := os.Getwd()
    if err == nil {
        if fc, _, err := config.LoadFile(cwd); err == nil && fc.Target.Host != "" {
            hosts = append(hosts, fc.Target.Host)
        }
    }
    // 2. Read ~/.ssh/config aliases (silent failure on error — D-03)
    home, err := os.UserHomeDir()
    if err == nil {
        sshCfgPath := filepath.Join(home, ".ssh", "config")
        hosts = append(hosts, sshconfig.ListHosts(sshCfgPath)...)
    }
    // 3. Deduplicate
    hosts = dedupStrings(hosts)
    return hosts, cobra.ShellCompDirectiveNoFileComp
})
```

### Pattern 3: sshconfig.ListHosts — new function
**What:** Enumerates all non-wildcard Host block alias names from an SSH config file.
**When to use:** Completion time only — `LookupHost` cannot enumerate, it only matches.

```go
// Source: mirrors LookupHost scanner pattern in sshconfig.go
// Returns alias names (the label after "Host", not HostName).
// Skips wildcard patterns ("*", "*.domain.com") — not valid completion candidates.
// Silent on file-open errors (D-03 intent: return empty, not error).
func ListHosts(configPath string) []string {
    f, err := os.Open(configPath)
    if err != nil {
        return nil
    }
    defer f.Close()

    var aliases []string
    scanner := bufio.NewScanner(f)
    for scanner.Scan() {
        line := strings.TrimSpace(scanner.Text())
        if line == "" || strings.HasPrefix(line, "#") {
            continue
        }
        parts := strings.Fields(line)
        if len(parts) < 2 || strings.ToLower(parts[0]) != "host" {
            continue
        }
        for _, pattern := range parts[1:] {
            // Exclude wildcard patterns — not useful as completion values
            if !strings.ContainsAny(pattern, "*?") {
                aliases = append(aliases, pattern)
            }
        }
    }
    if scanner.Err() != nil {
        return nil
    }
    return aliases
}
```

### Pattern 4: --path completion
```go
// Source: mirrors cwd-basename logic in runDeploy (main.go)
_ = cmd.RegisterFlagCompletionFunc("path", func(_ *cobra.Command, _ []string, _ string) ([]cobra.Completion, cobra.ShellCompDirective) {
    cwd, err := os.Getwd()
    if err != nil {
        return nil, cobra.ShellCompDirectiveNoFileComp
    }
    suggestion := "/opt/" + filepath.Base(cwd)
    return []cobra.Completion{suggestion}, cobra.ShellCompDirectiveNoFileComp
})
```

### Pattern 5: --compose-file completion
```go
// Source: mirrors auto-detect logic in config.Resolve()
_ = cmd.RegisterFlagCompletionFunc("compose-file", func(_ *cobra.Command, _ []string, _ string) ([]cobra.Completion, cobra.ShellCompDirective) {
    var suggestions []cobra.Completion
    for _, name := range []string{"compose.yaml", "docker-compose.yml"} {
        if _, err := os.Stat(name); err == nil {
            suggestions = append(suggestions, name)
        }
    }
    return suggestions, cobra.ShellCompDirectiveNoFileComp
})
```

### Anti-Patterns to Avoid
- **Calling `LookupHost` to enumerate all hosts:** It requires a known alias to match against — it cannot list all aliases. Use the new `ListHosts` function.
- **Returning an error from a completion func:** Completion functions must never return an error to the caller. Use `return nil, cobra.ShellCompDirectiveNoFileComp` on all error paths (D-03).
- **Using `Run` instead of `RunE` in the completion subcommand:** `RunE` returns error to cobra's error handler, which is the established pattern in this codebase (`buildVersionCmd`, `buildValidateCmd` both use `RunE`). GenBashCompletion returns `error` — use `RunE` to propagate it.
- **Registering completions before flag definition:** `RegisterFlagCompletionFunc` returns an error if the flag does not exist. All registrations must follow `cmd.Flags().StringVar(...)` calls.
- **Relying on cobra's built-in completion command:** cobra's `InitDefaultCompletionCmd` auto-adds a `completion` command when `D-04` requires a manually-built visible one. The manual `buildCompletionCmd()` overrides the built-in when added with `cmd.AddCommand()`.

## Don't Hand-Roll

| Problem | Don't Build | Use Instead | Why |
|---------|-------------|-------------|-----|
| Shell completion script generation | Custom bash/zsh script template | `cobra.GenBashCompletion` / `cobra.GenZshCompletion` | Handles subcommand discovery, flag traversal, dynamic completions, zsh compdef boilerplate |
| Plugging completions into cobra | Manual `__complete` subcommand | `RegisterFlagCompletionFunc` | cobra's completion protocol automatically invokes registered functions |

**Key insight:** cobra at v1.10.2 handles 100% of shell integration plumbing. Implementation effort is registering three completion functions and one subcommand — no script authoring needed.

## Common Pitfalls

### Pitfall 1: sshconfig.ListHosts does not exist yet
**What goes wrong:** `LookupHost` is the only parser function. It cannot enumerate aliases — it matches a known alias to a Host block. Calling it without a known alias returns `found=false`.
**Why it happens:** Phase 14 only needed lookup-by-alias for SSH dialing. Enumeration was never required.
**How to avoid:** Add `ListHosts(configPath string) []string` to `internal/sshconfig/sshconfig.go` in Wave 0 (before the completion registration).
**Warning signs:** `--host` completions return empty even when `~/.ssh/config` has entries.

### Pitfall 2: cobra.Completion is `= string` but the type name matters
**What goes wrong:** Older cobra examples return `([]string, ShellCompDirective)`. In v1.10.2 the signature is `([]Completion, ShellCompDirective)` where `Completion = string`. Mixing `[]string` literals with `[]cobra.Completion` in the same expression compiles fine but tools like golangci-lint may flag it.
**How to avoid:** Use `[]cobra.Completion{value}` consistently, or cast: `cobra.Completion(value)`.

### Pitfall 3: Completion function called with cwd of the user's shell, not the plugin binary
**What goes wrong:** `os.Getwd()` inside a completion function returns the cwd where the user is running the shell. This is intentional — it is the project directory the user is in. But it means the function must not assume a fixed path.
**How to avoid:** Always call `os.Getwd()` at completion time (not at registration time). Already shown in Pattern 2.

### Pitfall 4: `cmd.Root().GenBashCompletion()` vs `cmd.Root().GenZshCompletion()`
**What goes wrong:** Using `cmd.GenBashCompletion()` (without `.Root()`) from inside the completion subcommand generates a script scoped only to the `completion` subcommand, not the full `deploy` command tree.
**How to avoid:** Always call `cmd.Root().GenBashCompletion(w)` — `cmd` inside `RunE` is the `completion` subcommand itself.

### Pitfall 5: cobra's default completion command conflict
**What goes wrong:** When cobra detects a root command has subcommands, it auto-injects a `completion` command via `InitDefaultCompletionCmd`. If you also add one via `cmd.AddCommand(buildCompletionCmd())`, cobra may panic or skip the auto-inject silently (depends on version).
**Why it happens:** cobra checks whether a completion command already exists before adding its own. At v1.10.2, manual addition via `AddCommand` takes priority — no conflict.
**How to avoid:** Add `buildCompletionCmd()` explicitly — the auto-generated one is suppressed. Verify with `TestCompletionCmd_Registered` test.

### Pitfall 6: Docker plugin.Run() wraps the root command — `cmd.Root()` is the deploy command
**What goes wrong:** In `plugin.Run(func(_ command.Cli) *cobra.Command { return buildDeployCmd() }, ...)`, the returned command IS the root cobra command for the plugin. `cmd.Root()` from within a subcommand returns `buildDeployCmd()`, which is correct for script generation.
**How to avoid:** No action needed — just confirm by checking that the generated script references `deploy` not a wrapper command name.

## Code Examples

Verified patterns from official sources:

### RegisterFlagCompletionFunc placement in buildDeployCmd()
```go
// Source: cobra v1.10.2 completions.go — must follow flag definitions
// Place after all cmd.Flags().StringVar() calls, before return cmd.

_ = cmd.RegisterFlagCompletionFunc("host", hostCompletionFunc)
_ = cmd.RegisterFlagCompletionFunc("path", pathCompletionFunc)
_ = cmd.RegisterFlagCompletionFunc("compose-file", composeFileCompletionFunc)

cmd.AddCommand(buildVersionCmd())
cmd.AddCommand(buildValidateCmd())
cmd.AddCommand(buildCompletionCmd())  // D-04: visible subcommand

return cmd
```

### GenBashCompletion vs GenBashCompletionV2
```go
// V1 (legacy, still supported at v1.10.2):
cmd.Root().GenBashCompletion(os.Stdout)

// V2 (preferred for new code):
cmd.Root().GenBashCompletionV2(os.Stdout, false) // false = no descriptions in bash
// or with descriptions:
cmd.Root().GenBashCompletionV2(os.Stdout, true)

// zsh (only one version, always includes descriptions):
cmd.Root().GenZshCompletion(os.Stdout)
```

## State of the Art

| Old Approach | Current Approach | When Changed | Impact |
|--------------|------------------|--------------|--------|
| `MarkFlagCustom()` (bash-only) | `RegisterFlagCompletionFunc()` (all shells) | cobra v1.2 | Cross-shell portability; the old approach is deprecated |
| `GenBashCompletion()` V1 | `GenBashCompletionV2()` | cobra v1.2+ | V2 supports descriptions; V1 is legacy but not removed |
| `[]string` return from completion func | `[]cobra.Completion` (= string alias) | cobra v1.5+ | Wire-compatible with `[]string`; type alias not a breaking change |

**Deprecated/outdated:**
- `MarkFlagCustom()`: bash-only, superseded by `RegisterFlagCompletionFunc`
- Handwritten bash completion scripts: entirely replaced by cobra generation

## Assumptions Log

| # | Claim | Section | Risk if Wrong |
|---|-------|---------|---------------|
| A1 | cobra's built-in auto-completion injection is suppressed when `buildCompletionCmd()` is manually added | Anti-Patterns | If not suppressed, two `completion` subcommands appear; test coverage catches this |
| A2 | `plugin.Run()` does not introduce additional wrapping layers that change `cmd.Root()` behavior | Pitfall 6 | GenBashCompletion would generate an unusable script; catch with smoke test |

## Open Questions (RESOLVED)

1. **GenBashCompletion V1 vs V2 — which to ship?**
   - What we know: V2 is preferred, supports descriptions, produces cleaner scripts; V1 is legacy.
   - What's unclear: Whether end-user bash versions in target environments support the V2 format (requires bash 4.1+; macOS ships bash 3.2).
   - RESOLVED: Ship `GenBashCompletionV2(w, false)` — V2 is the current standard. macOS users who need completion typically use `brew install bash` (bash 5.x).

2. **Should `--host` completion include the `ssh://` URL prefix?**
   - What we know: deploy.yaml hosts may be stored as bare aliases (`minipc`) or full URLs (`ssh://user@host`). SSH config aliases are bare names.
   - What's unclear: D-02 says "deploy.yaml host value" — if it is `ssh://user@host`, should that be a completion candidate?
   - RESOLVED: Return the raw value from deploy.yaml unchanged; return bare alias names from SSH config. The user can Tab-complete to either form.

## Environment Availability

Step 2.6: SKIPPED — no external dependencies. All work is pure Go code using already-imported packages. No CLI tools, databases, or services required.

## Validation Architecture

### Test Framework
| Property | Value |
|----------|-------|
| Framework | Go standard `testing` package |
| Config file | none — `go test ./...` discovers all tests |
| Quick run command | `go test ./cmd/docker-deploy/... ./internal/sshconfig/...` |
| Full suite command | `go test ./...` |

### Phase Requirements → Test Map
| Req ID | Behavior | Test Type | Automated Command | File Exists? |
|--------|----------|-----------|-------------------|-------------|
| D-01 | `completion bash` writes valid bash script to stdout | unit | `go test ./cmd/docker-deploy/... -run TestCompletionCmd` | ❌ Wave 0 |
| D-01 | `completion zsh` writes valid zsh script to stdout | unit | `go test ./cmd/docker-deploy/... -run TestCompletionCmd` | ❌ Wave 0 |
| D-01 | `completion fish` returns error (not supported) | unit | `go test ./cmd/docker-deploy/... -run TestCompletionCmd_InvalidShell` | ❌ Wave 0 |
| D-02 | `--host` completion merges deploy.yaml + ssh config | unit | `go test ./cmd/docker-deploy/... -run TestHostCompletion` | ❌ Wave 0 |
| D-03 | `--host` completion returns empty on missing files | unit | `go test ./cmd/docker-deploy/... -run TestHostCompletion_Silent` | ❌ Wave 0 |
| D-04 | `completion` subcommand is registered in buildDeployCmd | unit | `go test ./cmd/docker-deploy/... -run TestCompletionCmd_Registered` | ❌ Wave 0 |
| D-06 | `--path` completion returns `/opt/<cwd-basename>` | unit | `go test ./cmd/docker-deploy/... -run TestPathCompletion` | ❌ Wave 0 |
| D-07 | `--compose-file` returns compose.yaml if it exists in cwd | unit | `go test ./cmd/docker-deploy/... -run TestComposeFileCompletion` | ❌ Wave 0 |
| D-02 | `sshconfig.ListHosts` enumerates all non-wildcard aliases | unit | `go test ./internal/sshconfig/... -run TestListHosts` | ❌ Wave 0 |

### Sampling Rate
- **Per task commit:** `go test ./cmd/docker-deploy/... ./internal/sshconfig/...`
- **Per wave merge:** `go test ./...`
- **Phase gate:** `go test ./...` green before `/gsd-verify-work`

### Wave 0 Gaps
- [ ] `internal/sshconfig/sshconfig_test.go` — add `TestListHosts_*` tests (file exists but needs new cases)
- [ ] `cmd/docker-deploy/main_test.go` — add completion subcommand and flag completion tests (file exists but needs new cases)
- No new test files needed; both test files already exist

## Security Domain

### Applicable ASVS Categories

| ASVS Category | Applies | Standard Control |
|---------------|---------|-----------------|
| V2 Authentication | no | — |
| V3 Session Management | no | — |
| V4 Access Control | no | — |
| V5 Input Validation | yes | `cobra.ExactValidArgs(1)` on completion subcommand; `ValidArgs: []string{"bash", "zsh"}` |
| V6 Cryptography | no | — |

### Known Threat Patterns for completion/config-reading

| Pattern | STRIDE | Standard Mitigation |
|---------|--------|---------------------|
| Shell injection via completion candidates from deploy.yaml host | Tampering | Completion values are not executed; they are printed to stdout as candidates. No shell execution in completion path. |
| Path traversal via `--compose-file` completion | Tampering | Completion only checks `os.Stat` on `compose.yaml` / `docker-compose.yml` — no user input is passed to `os.Stat` directly |
| Malicious `~/.ssh/config` content causing panic | Spoofing | `ListHosts` uses `bufio.Scanner` with no dynamic execution; silently returns `nil` on scanner error (D-03) |

**Note:** Completion functions run entirely on the local machine as the current user. They read config files the user already owns. The attack surface is narrow.

## Sources

### Primary (HIGH confidence)
- [cobra v1.10.2 completions.go (raw)](https://raw.githubusercontent.com/spf13/cobra/v1.10.2/completions.go) — `Completion = string` type alias, `CompletionFunc` signature, `ShellCompDirective` constants
- [cobra pkg.go.dev v1.10.2](https://pkg.go.dev/github.com/spf13/cobra@v1.10.2) — `RegisterFlagCompletionFunc`, `GenBashCompletion`, `GenBashCompletionV2`, `GenZshCompletion` API signatures
- codebase: `internal/sshconfig/sshconfig.go` — `LookupHost` exists, `ListHosts` does NOT exist [VERIFIED: grep]
- codebase: `internal/config/config.go` — `LoadFile()` returns `FileConfig` with `Target.Host` [VERIFIED: read]
- codebase: `cmd/docker-deploy/main.go` — subcommand factory pattern, flag definitions [VERIFIED: read]
- codebase: `go.mod` — cobra v1.10.2 [VERIFIED: read]

### Secondary (MEDIUM confidence)
- [cobra completions docs (GitHub)](https://github.com/spf13/cobra/blob/main/site/content/completions/_index.md) — completion subcommand patterns, ValidArgs, ShellCompDirective usage
- [jmtirado.net shell completion article](https://jmtirado.net/shell-completion-with-cobra-and-go/) — completion subcommand switch pattern

## Metadata

**Confidence breakdown:**
- Standard stack: HIGH — cobra at the exact version in go.mod verified via raw source and pkg.go.dev
- Architecture: HIGH — all integration points verified by reading actual source files
- Pitfalls: HIGH — `ListHosts` gap found by grep; cobra type alias confirmed by raw source

**Research date:** 2026-06-01
**Valid until:** 2026-09-01 (cobra is stable; no imminent breaking changes expected)
