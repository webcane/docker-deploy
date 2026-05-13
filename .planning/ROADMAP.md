# Roadmap: docker-deploy

## Overview

The plugin is built bottom-up: scaffold the Docker CLI plugin first so the interface contract is locked, then layer in SSH transport and config resolution, then file copy, then the full deploy loop with pre-flight and health polling, and finally the isolated first-deploy wizard. Each phase delivers a vertically complete capability that can be verified end-to-end before the next phase begins.

## Phases

**Phase Numbering:**
- Integer phases (1, 2, 3): Planned milestone work
- Decimal phases (2.1, 2.2): Urgent insertions (marked with INSERTED)

Decimal phases appear between their surrounding integers in numeric order.

- [ ] **Phase 1: Plugin Scaffolding** - `docker deploy --help` works in the Docker CLI; module locked; CI configured
- [ ] **Phase 2: SSH Transport & Config** - SSH dial (knownhosts, timeout, auth chain) and config resolution work; operator can verify connectivity
- [ ] **Phase 3: File Copy** - SFTP staging-dir upload with smart include/exclude filter; files land atomically on remote
- [ ] **Phase 4: Core Deploy Loop** - `docker deploy --host ...` runs compose up on remote and streams output; exit codes correct
- [ ] **Phase 5: Pre-flight & Health Polling** - All pre-flight checks run before deploy; health polling reports pass/fail after compose up
- [ ] **Phase 6: Init Wizard** - `--init` creates target directory and writes deploy.yaml via root SSH

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
- [ ] 01-PLAN-01.md — Go module + plugin binary scaffold (plugin.Run(), cobra root, Makefile)
- [ ] 01-PLAN-02.md — CI/CD pipeline (GoReleaser cross-platform builds, GitHub Actions workflows)

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
**Plans**: TBD

### Phase 3: File Copy
**Goal**: Local project files are transferred to the remote host atomically via SFTP using smart defaults and user-defined overrides
**Mode:** mvp
**Depends on**: Phase 2
**Requirements**: DEPLOY-02, DEPLOY-03, FILES-01, FILES-02, FILES-03
**Success Criteria** (what must be TRUE):
  1. Running the copy step uploads compose.yaml, .env, Makefile, and README.md to the remote by default when no include list is configured
  2. .git/, node_modules/, vendor/, *.log, .DS_Store, and __pycache__/ are excluded from the upload by default
  3. Files are staged to `/opt/<project>/.deploy-tmp-<timestamp>` first, then moved atomically; a failed mid-copy never leaves the target directory in a partial state
  4. User can specify custom include or exclude lists in deploy.yaml and they take effect on the next deploy
**Plans**: TBD

### Phase 4: Core Deploy Loop
**Goal**: A developer can deploy a local compose project to a remote VPS with a single command and see compose output streamed to their terminal
**Mode:** mvp
**Depends on**: Phase 3
**Requirements**: DEPLOY-01, DEPLOY-04, DEPLOY-05, DEPLOY-06
**Success Criteria** (what must be TRUE):
  1. `docker deploy --host ssh://user@host:port` completes a full copy-then-compose cycle without additional flags
  2. `docker compose up -d` output is streamed line-by-line to the local terminal as it executes on the remote
  3. The plugin exits with a non-zero code if file copy fails, if the remote compose command fails, or if SSH connectivity is lost mid-deploy
**Plans**: TBD

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
**Plans**: TBD

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

## Progress

**Execution Order:**
Phases execute in numeric order: 1 → 2 → 3 → 4 → 5 → 6

| Phase | Plans Complete | Status | Completed |
|-------|----------------|--------|-----------|
| 1. Plugin Scaffolding | 0/2 | Not started | - |
| 2. SSH Transport & Config | 0/? | Not started | - |
| 3. File Copy | 0/? | Not started | - |
| 4. Core Deploy Loop | 0/? | Not started | - |
| 5. Pre-flight & Health Polling | 0/? | Not started | - |
| 6. Init Wizard | 0/? | Not started | - |
