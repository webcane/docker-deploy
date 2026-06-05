# Retrospective: docker-deploy

---

## Milestone: v1.0 — docker-deploy MVP

**Shipped:** 2026-06-05
**Phases:** 15 | **Plans:** 60 | **Commits:** 690
**Timeline:** 2026-05-13 → 2026-06-05 (19 days)

### What Was Built

1. Docker CLI plugin scaffold + CI pipeline (GoReleaser, GitHub Actions, ldflags version injection)
2. SSH transport with knownhosts TOFU verification, goroutine dial timeout, full auth chain
3. Atomic SFTP file staging: `/tmp/docker-deploy-<ts>` → four-step swap with rollback + single sudo prompt via SudoCreds
4. Core deploy loop: streaming `docker compose up -d` output; pre-flight checks (7 checks); per-container health polling
5. Integration test suite: testcontainers DinD+SSH covering all v1 requirements automatically
6. Distribution: GoReleaser+cosign, Homebrew tap, install.sh, bash/zsh shell completions
7. CLI subcommands: `version`, `validate`; deploy.yaml relative to cwd; verbose file diff before confirm; path-aware sudo detection
8. SSH config host alias resolution (~/.ssh/config HostName/User/Port lookup)
9. Per-service healthcheck config format (interval, timeout, retries) with strict YAML parsing
10. Extended linter coverage: gosec, errorlint, wrapcheck, gocritic, gocognit, nestif, prealloc — zero findings

### What Worked

- **Bottom-up layering**: Each phase delivered a vertically complete, verifiable capability before the next. This prevented integration surprises.
- **Four-step atomic swap design upfront**: Deciding the staging model in Phase 3 made all later sudo/rollback work straightforward.
- **SudoCreds/Zero() in Phase 13**: Consolidating all sudo machinery into a single exported type with `[]byte` credential wiping eliminated a class of bugs (password leakage, multiple prompts).
- **testcontainers DinD+SSH**: Having real integration tests against an actual SSH daemon caught CI environment issues (race conditions, DNS, host key capture) that no mock would have exposed.
- **Audit before archival**: The `v1.0-MILESTONE-AUDIT.md` pre-audit gave a clean 29/29 requirements picture before the close, making requirements archive straightforward.
- **Incremental gap-closure plans** (03-04, 03-05, 04-04, etc.): Rather than over-engineering initial plans, gap plans kept initial execution fast and handled edge cases cleanly.

### What Was Inefficient

- **Phase 6 init wizard**: Started as a v1.0 phase, never executed, eventually promoted to backlog 999.1. Should have been explicitly backlog-scoped from roadmap creation.
- **Phase 9 documentation skipped→redone in Phase 12**: Documentation was planned in Phase 9 but many plans re-executed in Phase 12 (Docs Polish). Better to have separated distribution from documentation from the start.
- **Phase 10 dynamic→static completion rework**: The original Phase 10 design used `RegisterFlagCompletionFunc` (dynamic), which was fully reworked to static completions (10-03 through 10-05). The rework was clean but avoidable with earlier design clarity.
- **CI environmental issues**: Multiple quick-tasks (DinD DNS, host key race, Node.js 20, goreleaser cosign) were caused by CI environment details discovered only after shipping. Earlier CI smoke testing would have surfaced these faster.
- **REQUIREMENTS.md checkbox drift**: Many requirements verified as complete never had their traceability table updated from "Pending" to "Complete". This created confusion at milestone close and required a documentation_gaps note in the audit.

### Patterns Established

- **Atomic staging model**: Always stage to `/tmp/<tool>-<ts>`, swap atomically, rollback on failure. Never write to final destination directly.
- **Single sudo prompt per deploy**: Capture once via interactive prompt, reuse via closure; never prompt per-file. SudoCreds/Zero() for safe memory.
- **Interface for SSH runners**: Accept `SSHRunner` interface in packages that exec commands — enables mock-free unit tests while keeping integration tests real.
- **`//go:build integration` guard**: All integration tests in `integration/` with external package name and build tag. `TestMain` owns container lifecycle — tests cannot dirty state.
- **Gap closure plans**: Don't over-engineer initial plans. Close gaps with named follow-up plans (e.g., `03-04-PLAN.md`). Better than rework or bloated initial scope.
- **Strict YAML parsing (KnownFields)**: Apply `yaml.Decoder.KnownFields(true)` to all user-facing config structs. Catches typos immediately.

### Key Lessons

1. **Lock critical dependencies first** — `github.com/docker/cli` version must be the first dependency added; transitive conflicts are painful to unwind late.
2. **SSH sessions are not reusable** — One `NewSession()` per command is a hard rule. Violating it causes subtle, hard-to-reproduce hangs.
3. **`ClientConfig.Timeout` covers only TCP** — Wrap `ssh.Dial` in goroutine + `context.WithTimeout` for true handshake timeout.
4. **DinD integration tests catch environment issues mocks cannot** — Race conditions in sshd startup, DNS resolution, host key capture are all real problems in containerized CI that mock-based tests would never expose.
5. **Deferred work needs explicit backlog promotion** — Phase 6 and 14-03 floated as "deferred" without a clear backlog entry. Explicit `999.x` backlog phases give them a home and prevent them getting lost.
6. **Document requirements as they're satisfied** — Updating traceability tables inline during phase execution (not at milestone close) avoids the documentation_gaps problem.

### Cost Observations

- Model mix: Primarily Sonnet 4.6 (yolo mode, no Opus gate)
- Sessions: Multiple across 19 days
- Notable: High-parallelization phases (08, 10) executed faster than expected; single-plan gap-closure phases were low-overhead

---

## Cross-Milestone Trends

| Milestone | Phases | Plans | Timeline | LOC | Req Coverage |
|-----------|--------|-------|----------|-----|--------------|
| v1.0 MVP | 15 | 60 | 19 days | ~12,000 | 29/29 |

*(Updated at each milestone close)*
