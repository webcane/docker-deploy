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
- [ ] **Phase 6: Init Wizard** - `--init` creates target directory and writes deploy.yaml via root SSH
- [ ] **Phase 7: v2 — Leftovers** - Expanded default excludes, `--skip-env` / `skip_env` setting, and `--verbose` flag
- [ ] **Phase 8: Integration Tests** - Testcontainers-based suite verifies all requirements automatically against a real SSH daemon
- [ ] **Phase 9: Documentation** - README.md tells the full story: why, how to install, use-cases, comparison table, prerequisites, troubleshooting, and project badges

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

### Phase 6: Init Wizard
**Goal**: A developer can run `--init` to set up a fresh VPS deploy target via root SSH and have deploy.yaml written automatically
**Mode:** mvp
**Depends on**: Phase 5
**Requirements**: INIT-01, INIT-02, INIT-03, INIT-04
**Success Criteria** (what must be TRUE):
  1. `docker deploy --init` triggers an interactive wizard that accepts root SSH credentials separate from the deploy user credentials
  2. On first deploy, if the target directory does not exist or is not writable, the wizard offers to run the init flow automatically
  3. After a successful init, `/opt/<project>` exists and is owned by the deploy user on the remote
  4. A `deploy.yaml` containing host, user, and path is written to the project root after a successful wizard run
**Plans**: TBD

### Phase 7: v2 — Leftovers
**Goal**: Ship a wave of small v2 quality-of-life improvements: expand the built-in exclude list to cover common dev-tooling directories, add a `--skip-env` flag so operators can preserve remote secrets across deploys, and add a `--verbose` flag for detailed deploy output
**Depends on**: Phase 6
**Plans**: TBD

**Success Criteria** (what must be TRUE):

**Wave 1 — Expanded default excludes + skip-env**
  1. Passing `--skip-env` on the command line causes the `.env` file to be excluded from the SFTP upload, leaving the remote copy untouched
  2. Setting `skip_env: true` in `deploy.yaml` has the same effect as `--skip-env`; the CLI flag takes precedence when both are set
  3. The exclude logic is additive — `--skip-env` appends `.env` to the effective exclude list without replacing any other configured excludes
  4. When `.env` is skipped, a visible warning is printed so the operator knows the remote `.env` was not updated
  5. `.claude/`, `.github/`, `.planning/`, `.idea/`, `.vscode/`, `*.swp`, `*.swo`, `coverage/`, `dist/`, and `.terraform/` are added to the built-in default exclude list and silently skipped unless the user explicitly re-includes them

**Wave 2 — Verbose flag**
  6. `--verbose` prints each file being transferred, each SSH command executed, and its exit code; without the flag output remains as terse as today

### Phase 8: Integration Tests
**Goal**: A testcontainers-based test suite automatically verifies all v1 requirements against a real SSH daemon — SSH connectivity, root-user warning, sshuser sudo permissions, preflight checks, file copy atomicity, compose execution, and health polling — so regressions are caught without manual VPS access
**Depends on**: Phase 7
**Plans**: TBD

**Success Criteria** (what must be TRUE):
  1. `go test ./integration/...` spins up a real SSH daemon container and runs end-to-end against it without any manual setup
  2. SSH connectivity verification (knownhosts, timeout, auth chain) is covered by at least one test
  3. Root-user warning (CHECK-07) is triggered and asserted when connecting as root
  4. Passwordless-sudo permission check (CHECK-04, CHECK-06) passes for a correctly configured sshuser and fails with a clear error for a misconfigured user
  5. File copy atomicity is verified — a simulated mid-copy failure leaves the target directory in its pre-deploy state
  6. All preflight checks (CHECK-01 through CHECK-07) have at least one passing and one failing scenario covered
  7. Health polling (HEALTH-01 through HEALTH-03) is exercised against a container with and without a HEALTHCHECK defined

### Phase 9: Documentation
**Goal**: README.md becomes the single authoritative resource for new users — explaining why the tool exists, how to install it, how to use it across all scenarios, and how to get help when things go wrong
**Depends on**: Phase 8
**Plans**: TBD

**Success Criteria** (what must be TRUE):
  1. README.md explains the core value proposition and motivation — why docker-deploy exists as a simpler alternative to complex CI/CD pipelines for developers who just want `scp + compose up`
  2. README.md includes a use-case section covering three scenarios: sshuser vs root, flags-only usage, and deploy.yaml config-driven usage with working examples
  3. README.md includes clear installation instructions covering binary download, plugin placement in `~/.docker/cli-plugins/`, and verification with `docker deploy --help`
  4. A comparison table of concurrent tools (Deployer, Kamal, Ansible, manual SSH scripts) is included with objective tradeoffs on complexity, dependencies, and target audience
  5. Prerequisite guide(s) cover SSH key setup and adding passwordless sudo to sshuser on the remote
  6. A troubleshooting section covers the most common failure scenarios (SSH auth failure, unknown host, writable dir, Docker not found, compose v1) with actionable fixes
  7. A feedback and suggestions section links to GitHub Issues with a welcome message for bug reports and feature requests
  8. README.md includes badges for build status, latest release version, and test status

## Progress

**Execution Order:**
Phases execute in numeric order: 1 → 2 → 3 → 4 → 5 → 6

| Phase | Plans Complete | Status | Completed |
|-------|----------------|--------|-----------|
| 1. Plugin Scaffolding | 2/2 | Complete | 2026-05-13 |
| 2. SSH Transport & Config | 3/3 | Complete | 2026-05-14 |
| 3. File Copy | 5/5 | Complete | 2026-05-15 |
| 4. Core Deploy Loop | 3/3 | Complete | 2026-05-15 |
| 5. Pre-flight & Health Polling | 4/4 | Complete | 2026-05-17 |
| 6. Init Wizard | 0/? | Not started | - |
| 7. v2 — Leftovers | 0/? | Not started | - |
| 8. Integration Tests | 0/? | Not started | - |
| 9. Documentation | 0/? | Not started | - |
