---
phase: 10
phase_name: "add-phase-autosuggestion"
project: "docker-deploy"
generated: "2026-06-05"
counts:
  decisions: 7
  lessons: 5
  patterns: 5
  surprises: 3
missing_artifacts:
  - "10-HUMAN-UAT.md — no human UAT file present for this phase"
---

# Phase 10 Learnings: Add Phase Autosuggestion (Shell Completions)

## Decisions

### Dynamic per-flag completion hooks replaced with static cobra-generated scripts
The original implementation (Plan 02) added `RegisterFlagCompletionFunc` hooks for `--host`, `--path`, and `--compose-file` that read `deploy.yaml` and `~/.ssh/config` at tab-press time. Plan 03 deleted this approach in favor of pre-generated static bash and zsh completion scripts.

**Rationale:** Dynamic completion added complexity (sshconfig.ListHosts, config.LoadFile in the hot path), was harder to test end-to-end, and the static approach covers all flag names and subcommand names without any runtime file reads. Static scripts can be committed and shipped in release tarballs.
**Source:** 10-03-SUMMARY.md

---

### Hidden: true + DisableFlagsInUseLine: true on the completion subcommand
The `completion [bash|zsh]` subcommand is marked `Hidden: true` so it does not appear in `docker deploy --help`.

**Rationale:** The subcommand is a pipeline tool for generating scripts (invoked by `make completions` and release tooling), not a user-facing command. Hiding it keeps the help output clean while keeping the subcommand fully functional. `DisableFlagsInUseLine: true` prevents `[flags]` from appearing in the Use line since the subcommand takes a positional arg, not flags.
**Source:** 10-03-SUMMARY.md

---

### Standalone cobra root for completion script generation
`buildStandaloneRootForCompletion()` creates a new `cobra.Command` named `docker-deploy` (cloning the deploy parent's flags) rather than using `cmd.Root()` which returns the Docker plugin root named `docker`.

**Rationale:** When the binary is invoked as a Docker CLI plugin, `cmd.Root()` is the docker plugin root, not the `docker-deploy` root. Completion scripts generated from the plugin root are named for `docker`, not `docker-deploy`. The standalone root ensures scripts carry the correct command name.
**Source:** 10-04-SUMMARY.md

---

### make completions target invokes `deploy completion zsh`, not bare `completion zsh`
The Makefile completions target uses `./bin/docker-deploy deploy completion zsh` (with the explicit `deploy` subcommand) rather than `./bin/docker-deploy completion zsh`.

**Rationale:** `plugin.Run()` intercepts bare `completion` as the first argument and routes it to Docker's own completion system, not to the hidden completion subcommand. Prefixing with `deploy` routes correctly through the plugin's command tree.
**Source:** 10-04-SUMMARY.md

---

### goreleaser archives[].files (not release.extra_files) for per-arch tarball inclusion
Completion files are added to each per-arch release tarball using `archives[].files` with `src:` entries, not `release.extra_files`.

**Rationale:** `release.extra_files` attaches files to the GitHub release page directly but does NOT include them in the per-arch tarballs that the Homebrew formula downloads and extracts. The `archives[].files` key adds files to each tarball where the Homebrew `install` block can reference them by basename.
**Source:** 10-05-SUMMARY.md

---

### Homebrew formula installs completion files from tarball to standard fpath paths
The `brews[0].install` Ruby block includes two additional lines: `(share/"zsh/site-functions").install "_docker-deploy"` and `(share/"bash-completion/completions").install "docker-deploy.bash"`.

**Rationale:** Homebrew's `share/zsh/site-functions/` directory is already on `$FPATH` on macOS with Homebrew. Installing `_docker-deploy` there enables zsh completions with zero user configuration after `brew install`.
**Source:** 10-05-SUMMARY.md

---

### INSTALL.md updated, README.md intentionally left untouched
Shell completion installation instructions go into INSTALL.md only. README.md is not modified.

**Rationale:** The completion feature is a convenience for power users, not a core value proposition. Advertising it in README.md would add noise for users who don't need or want completions. INSTALL.md is the appropriate depth for this content.
**Source:** 10-05-SUMMARY.md

---

## Lessons

### cobra v1.10.2 stores RegisterFlagCompletionFunc results in a global map, not flag annotations
Plan 02 specified asserting `flag.Annotations` is non-nil to verify that `Register()` had wired flag completions. In cobra v1.10.2, completion functions are stored in a package-level global map keyed by the flag — `flag.Annotations` stays nil.

**Context:** Discovered during the GREEN phase of TDD. Fixed by using `cmd.GetFlagCompletionFunc("flagname")` instead, which queries the actual storage mechanism cobra uses.
**Source:** 10-02-SUMMARY.md

---

### ListHosts must return nil (not empty slice) on all error paths
The `sshconfig.ListHosts` function returns `nil` rather than `[]string{}` when the file cannot be opened or the scanner encounters an error. This is consistent with the project's D-03 silent-fail contract.

**Context:** Returning `nil` vs empty slice matters for callers that check `if hosts != nil`. The `bufio.Scanner` skeleton from `LookupHost` was followed exactly, including the `return nil` on `scanner.Err() != nil`.
**Source:** 10-01-SUMMARY.md

---

### plugin.Run intercepts "completion" before the plugin's own command tree
When a Docker CLI plugin binary is invoked with `completion` as the first argument without a subcommand prefix, `plugin.Run()` (from `github.com/docker/cli`) intercepts it and routes to Docker's own completion system.

**Context:** This causes `./bin/docker-deploy completion bash` to fail or produce unexpected output (Docker's own completion, not the plugin's). The fix requires prefixing with the plugin's own subcommand name: `./bin/docker-deploy deploy completion bash`.
**Source:** 10-04-SUMMARY.md

---

### .gitignore pattern matching can interfere with tracking generated files in contrib/
The repository `.gitignore` has a pattern `docker-deploy` that could potentially match files in `contrib/docker-deploy.bash`. In practice, the files were already tracked by git so the commit worked without re-staging, but this is a subtle interaction to be aware of.

**Context:** Mentioned in 10-03-SUMMARY.md as an issue encountered. No lasting impact, but worth noting for future contributors adding files to `contrib/`.
**Source:** 10-03-SUMMARY.md

---

### TDD RED phase must confirm "undefined: X" failure, not just "test fails"
The TDD gate compliance requires running `go test` after the RED phase to confirm compilation failure (`undefined: ListHosts`, `undefined: Register`), not just a test assertion failure. A test that fails for the wrong reason (e.g., panics, wrong package, import error) does not constitute a valid RED gate.

**Context:** Both Plan 01 and Plan 02 committed the RED phase separately (5022181, a824a7a) before implementing. This ensures the GREEN commits (44ac600, 650c503) are correctly attributable to the implementation, not to test fixes.
**Source:** 10-01-SUMMARY.md, 10-02-SUMMARY.md

---

## Patterns

### sshconfig.ListHosts: enumerate SSH config aliases using bufio.Scanner skeleton
`ListHosts(configPath string) []string` follows the same `os.Open` → `bufio.Scanner` → `strings.Fields` skeleton as `LookupHost`. It appends non-wildcard Host pattern values (those without `*` or `?` via `strings.ContainsAny`) and returns nil on any error.

**When to use:** Any function that needs to enumerate rather than look up SSH config entries. The ListHosts/LookupHost pair covers the two primary SSH config use cases: enumerate all aliases, or look up a specific one.
**Source:** 10-01-SUMMARY.md

---

### Hidden cobra subcommands for release-pipeline maintenance commands
Use `Hidden: true` + `DisableFlagsInUseLine: true` on cobra subcommands that are intended for tooling pipelines (CI, Makefile targets) rather than direct user invocation.

**When to use:** Any subcommand used by `make` targets, release scripts, or CI workflows that should not appear in user-facing `--help` output. The command remains fully functional — it just does not clutter help text.
**Source:** 10-03-SUMMARY.md

---

### Makefile completions target with build dependency
The `completions: build` Makefile target ensures the binary is rebuilt before generating completion scripts. Both zsh and bash scripts are written to `contrib/` and committed to the repo.

**When to use:** Any CLI project that ships pre-generated shell completion scripts. Committing the generated files to `contrib/` rather than generating them at release time ensures Goreleaser can package them into tarballs without requiring a build step inside the release workflow.
**Source:** 10-04-SUMMARY.md

---

### POSIX install script with shell auto-detection for completions
`contrib/install-completions.sh` uses `basename "$SHELL"` to detect the running shell and downloads only the relevant completion file. Handles Homebrew Apple Silicon (`/opt/homebrew/...`), Homebrew Intel (`/usr/local/...`), and user-directory fallbacks (`~/.zsh/completions/`, `~/.bash_completion.d/`).

**When to use:** Non-Homebrew completion install paths for CLI tools that support both bash and zsh. Detecting `$SHELL` and handling both Homebrew prefix variants covers the majority of macOS and Linux developer workstations.
**Source:** 10-04-SUMMARY.md

---

### GenerateBash/GenerateZsh accept root command directly, not derive it via cmd.Root()
After the Plan 04 fix, `GenerateBash(root *cobra.Command, w io.Writer)` and `GenerateZsh(root *cobra.Command, w io.Writer)` accept the root command as a parameter rather than calling `cmd.Root()` internally.

**When to use:** Any CLI tool that is also a Docker plugin (or any plugin framework where `cmd.Root()` returns the host CLI's root rather than the plugin's own root). Passing the root explicitly avoids surprising behavior from `cmd.Root()` traversal.
**Source:** 10-04-SUMMARY.md

---

## Surprises

### plugin.Run's completion interception was discovered at make completions runtime, not during unit tests
The unit tests for `GenerateBash` and `GenerateZsh` built minimal cobra commands directly and never invoked the binary as a plugin. The interception of bare `completion` by `plugin.Run()` only manifested when `make completions` was first run against the built binary.

**Impact:** Required adding `buildStandaloneRootForCompletion()` and updating `GenerateBash`/`GenerateZsh` signatures, plus correcting the Makefile invocation. These changes touched 4 files in a single bug-fix commit (1cbf520) during Plan 04.
**Source:** 10-04-SUMMARY.md

---

### Dynamic completion approach was implemented and then fully removed in the same phase
Plans 01 and 02 implemented dynamic per-flag completions (HostCompletionFunc, PathCompletionFunc, ComposeFileCompletionFunc) with TDD. Plan 03 deleted all of it. The final phase deliverable contains none of the code written in Plans 01-02 except `sshconfig.ListHosts` (which remains useful) and `GenerateBash`/`GenerateZsh`.

**Impact:** Two plans' worth of TDD work (sshconfig.ListHosts, the completion package, test files) was partially discarded. The ListHosts function and the bash/zsh generator wrappers survived; everything involving RegisterFlagCompletionFunc was deleted. This was the correct outcome — the static approach is simpler and more reliable.
**Source:** 10-03-SUMMARY.md

---

### cobra GenBashCompletionV2 requires the root command named correctly for script headers
The first line of a cobra-generated bash script is `# bash completion V2 for <command-name>`, and the zsh script begins with `#compdef <command-name>`. When the root command is the Docker plugin root (named `docker`), these headers say `docker`, not `docker-deploy`.

**Impact:** Led to `buildStandaloneRootForCompletion()` which creates a cobra.Command named `docker-deploy` and copies the deploy flags onto it. This standalone root is passed to `GenerateBash`/`GenerateZsh` so the generated scripts have the correct command name and can be installed as `_docker-deploy`.
**Source:** 10-04-SUMMARY.md
