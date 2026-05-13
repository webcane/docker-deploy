# Requirements: docker-deploy

**Defined:** 2026-05-13
**Core Value:** Deploy a local docker-compose project to any SSH-accessible VPS with a single command — no git required on the remote.

## v1 Requirements

### Plugin (PLUG)

- [ ] **PLUG-01**: Binary named `docker-deploy` in `~/.docker/cli-plugins/` responds to `docker deploy` in the Docker CLI
- [ ] **PLUG-02**: `docker deploy --help` displays command usage natively within Docker CLI help
- [ ] **PLUG-03**: Plugin responds to `docker-cli-plugin-metadata` with valid metadata JSON

### Deploy (DEPLOY)

- [ ] **DEPLOY-01**: User can deploy with `docker deploy --host ssh://user@host:port`
- [ ] **DEPLOY-02**: Project files are copied to remote via SFTP (`github.com/pkg/sftp` over the existing SSH connection)
- [ ] **DEPLOY-03**: Files are staged to a temp directory first (`/opt/<project>/.deploy-tmp-<timestamp>`) via SFTP, then moved atomically on the remote to prevent partial-deploy state
- [ ] **DEPLOY-04**: `docker compose up -d` is executed on the remote via SSH after file copy
- [ ] **DEPLOY-05**: Plugin exits with non-zero code if any step (pre-flight, copy, compose) fails
- [ ] **DEPLOY-06**: Deploy output (compose up result) is streamed back to the local terminal

### File Management (FILES)

- [ ] **FILES-01**: Default include list: compose.yaml (or docker-compose.yml), .env, Makefile, README.md
- [ ] **FILES-02**: Default exclude list: .git/, node_modules/, vendor/, *.log, .DS_Store, __pycache__/
- [ ] **FILES-03**: User can extend or override include/exclude lists via deploy.yaml

### Configuration (CFG)

- [ ] **CFG-01**: `--host` flag accepts `ssh://user@host:port` format
- [ ] **CFG-02**: `--path` flag overrides default remote target directory (default: `/opt/<project_name>`)
- [ ] **CFG-03**: `deploy.yaml` in project root stores persistent config (host, path, include, exclude)
- [ ] **CFG-04**: Flag values take precedence over deploy.yaml; deploy.yaml takes precedence over defaults
- [ ] **CFG-05**: `deploy.yaml` schema is forward-compatible with future multi-target extension (v2)

### Pre-Flight Checks (CHECK)

- [ ] **CHECK-01**: Verify Docker is installed on remote host
- [ ] **CHECK-02**: Verify `docker compose` v2 plugin is installed (not docker-compose v1, which is EOL)
- [ ] **CHECK-03**: Verify Docker daemon is running on remote
- [ ] **CHECK-04**: Check if SSH user is in the `docker` group; if not, attempt to create group and add user (requires sudo)
- [ ] **CHECK-05**: Check if user has sudo access on remote
- [ ] **CHECK-06**: Verify target directory is writable; if not, attempt `sudo mkdir -p` and `sudo chown` to fix
- [ ] **CHECK-07**: Warn (but do not block) if SSH user is root — deploying as root is dangerous

### Post-Deploy Health (HEALTH)

- [ ] **HEALTH-01**: After `docker compose up -d`, poll container health status via `docker inspect`
- [ ] **HEALTH-02**: Report final health result: pass (healthy) / fail (unhealthy) / unknown (no healthcheck)
- [ ] **HEALTH-03**: Exit with non-zero code if any container reaches `unhealthy` state

### Init Wizard (INIT)

- [ ] **INIT-01**: `--init` flag triggers setup wizard using root SSH credentials
- [ ] **INIT-02**: Wizard detects on first deploy if target directory doesn't exist or isn't writable, and offers to run init
- [ ] **INIT-03**: Wizard creates target directory (`/opt/<project>`) and sets correct ownership (`chown`) for the deploy user
- [ ] **INIT-04**: Wizard writes `deploy.yaml` with host, user, and path after successful setup

## v2 Requirements

### Observability

- **OBS-01**: `--detach` / `-d` flag for fire-and-forget mode (exit immediately after `compose up` without streaming logs)
- **OBS-02**: Warn when compose.yaml has no HEALTHCHECK instruction defined

### Multi-Target

- **MULTI-01**: Named deploy targets in deploy.yaml (e.g., staging, prod) selectable via `--env`
- **MULTI-02**: `docker deploy --env prod` deploys to named target config

### Init Wizard Extended

- **INIT-EXT-01**: `--init` with root SSH creates a dedicated non-root deploy user
- **INIT-EXT-02**: New deploy user is added to docker group during init

## Out of Scope

| Feature | Reason |
|---------|--------|
| Docker remote context setup | Adds local complexity; observability via SSH exec is sufficient for v1 |
| Container registry / image push | Tool is file-copy-based; image building is user's responsibility |
| Blue-green / zero-downtime | Requires reverse proxy in compose.yaml — user-controlled |
| Rollback | Out of scope for v1; atomic staging-dir pattern limits blast radius |
| Git on VPS | Explicit non-requirement — the entire point of the tool |
| docker-compose v1 compatibility | EOL since June 2023; v1 is a hard-fail pre-flight condition |
| Web UI / server daemon | Tool is CLI-only; server-side complexity is a different product (Coolify, Dokploy) |
| Secrets vault integration | Out of scope; .env copied as-is, user is responsible |

## Traceability

Updated during roadmap creation.

| Requirement | Phase | Status |
|-------------|-------|--------|
| PLUG-01 | Phase 1 | Pending |
| PLUG-02 | Phase 1 | Pending |
| PLUG-03 | Phase 1 | Pending |
| DEPLOY-01 | Phase 4 | Pending |
| DEPLOY-02 | Phase 3 | Pending |
| DEPLOY-03 | Phase 3 | Pending |
| DEPLOY-04 | Phase 4 | Pending |
| DEPLOY-05 | Phase 4 | Pending |
| DEPLOY-06 | Phase 4 | Pending |
| FILES-01 | Phase 3 | Pending |
| FILES-02 | Phase 3 | Pending |
| FILES-03 | Phase 3 | Pending |
| CFG-01 | Phase 2 | Pending |
| CFG-02 | Phase 2 | Pending |
| CFG-03 | Phase 2 | Pending |
| CFG-04 | Phase 2 | Pending |
| CFG-05 | Phase 2 | Pending |
| CHECK-01 | Phase 4 | Pending |
| CHECK-02 | Phase 4 | Pending |
| CHECK-03 | Phase 4 | Pending |
| CHECK-04 | Phase 4 | Pending |
| CHECK-05 | Phase 4 | Pending |
| CHECK-06 | Phase 4 | Pending |
| CHECK-07 | Phase 4 | Pending |
| HEALTH-01 | Phase 4 | Pending |
| HEALTH-02 | Phase 4 | Pending |
| HEALTH-03 | Phase 4 | Pending |
| INIT-01 | Phase 5 | Pending |
| INIT-02 | Phase 5 | Pending |
| INIT-03 | Phase 5 | Pending |
| INIT-04 | Phase 5 | Pending |

**Coverage:**
- v1 requirements: 27 total
- Mapped to phases: 27
- Unmapped: 0 ✓

---
*Requirements defined: 2026-05-13*
*Last updated: 2026-05-13 after initial definition*
