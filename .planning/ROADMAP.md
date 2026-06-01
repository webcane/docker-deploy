# Roadmap: docker-deploy

## Overview

The plugin is built bottom-up: scaffold the Docker CLI plugin first so the interface contract is locked, then layer in SSH transport and config resolution, then file copy, then the full deploy loop with pre-flight and health polling, and finally the isolated first-deploy wizard. Each phase delivers a vertically complete capability that can be verified end-to-end before the next phase begins.

## Phases

**Phase Numbering:**
- Integer phases (1, 2, 3): Planned milestone work
- Decimal phases (2.1, 2.2): Urgent insertions (marked with INSERTED)

Decimal phases appear between their surrounding integers in numeric order.

- [x] **Phase 1: Plugin Scaffolding** - `docker deploy --help` works in the Docker CLI; module locked; CI configured — completed 2026-05-13
- [x] **Phase 2: SSH Transport & Config** - SSH dial (knownhosts, timeout, auth chain) and config resolution work; operator can verify connectivity — completed 2026-05-14
- [x] **Phase 3: File Copy** - SFTP staging-dir upload with smart include/exclude filter; files land atomically on remote — completed 2026-05-14
- [x] **Phase 4: Core Deploy Loop** - `docker deploy --host ...` runs compose up on remote and streams output; exit codes correct — completed 2026-05-15
- [x] **Phase 5: Pre-flight & Health Polling** - All pre-flight checks run before deploy; health polling reports pass/fail after compose up — completed 2026-05-17
- [x] **Phase 7: v2 — Leftovers** - Expanded default excludes, `--skip-env` / `skip_env` setting, and `--verbose` flag — completed 2026-05-20
- [x] **Phase 8: Integration Tests** - Testcontainers-based suite verifies all requirements automatically against a real SSH daemon — completed 2026-05-22
- [x] **Phase 9: Documentation** - README.md tells the full story: why, how to install, use-cases, comparison table, prerequisites, troubleshooting, and project badges
- [x] **Phase 10: Add Phase Autosuggestion** - add phase autosuggestion — completed 2026-06-01
- [x] **Phase 11: CI & Tooling Polish** - Fix Codecov, bump GitHub Actions versions, add Brew auto-symlink on install and cleanup on uninstall (completed 2026-05-23)
- [x] **Phase 12: Docs Polish** - Fix help description, sharpen README value prop, restructure install docs to INSTALL.md, add comparison feedback link (completed 2026-05-24)
- [x] **Phase 13: CLI Subcommands & Deploy UX** - Resolve `deploy.yaml` relative to cwd; `version` and `validate` subcommands; consolidate sudo into one SSH session; verbose pre-confirm file diff; path-aware sudo detection (completed 2026-05-26)
- [x] **Phase 14: SSH Config Host Alias Resolution** - parse `~/.ssh/config` to resolve short host aliases without a full SSH URL — completed 2026-05-29
- [x] **Phase 15: Deploy Healthcheck Config Format** - define a config format for customising healthcheck polling behaviour per service (completed 2026-05-30)
- [x] **Phase 16: Release Tooling Enhancement** - extend `/gsd:release-tag` with STATE.md update and REQ-derived commit message body (completed 2026-05-27)

## Phase Details

### Phase 1: Plugin Scaffolding
**Goal**: The `docker deploy` plugin is installable and discoverable in the Docker CLI before any SSH or business logic exists
**Mode:** mvp
**Depends on**: Nothing (first phase)
**Requirements**: PLUG-01, PLUG-02, PLUG-03
**Success Criteria** (what must be TRUE):
  1. Running `docker deploy --help` from a shell with the binary installed shows command usage via the Docker CLI
  2. Running `docker-deploy docker-cli-plugin-metadata` returns valid JSON with plugin name and version
  3. GoReleaser builds a cross-platform binary and CI passes on every push
**Plans**: 2 plans

Plans:
- [x] 01-PLAN-01.md — Go module + plugin binary scaffold (plugin.Run(), cobra root, Makefile)
- [x] 01-PLAN-02.md — CI/CD pipeline (GoReleaser cross-platform builds, GitHub Actions workflows)

### Phase 2: SSH Transport & Config
**Goal**: The plugin can open a verified SSH connection to a remote host and resolve configuration from flags, deploy.yaml, and defaults in the correct precedence order
**Mode:** mvp
**Depends on**: Phase 1
**Requirements**: CFG-01, CFG-02, CFG-03, CFG-04, CFG-05
**Success Criteria** (what must be TRUE):
  1. `--host ssh://user@host:port` flag is accepted and parsed correctly
  2. SSH dial uses knownhosts verification; connecting to an unknown host prompts for confirmation rather than failing silently or accepting blindly
  3. SSH handshake timeout is enforced — a non-responsive host does not hang the process indefinitely
  4. `deploy.yaml` values are loaded when present; flag values override deploy.yaml; deploy.yaml overrides built-in defaults
  5. `deploy.yaml` schema accommodates future multi-target keys without breaking existing single-target configs
**Plans**: 3 plans

Plans:

**Wave 1** *(run in parallel)*
- [x] 02-01-PLAN.md — Dependencies + config resolution (go.mod, internal/config package, --host/--path/--dry-run flags)
- [x] 02-02-PLAN.md — SSH transport (internal/ssh package, knownhosts TOFU, hard-fail, auth chain, timeout)

**Wave 2** *(blocked on Wave 1 completion)*
- [x] 02-03-PLAN.md — Dry-run integration (wire Resolve + Dial in deploy RunE; human verification)

Cross-cutting constraints:
- InsecureIgnoreHostKey must not appear anywhere in the codebase (verified by grep gate in 02-02 and 02-03)
- SSH dial timeout uses goroutine + context.WithTimeout pattern (not ClientConfig.Timeout alone)

### Phase 3: File Copy
**Goal**: Local project files are transferred to the remote host atomically via SFTP using smart defaults and user-defined overrides
**Mode:** mvp
**Depends on**: Phase 2
**Requirements**: DEPLOY-02, DEPLOY-03, FILES-01, FILES-02, FILES-03
**Success Criteria** (what must be TRUE):
  1. Running the copy step uploads all non-excluded files (copy-everything-minus-excludes model) to the remote; .env is always copied
  2. .git/, node_modules/, vendor/, *.log, .DS_Store, and __pycache__/ are excluded from the upload by default and cannot be removed from the exclude list
  3. Files are staged to a `.deploy-tmp-<timestamp>` directory first, then moved atomically; a failed mid-copy never leaves the target directory in a partial state
  4. User can extend the exclude list via deploy.yaml `target.exclude:` or `--exclude` flag; both extend rather than replace the built-in defaults
  5. Repeat deploys prompt for confirmation (default No); --force or `force: true` in deploy.yaml skips the prompt
**Plans**: 5 plans

Plans:

**Wave 1** *(run in parallel)*
- [x] 03-01-PLAN.md — Config extension (Excludes/Force fields in Config and TargetConfig, updated Resolve() signature, tests)
- [x] 03-02-PLAN.md — filetransfer package (ShouldExclude/WalkFiles filter logic, SFTP Upload with atomic staging, unit tests)

**Wave 2** *(blocked on Wave 1 completion)*
- [x] 03-03-PLAN.md — Wire into main.go (--exclude/--force flags, replace-confirmation prompt, Upload() call, human verification)

**Wave 3** *(gap closure — blocked on Wave 2 completion)*
- [x] 03-04-PLAN.md — Gap closure: ShellQuote export, sudoPw reuse, four-step atomic swap with rollback
- [x] 03-05-PLAN.md — Gap closure: first-deploy mv nesting bug (rm -rf target placeholder before rename)

Cross-cutting constraints:
- .env must never appear in default excludes — it is the core value proposition of the tool
- Staging dir uses `/tmp/docker-deploy-<ts>` (always writable); target dir created separately with optional sudo fallback
- Each SSH exec command uses a separate client.NewSession() call (sessions are not reusable)

### Phase 4: Core Deploy Loop
**Goal**: A developer can deploy a local compose project to a remote VPS with a single command and see compose output streamed to their terminal
**Mode:** mvp
**Depends on**: Phase 3
**Requirements**: DEPLOY-01, DEPLOY-04, DEPLOY-05, DEPLOY-06
**Success Criteria** (what must be TRUE):
  1. `docker deploy --host ssh://user@host:port` completes a full copy-then-compose cycle without additional flags
  2. `docker compose up -d` output is streamed line-by-line to the local terminal as it executes on the remote
  3. The plugin exits with a non-zero code if file copy fails, if the remote compose command fails, or if SSH connectivity is lost mid-deploy
**Plans**: 3 plans

Plans:

**Wave 1** *(run in parallel)*
- [x] 04-01-PLAN.md — Config extension (ComposeFile field, compose_file yaml key, updated Resolve() signature, TDD)
- [x] 04-02-PLAN.md — compose package (RunCompose() with PTY/pipe output routing, exit code propagation, TDD)

**Wave 2** *(blocked on Wave 1 completion)*
- [x] 04-03-PLAN.md — Wire into main.go (--compose-file flag, Resolve() call update, basename validation, RunCompose() call, human verification)

Cross-cutting constraints:
- InsecureIgnoreHostKey must not appear anywhere in the codebase (grep gate in 04-02 and 04-03)
- RunCompose() uses a dedicated NewSession() per CLAUDE.md — sessions are not reusable
- compose_file argument is a basename only — no path separators allowed (validated in 04-03)

Cross-cutting constraints:
- InsecureIgnoreHostKey must not appear anywhere in the codebase
- Each SSH exec (compose) uses a dedicated client.NewSession() — sessions are NOT reusable
- Remote command uses explicit `-f <path>/<file>` — no `cd &&` pattern
- --remove-orphans always included in compose command

### Phase 5: Pre-flight & Health Polling
**Goal**: The plugin validates remote readiness before deploying and reports container health after compose up completes
**Mode:** mvp
**Depends on**: Phase 4
**Requirements**: CHECK-01, CHECK-02, CHECK-03, CHECK-04, CHECK-05, CHECK-06, CHECK-07, HEALTH-01, HEALTH-02, HEALTH-03
**Success Criteria** (what must be TRUE):
  1. Deploying to a remote without Docker installed produces a clear error before any files are copied
  2. Deploying to a remote with only docker-compose v1 (EOL) produces a clear error distinguishing it from the required docker compose v2 plugin
  3. Deploying as root produces a visible warning but does not block the deploy
  4. If the target directory is not writable, the plugin attempts to create and chown it (with sudo) before failing
  5. After compose up, the plugin polls container health status and reports healthy / unhealthy / unknown (no HEALTHCHECK defined) for each service
  6. The plugin exits with a non-zero code if any container reaches the unhealthy state
**Plans**: 4 plans

Plans:

**Wave 1**
- [x] 05-01-PLAN.md — Config extension (HealthTimeout/HealthInterval fields in Config and TargetConfig, updated Resolve() signature, TDD)

**Wave 2** *(run in parallel — both blocked on Wave 1)*
- [x] 05-02-PLAN.md — preflight package (RunPreflightChecks with CHECK-01 through CHECK-07, TDD)
- [x] 05-03-PLAN.md — health package (PollHealth with HEALTH-01 through HEALTH-03, TDD)

**Wave 3** *(blocked on Wave 2 completion)*
- [x] 05-04-PLAN.md — Wire into main.go (RunPreflightChecks after Dial, PollHealth after RunCompose, human verification)

Cross-cutting constraints:
- InsecureIgnoreHostKey must not appear anywhere in the codebase
- Each SSH exec (per check, per poll tick) uses a dedicated client.NewSession() — sessions are NOT reusable
- ShellQuote() applied to all path and username args in SSH exec commands
- Pre-flight is silent on pass (D-01); errors and warnings to os.Stderr (D-02)
- Verbose checklist output (--verbose) deferred to Phase 7 (D-03)
- CHECK-03 (daemon not running) and CHECK-07 (root user) are warnings only — never block (D-05, D-06)
- CHECK-04 and CHECK-06 assume passwordless sudo; print actionable fix command on failure (D-07, D-08)

### Phase 7: v2 — Leftovers
**Goal**: Ship a wave of small v2 quality-of-life improvements: expand the built-in exclude list to cover common dev-tooling directories, add a `--skip-env` flag so operators can preserve remote secrets across deploys, and add a `--verbose` flag for detailed deploy output
**Depends on**: Phase 6
**Plans**: 2 plans

**Success Criteria** (what must be TRUE):

**Wave 1 — Expanded default excludes + skip-env**
  1. Passing `--skip-env` on the command line causes the `.env` file to be excluded from the SFTP upload, leaving the remote copy untouched
  2. Setting `skip_env: true` in `deploy.yaml` has the same effect as `--skip-env`; the CLI flag takes precedence when both are set
  3. The exclude logic is additive — `--skip-env` appends `.env` to the effective exclude list without replacing any other configured excludes
  4. When `.env` is skipped, a visible warning is printed so the operator knows the remote `.env` was not updated
  5. `.claude/`, `.github/`, `.planning/`, `.idea/`, `.vscode/`, `*.swp`, `*.swo`, `coverage/`, `dist/`, and `.terraform/` are added to the built-in default exclude list and silently skipped unless the user explicitly re-includes them

**Wave 2 — Verbose flag**
  6. `--verbose` prints each file being transferred, each SSH command executed, and its exit code; without the flag output remains as terse as today

Plans:

**Wave 1** *(run independently)*
- [x] 07-01-PLAN.md — Config foundation (FlagOpts struct, Resolve() refactor, expanded defaultExcludes, SkipEnv/Verbose fields, config tests)

**Wave 2** *(blocked on Wave 1 completion)*
- [x] 07-02-PLAN.md — Wire-up (--skip-env/--verbose flags, FlagOpts call sites, Upload() verbose, warning rollup, preflight checklist, human verification)

### Phase 8: Integration Tests
**Goal**: A testcontainers-based test suite automatically verifies all v1 requirements against a real SSH daemon — SSH connectivity, root-user warning, sshuser sudo permissions, preflight checks, file copy atomicity, compose execution, and health polling — so regressions are caught without manual VPS access
**Depends on**: Phase 7
**Plans**: 6 plans

**Success Criteria** (what must be TRUE):
  1. `go test -tags integration -timeout 5m ./integration/...` spins up a real SSH daemon container and runs end-to-end against it without any manual setup
  2. SSH connectivity verification (knownhosts, timeout, auth chain) is covered by at least one test
  3. Root-user warning (CHECK-07) is triggered and asserted when connecting as root
  4. Passwordless-sudo permission check (CHECK-04, CHECK-06) passes for a correctly configured sshuser and fails with a clear error for a misconfigured user
  5. File copy atomicity is verified — a simulated mid-copy failure leaves the target directory in its pre-deploy state
  6. All preflight checks (CHECK-01 through CHECK-07) have at least one passing and one failing scenario covered
  7. Health polling (HEALTH-01 through HEALTH-03) is exercised against a container with and without a HEALTHCHECK defined

Plans:

**Wave 1** *(run in parallel)*
- [x] 08-01-PLAN.md — Container infrastructure (Dockerfile.sshd DinD+SSH with 4 users, helpers_test.go with TestMain + shared helpers)
- [x] 08-02-PLAN.md — CI/Makefile (make test-integration target, GitHub Actions integration job after unit-test)

**Wave 2** *(blocked on Wave 1 completion — run in parallel)*
- [x] 08-03-PLAN.md — SSH connectivity tests (integration/dial_test.go: TestDial_Timeout, TOFU rejection, TOFU acceptance, Success using sshA)
- [x] 08-04-PLAN.md — Preflight check tests (integration/preflight_test.go: CHECK-01 through CHECK-07 pass+fail, CHECK-07 root warning SC-3, sshuser/nosudouser SC-4)
- [x] 08-05-PLAN.md — File copy atomicity tests (integration/filetransfer_test.go: TestUpload_HappyPath, AtomicCancel SC-5, SkipEnv)

**Wave 3** *(blocked on Wave 2 completion)*
- [x] 08-06-PLAN.md — Compose + health E2E tests (integration/compose_test.go: healthy nginx SC-7, unhealthy HEALTH-03, no-containers; testdata compose files)

Cross-cutting constraints:
- //go:build integration on line 1 of every file in integration/ (per D-06)
- package integration_test (external test package) across all files (per D-06)
- TestMain starts ALL containers once; tests must not leave dirty state (per D-07)
- Internal package APIs called directly — no exec.Command subprocess invocation (per D-08)
- No InsecureIgnoreHostKey anywhere including test helpers (CLAUDE.md Rule 1)
- One NewSession() per SSH command in test helpers (CLAUDE.md Rule 3)

### Phase 9: Distribution & Documentation
**Goal**: docker-deploy is installable via three progressively convenient methods (manual binary, install script, Homebrew tap) and README.md is the single authoritative resource for new users — explaining why the tool exists, how to install it, how to use it across all scenarios, and how to get help when things go wrong
**Depends on**: Phase 8
**Plans**: 2 plans

**Success Criteria** (what must be TRUE):

**Distribution:**
  1. GoReleaser produces signed binaries for darwin/linux × amd64/arm64 and attaches them to GitHub Releases automatically on tag push
  2. `install.sh` detects OS and architecture, downloads the correct binary from GitHub Releases, places it in `~/.docker/cli-plugins/docker-deploy`, and sets the executable bit — a single `curl | sh` invocation completes a working install
  3. A Homebrew tap (`homebrew-docker-deploy`) hosts a formula that installs the correct binary from GitHub Releases; `brew install <tap>/docker-deploy` produces a working plugin
  4. All three install methods are verified to produce a working `docker deploy --help` after install

**Documentation:**
  5. README.md explains the core value proposition and motivation — why docker-deploy exists as a simpler alternative to complex CI/CD pipelines for developers who just want `scp + compose up`
  6. README.md install section covers all three methods (manual binary, install script, Homebrew) with copy-paste commands for each
  7. README.md includes a use-case section covering three scenarios: sshuser vs root, flags-only usage, and deploy.yaml config-driven usage with working examples
  8. A comparison table of concurrent tools (Deployer, Kamal, Ansible, manual SSH scripts) is included with objective tradeoffs on complexity, dependencies, and target audience
  9. Prerequisite guide(s) cover SSH key setup and adding passwordless sudo to sshuser on the remote
  10. A troubleshooting section covers the most common failure scenarios (SSH auth failure, unknown host, writable dir, Docker not found, compose v1) with actionable fixes
  11. A feedback and suggestions section links to GitHub Issues with a welcome message for bug reports and feature requests
  12. README.md includes badges for build status, latest release version, and test status

Plans:
- [ ] 09-01-PLAN.md — GoReleaser darwin builds + cosign signing + brews block + release.yml update (autonomous: false — tap repo + secret setup required)
- [ ] 09-02-PLAN.md — install.sh: OS/arch detection, SHA256 verification, cosign fallback
- [ ] 09-03-PLAN.md — README.md: value prop, all 4 install methods, 3 use-case scenarios, badges, TON badge
- [ ] 09-04-PLAN.md — Supporting docs: PREREQUISITES.md, COMPARISON.md, TROUBLESHOOTING.md, DEPLOY_CONFIG.md

### Phase 10: Add Phase Autosuggestion
**Goal**: Add shell tab completion to the `docker deploy` CLI plugin — `completion bash` and `completion zsh` subcommand, plus dynamic `--host`, `--path`, and `--compose-file` suggestions sourced from deploy.yaml and ~/.ssh/config
**Depends on**: Phase 8
**Plans**: 2 plans

**Success Criteria** (what must be TRUE):
  1. `docker deploy completion bash` writes a bash completion script to stdout; piping it to a file and sourcing it makes Tab work in bash
  2. `docker deploy completion zsh` writes a zsh completion script to stdout; piping it to a file enables Tab in zsh
  3. Pressing Tab on `--host` shows candidates merged from deploy.yaml `target.host` and all non-wildcard Host block aliases in `~/.ssh/config`
  4. Pressing Tab on `--path` suggests `/opt/<cwd-basename>`
  5. Pressing Tab on `--compose-file` suggests `compose.yaml` and/or `docker-compose.yml` if they exist in cwd
  6. All completion functions return empty candidates (not an error) when their data sources are missing or unreadable

Plans:

**Wave 1**
- [x] 10-01-PLAN.md — sshconfig.ListHosts TDD (add ListHosts enumeration function + tests)

**Wave 2** *(blocked on Wave 1 completion)*
- [x] 10-02-PLAN.md — buildCompletionCmd + RegisterFlagCompletionFunc wiring (completion subcommand, --host/--path/--compose-file completions, tests)

### Phase 11: CI & Tooling Polish
**Goal**: Restore Codecov coverage reporting, bump all GitHub Actions to current stable versions, add golangci-lint enforcement, and make the Homebrew formula handle plugin symlink lifecycle automatically on install and uninstall
**Depends on**: Phase 9
**Requirements**: TBD
**Plans**: 2 plans

**Success Criteria** (what must be TRUE):
  1. Coverage reports are uploaded to Codecov and a badge displays current coverage in README
  2. All actions in `.github/workflows/` reference current major versions with no deprecation warnings in CI
  3. `brew install <tap>/docker-deploy` results in a working `docker deploy --help` with no additional user steps
  4. `brew uninstall docker-deploy` leaves no dangling symlink or binary in `~/.docker/cli-plugins/`
  5. `make lint` runs errcheck, govet, staticcheck, and goimports; a CI lint job must pass before merge

Plans:

**Wave 1** *(run in parallel)*
- [x] 11-01-PLAN.md — Codecov config + badge (fix README badge branch, add coverage upload step)
- [x] 11-02-PLAN.md — GitHub Actions hygiene (remove FORCE_JAVASCRIPT_ACTIONS_TO_NODE24, add Dependabot)
- [x] 11-04-PLAN.md — Linting & formatting (.golangci.yml, make lint, make fmt, CI lint job)

**Wave 2** *(independent of Wave 1 — only touches .goreleaser.yaml)*
- [x] 11-03-PLAN.md — Brew formula: post_install symlink + def uninstall cleanup + remove caveats

### Phase 12: Docs Polish
**Goal**: Tighten all user-facing documentation in one pass — fix the plugin help text, sharpen the README value proposition, move install instructions to a dedicated INSTALL.md, and add a feedback link to COMPARISON.md
**Depends on**: Phase 9
**Requirements**: TBD
**Plans**: 2 plans

**Success Criteria** (what must be TRUE):
  1. `docker deploy --help` shows an accurate, well-formed description of the plugin
  2. README clearly explains who the tool is for and why it is simpler than CI/CD pipelines in under 100 words
  3. INSTALL.md exists with a section for each install method (Script, Homebrew, Manual, `go install`); README links to it
  4. COMPARISON.md contains a visible link to GitHub Issues for users to suggest additions

Plans:

**Wave 1** *(run in parallel — all independent markdown edits)*
- [ ] 12-01-PLAN.md — Fix help description (cobra root command Short/Long strings)
- [ ] 12-02-PLAN.md — README: sharpen value prop + add INSTALL.md link
- [ ] 12-03-PLAN.md — Create INSTALL.md; strip install section from README
- [ ] 12-04-PLAN.md — COMPARISON.md: add feedback/contribution link

### Phase 13: CLI Subcommands & Deploy UX
**Goal**: Fix six self-contained Go issues: resolve `deploy.yaml` relative to cwd, add `docker deploy version` and `docker deploy validate` subcommands, consolidate remote sudo calls into a single SSH session, add a verbose pre-confirm file diff, and add path-aware sudo detection to skip guaranteed-to-fail direct copy attempts on elevated paths
**Depends on**: Phase 9
**Requirements**: TBD
**Plans**: 7 plans

**Success Criteria** (what must be TRUE):
  1. `deploy.yaml` is resolved relative to cwd; no hardcoded absolute paths in config resolution logic
  2. `docker deploy version` is a standalone subcommand that prints a single version string and exits 0
  3. When built from a tagged commit, the version string is the semver tag (e.g. `v0.6.3`); untagged builds print the short git commit hash
  4. Version values are injected at build time via Go `-ldflags`; no runtime git invocation
  5. `docker deploy validate` exits 0 and prints `✓ deploy.yaml is valid` on a good config; exits non-zero and lists field errors on a bad config — no SSH connection made
  6. A deploy to a sudo-required path (e.g. `/opt/<project>`) issues exactly one sudo prompt regardless of file count; user-writable paths are unaffected
  7. In `--verbose` mode, remote files and local files are listed before the "Replace all contents?" prompt so the operator can see what will change
  8. `sudoRunWithFallback` skips the direct-copy attempt when `test -w <path>` already indicates elevation is required; user-writable paths are unaffected

Plans:

**Wave 1** *(run in parallel — 13-01 through 13-05 and 13-07 are independent)*
- [x] 13-01-PLAN.md — Resolve deploy.yaml relative to cwd (config.LoadFile audit + unit test)
- [x] 13-02-PLAN.md — `version` subcommand + ldflags wiring in GoReleaser/Makefile
- [x] 13-03-PLAN.md — `validate` subcommand (buildValidateCmd + runValidate, no SSH)
- [x] 13-04-PLAN.md — SudoExec refactor (exported SudoExec, SudoCreds type, sshRun merge, Upload signature update)
- [x] 13-05-PLAN.md — Verbose pre-confirm file diff (move confirm prompt into Upload(), SFTP ReadDir + WalkFiles before prompt)
- [x] 13-07-PLAN.md — Verbose `sudo -l` output in CHECK-04 preflight (best-effort, stderr, [sudo -l] prefix)

**Wave 2** *(blocked on 13-04 completion — calls exported SudoExec)*
- [x] 13-06-PLAN.md — Path-aware sudo detection (`test -w` probe → needsSudo flag, bypass SudoExec on writable paths)

### Phase 14: SSH Config Host Alias Resolution
**Goal**: Parse `~/.ssh/config` so that short host aliases (e.g. `minipc`) resolve to the real `HostName`, `User`, and `Port` without requiring a full SSH URL; also improve deploy.yaml error messages so users can tell whether their config file is being read at all
**Depends on**: Phase 9
**Requirements**: TBD
**Plans**: 2 plans

**Success Criteria** (what must be TRUE):

**Wave 1 — SSH config alias resolution**
  1. `--host minipc` resolves via `~/.ssh/config` and connects successfully when a matching `Host` block exists
  2. `HostName`, `User`, and `Port` directives are all honoured; missing directives fall back to defaults
  3. An unmatched alias produces a clear error distinguishing "alias not found in ssh config" from a dial failure

**Wave 2 — Better deploy.yaml error messages**
  4. When no `deploy.yaml` exists and no `--host` flag is given, the error message says the file was not found
  5. When `deploy.yaml` exists but `target.host` is empty or unparseable, the error message names the file and the missing/invalid field
  6. Both error paths are covered by unit tests in the config package; no SSH connection is attempted in either case

**Wave 3 — macOS Keychain / system keyring sudo credential caching**
  7. On the first deploy requiring sudo, the user is prompted for the password and offered "Save to system keyring? [y/N]"
  8. Stored credentials are keyed per `docker-deploy:<host>:<user>` and retrieved silently on subsequent deploys
  9. `--clear-credentials` removes stored keyring entries for the current target
  10. Falls back gracefully to interactive prompt when the keyring is unavailable; credential is never logged or exposed
  11. Only activates on paths that require sudo; user-writable paths are unaffected

Plans:
- [x] 14-01-PLAN.md — SSH config alias resolution (`~/.ssh/config` parser, HostName/User/Port lookup, fallback to defaults)
- [x] 14-02-PLAN.md — Distinguish file-not-found vs host-not-set in config.Resolve() / runDeploy error path
- [ ] 14-03-PLAN.md — Keyring integration (`go-keyring` or `99designs/keyring`, prompt-and-save flow, `--clear-credentials`, graceful fallback)

### Phase 15: Deploy Healthcheck Config Format
**Goal**: Define a `deploy.yaml` config format for customising health-polling behaviour (timeout, interval) per service so operators aren't locked into global defaults
**Depends on**: Phase 9
**Requirements**: TBD
**Plans**: 3 plans (2 + 1 gap closure)

**Success Criteria** (what must be TRUE):
  1. Users can set `target.healthcheck.{interval,timeout,retries}` in `deploy.yaml` using Docker-style duration strings and the values are respected during polling
  2. Omitting the `healthcheck` block skips health polling entirely (no hardcoded defaults in code; defaults live in the global config file per D-04)
  3. CLI flags `--healthcheck-timeout`, `--healthcheck-interval`, `--healthcheck-retries` override `deploy.yaml`
  4. Retries semantics: per-container consecutive-unhealthy counter; resets on healthy; trips fail when `>= retries`
  5. YAML typos in healthcheck keys return a parse error (KnownFields strict mode)
  6. `--dry-run` output includes resolved healthcheck config unconditionally

Plans:

**Wave 1**
- [x] 15-01-PLAN.md — Config schema + resolution + flag registration (HealthcheckConfig struct, duration parsing, --healthcheck-{timeout,interval,retries} flags, config tests)

**Wave 2** *(blocked on Wave 1 completion)*
- [x] 15-02-PLAN.md — Health polling retries semantics (consume cfg.Healthcheck.{Interval,Timeout,Retries}; per-container failCount; reset on healthy; poll tests)

**Gap Closure**
- [x] 15-03-PLAN.md — Strict YAML parsing (KnownFields) + dry-run healthcheck row (completed 2026-05-31)

### Phase 16: Release Tooling Enhancement
**Goal**: Extend `/gsd:release-tag` so a release is one command: run local checks (tests, linter) to catch failures before they hit CI, update STATE.md with the new version and date, generate a meaningful commit message body from `.planning/research/SUMMARY.md` requirements, then tag and push; also add a terminal demo recording to README so visitors immediately see the tool in action
**Depends on**: Phase 9
**Requirements**: TBD
**Plans**: 2 plans

**Success Criteria** (what must be TRUE):

**Wave 0 — Pre-release checks**
  1. `go test ./...` passes with no failures before any release commit is made
  2. `go test -tags integration ./...` (test-ci) passes before tag+push
  3. Linter (`golangci-lint run`) runs before tag+push; if it reports issues, `make lint-fix` is run automatically to apply auto-fixable corrections
  4. After `make lint-fix`, lint runs a second time; only non-auto-fixable issues abort the release with a clear error message — no partial release

**Wave 1 — Release tag tooling**
  5. `STATE.md` is updated with the new version and release date as part of every `/gsd:release-tag` run
  6. The release commit message body is derived from the REQ summary, not a generic "bump version" line

**Wave 2 — Terminal demo for README**
  7. A terminal session demo (e.g. via `vhs` or `asciinema`) showing a full `docker deploy` run is recorded and embedded in README.md
  8. The demo covers at minimum: config resolution, file copy, and compose up output
  9. The recording is reproducible — a script or `vhs` tape file is committed to the repo

**Wave 3 — Extended linter coverage**
  10. golangci-lint runs gosec, ineffassign, unused, bodyclose, and noctx (security and resource-leak linters)
  11. gocritic, revive, errorlint, and wrapcheck (code quality linters) run and pass
  12. gocognit (min-complexity: 15) and nestif (min-complexity: 5) enforce complexity bounds
  13. prealloc flags slice pre-allocation opportunities in loops
  14. errcheck excludes `fmt.Fprintf`, `fmt.Fprintln`, `fmt.Fprint`, and SSH/SFTP `.Close()` calls from reporting
  15. wrapcheck ignores `.Errorf(`, `errors.New(`, and `errors.Unwrap(` — no wrapping noise on stdlib error constructors
  16. All new linters pass with zero issues on the existing codebase (or each finding is fixed before the plan closes)

Plans:

**Wave 1** *(both plans run in parallel — independent files)*
- [x] 16-00-PLAN.md — Pre-release checks + STATE.md update + commit body: extend `release-tag.md` with Wave 0 checks (go test, lint+fix-retry, test-ci Docker auto-detect) and Wave 1 changes (STATE.md last_updated/last_activity, git log-derived commit body)
- [x] 16-03-PLAN.md — Extend golangci-lint config: add gosec, ineffassign, unused, bodyclose, noctx, gocritic, revive, errorlint, wrapcheck, gocognit, nestif, prealloc; tune errcheck excludes and wrapcheck ignore-sigs; fix any new findings

**Wave 2 — SKIPPED** *(terminal demo deferred per D-14)*

## Progress

**Execution Order:**
Phases execute in numeric order: 1 → 2 → 3 → 4 → 5 → 6

| Phase | Plans Complete | Status | Completed |
|-------|----------------|--------|-----------|
| 1. Plugin Scaffolding | 2/2 | Complete | 2026-05-13 |
| 2. SSH Transport & Config | 3/3 | Complete | 2026-05-14 |
| 3. File Copy | 5/5 | Complete | 2026-05-15 |
| 4. Core Deploy Loop | 4/4 | Complete | 2026-05-18 |
| 5. Pre-flight & Health Polling | 4/4 | Complete | 2026-05-17 |
| 7. v2 — Leftovers | 2/2 | Complete | 2026-05-20 |
| 8. Integration Tests | 6/6 | Complete | 2026-05-22 |
| 9. Distribution & Documentation | 4/4 | Complete | 2026-05-23 |
| 10. Add Phase Autosuggestion | 2/2 | Complete   | 2026-06-01 |
| 11. CI & Tooling Polish | 4/4 | Complete   | 2026-05-23 |
| 12. Docs Polish | 0/4 | Not started | - |
| 13. CLI Subcommands & Deploy UX | 7/7 | Complete | 2026-05-26 |
| 14. SSH Config Host Alias Resolution | 0/? | Not started | - |
| 15. Deploy Healthcheck Config Format | 2/2 | Complete   | 2026-05-30 |
| 16. Release Tooling Enhancement | 2/2 | Complete   | 2026-05-27 |

## Backlog

### Phase 999.1: Init Wizard (BACKLOG)

**Goal:** A developer can run `--init` to set up a fresh VPS deploy target via root SSH and have deploy.yaml written automatically
**Requirements:** INIT-01, INIT-02, INIT-03, INIT-04
**Plans:** 1/2 plans executed

**Success Criteria** (what must be TRUE):
  1. `docker deploy --init` triggers an interactive wizard that accepts root SSH credentials separate from the deploy user credentials
  2. On first deploy, if the target directory does not exist or is not writable, the wizard offers to run the init flow automatically
  3. After a successful init, `/opt/<project>` exists and is owned by the deploy user on the remote
  4. A `deploy.yaml` containing host, user, and path is written to the project root after a successful wizard run

Plans:
- [ ] TBD (promote with /gsd-review-backlog when ready)

