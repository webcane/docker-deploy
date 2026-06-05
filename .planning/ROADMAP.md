# Roadmap: docker-deploy

## Milestones

- ✅ **v1.0 MVP** — Phases 1–16 (shipped 2026-06-05)

## Phases

<details>
<summary>✅ v1.0 MVP (Phases 1–16) — SHIPPED 2026-06-05</summary>

- [x] Phase 1: Plugin Scaffolding (2/2 plans) — completed 2026-05-13
- [x] Phase 2: SSH Transport & Config (3/3 plans) — completed 2026-05-14
- [x] Phase 3: File Copy (5/5 plans) — completed 2026-05-14
- [x] Phase 4: Core Deploy Loop (4/4 plans) — completed 2026-05-18
- [x] Phase 5: Pre-flight & Health Polling (4/4 plans) — completed 2026-05-17
- [x] Phase 7: v2 — Leftovers (2/2 plans) — completed 2026-05-20
- [x] Phase 8: Integration Tests (6/6 plans) — completed 2026-05-22
- [x] Phase 9: Distribution & Documentation (4/4 plans) — completed 2026-05-23
- [x] Phase 10: Shell Completions (5/5 plans) — completed 2026-06-02
- [x] Phase 11: CI & Tooling Polish (4/4 plans) — completed 2026-05-23
- [x] Phase 12: Docs Polish (4/4 plans) — completed 2026-05-24
- [x] Phase 13: CLI Subcommands & Deploy UX (10/10 plans) — completed 2026-05-26
- [x] Phase 14: SSH Config Host Alias Resolution (2/2 plans) — completed 2026-05-29
- [x] Phase 15: Deploy Healthcheck Config Format (3/3 plans) — completed 2026-05-31
- [x] Phase 16: Release Tooling Enhancement (2/2 plans) — completed 2026-05-27

Full details: [milestones/v1.0-ROADMAP.md](milestones/v1.0-ROADMAP.md)

</details>

## Progress

| Phase | Milestone | Plans Complete | Status | Completed |
|-------|-----------|----------------|--------|-----------|
| 1. Plugin Scaffolding | v1.0 | 2/2 | Complete | 2026-05-13 |
| 2. SSH Transport & Config | v1.0 | 3/3 | Complete | 2026-05-14 |
| 3. File Copy | v1.0 | 5/5 | Complete | 2026-05-14 |
| 4. Core Deploy Loop | v1.0 | 4/4 | Complete | 2026-05-18 |
| 5. Pre-flight & Health Polling | v1.0 | 4/4 | Complete | 2026-05-17 |
| 7. v2 — Leftovers | v1.0 | 2/2 | Complete | 2026-05-20 |
| 8. Integration Tests | v1.0 | 6/6 | Complete | 2026-05-22 |
| 9. Distribution & Documentation | v1.0 | 4/4 | Complete | 2026-05-23 |
| 10. Shell Completions | v1.0 | 5/5 | Complete | 2026-06-02 |
| 11. CI & Tooling Polish | v1.0 | 4/4 | Complete | 2026-05-23 |
| 12. Docs Polish | v1.0 | 4/4 | Complete | 2026-05-24 |
| 13. CLI Subcommands & Deploy UX | v1.0 | 10/10 | Complete | 2026-05-26 |
| 14. SSH Config Host Alias | v1.0 | 2/2 | Complete | 2026-05-29 |
| 15. Healthcheck Config Format | v1.0 | 3/3 | Complete | 2026-05-31 |
| 16. Release Tooling Enhancement | v1.0 | 2/2 | Complete | 2026-05-27 |

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
