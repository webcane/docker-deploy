---
gsd_state_version: 1.0
milestone: v1.0
milestone_name: MVP
status: archived
stopped_at: v1.0 milestone archived 2026-06-05
last_updated: "2026-06-05T14:34:18Z"
last_activity: 2026-06-05 -- Released v1.0.1
progress:
  total_phases: 16
  completed_phases: 16
  total_plans: 58
  completed_plans: 60
  percent: 100
---

# Project State

## Project Reference

See: .planning/PROJECT.md (updated 2026-06-05)

**Core value:** Deploy a local docker-compose project to any SSH-accessible VPS with a single command — no git required on the remote.
**Current focus:** Planning next milestone — run `/gsd-new-milestone` to begin
**Shipped:** v1.0 MVP — 15 phases, 60 plans, 690 commits (2026-06-05)

## Current Position

Phase: 16 of 16 — ALL PHASES COMPLETE ✓
Status: Milestone complete
Last activity: 2026-06-03 -- Phase 16 UAT verified; Phase 09 documentation UAT complete
Resume file: None

Progress: [██████████] 100%

## Performance Metrics

**Velocity:**

- Total plans completed: 13
- Average duration: ~6 min
- Total execution time: ~54 min

**By Phase:**

| Phase | Plans | Total | Avg/Plan |
|-------|-------|-------|----------|
| 1 | 2 | ~10 min | ~5 min |
| 2 | 3 | ~15 min | ~5 min |
| 3 | 3 | ~29 min | ~10 min |
| 03 | 4 | - | - |

**Recent Trend:**

- Last 5 plans: 03-03 (25min), 03-02 (8min), 03-01 (3min), 02-03, 02-02
- Trend: 03-03 longer due to real-host verification and 3 deviations applied

*Updated after each plan completion*
| Phase 13-cli-subcommands-deploy-ux P01 | 5 | 1 tasks | 1 files |
| Phase 13-cli-subcommands-deploy-ux P02 | 8 | 2 tasks | 4 files |
| Phase 13-cli-subcommands-deploy-ux P03 | 8 | 1 tasks (TDD) | 2 files |
| Phase 13-cli-subcommands-deploy-ux P04 | 7 | 2 tasks (TDD) | 3 files |
| Phase 13-cli-subcommands-deploy-ux P07 | 8 | 1 task (TDD) | 2 files |
| Phase 13-cli-subcommands-deploy-ux P05 | 12 | 1 task (TDD) | 3 files |
| Phase 13-cli-subcommands-deploy-ux P06 | 8 | 1 task (TDD) | 2 files |
| Phase 15-deploy-healthcheck-config-format P01 | 56 | 3 tasks (TDD) | 5 files |
| Phase 15-deploy-healthcheck-config-format P02 | 10 | 2 tasks | 3 files |
| Phase 10 P01 | 3min | 1 tasks | 2 files |
| Phase 10 P02 | 8min | 2 tasks (TDD) | 8 files |

## Accumulated Context

### Decisions

Decisions are logged in PROJECT.md Key Decisions table.
Recent decisions affecting current work:

- Initial: Use `plugin.Run()` from `github.com/docker/cli` — must lock version before any business logic (Phase 1)
- Initial: Manual config resolution `Resolve(flags, file, defaults)` — Viper excluded due to flag-override ordering bugs
- Initial: SFTP staging-dir pattern (`/opt/<project>/.deploy-tmp-<timestamp>`) — prevents partial-deploy state from day one
- Initial: knownhosts verification required from day one — tool copies .env files, InsecureIgnoreHostKey is unacceptable
- 03-02: ShouldExclude does three-level directory matching: exact, prefix, and component scan for deep paths
- 03-02: Upload closes sftpClient before SSH exec mv/rename commands (resource ordering)
- 03-02: Remote path operations use path (not filepath) — remote is Linux, local may be any OS
- 03-03: Staging in /tmp/docker-deploy-<ts> avoids /opt permission issues; target dir creation is separate step with sudo fallback
- 03-03: Interactive sudo with term.ReadPassword + 3 retry attempts for target dir creation on remote
- 03-03: Graceful staging fallback leaves /tmp staged files with exact recovery commands when sudo exhausted
- 03-04: ShellQuote exported from filetransfer package; main.go uses filetransfer.ShellQuote to avoid duplication
- 03-04: sudoPw captured as Upload()-local var; sudoRun closure reuses it for all sudo mv/rm ops — single prompt per deploy
- 03-04: Four-step atomic swap (staging->new, remoteBase->old, new->remoteBase, rm backup) with rollback at step-2 and step-3; backup rm is non-fatal
- 03-05: Insert rm -rf remoteBase in first-deploy else branch before mv — fixes nesting bug where mkdir-p placeholder caused mv to nest staging inside remoteBase
- 04-01: Resolve() updated to 8-arg signature; ComposeFile auto-detects compose.yaml then docker-compose.yml; Plan 02 is responsible for filepath.Base() validation of ComposeFile value before remote execution
- 04-02: RunCompose() uses session.Start() not session.Run(); wg.Wait() before session.Wait() in non-TTY path ensures pipe drain before exit check; composeFile NOT ShellQuote'd (basename validated by callers per T-04-01-01)
- 04-03: RunCompose() writes its own stderr failure line; runDeploy() returns error without double-printing; basename validation (filepath.Base check) in runDeploy() per T-04-03-01; context.Background() passed to RunCompose (compose up -d is detached)
- 05-04: preflight.NewSSHRunner(client) used at call site — RunPreflightChecks accepts SSHRunner interface, not *gossh.Client directly; CheckResult slice discarded in Phase 5 (Phase 7 will use for verbose output)
- [Phase ?]: 13-01: LoadFile already correct no config.go change needed TestLoadFile_CwdRelative added as explicit regression anchor
- 13-02: runVersionTo(w) extracted for testability; runVersion() wraps to os.Stdout; buildTime!=unknown distinguishes tagged vs dev builds
- 13-03: validate subcommand uses os.Stat before LoadFile to emit exact "deploy.yaml not found" message; Resolve(FlagOpts{}) validates file-only config; SilenceUsage=true suppresses cobra usage block on error
- 13-04: SudoCreds stores pw as []byte with Zero() for safe memory wipe; sshRun unifies sshExec+sshExecWithSudoPassword; SudoExec uses D-11 step order (direct→cached→sudo-n→interactive); promptSudoPasswordFunc package var enables test injection; prompt EOF breaks to canonical error; all 8 Upload() call sites use SudoExec (D-15 rollback paths)
- 13-07: Verbose sudo -l block inserted after id -nG in checkDockerGroup; runs regardless of docker group membership; failure silently swallowed (D-27); no signature change to RunPreflightChecks
- 13-05: Upload() now owns confirm prompt and verbose diff; force bool added after creds, before warnedOnce; sftpClient.ReadDir used for remote listing; existing repeat-deploy tests use force=true
- 13-06: needsSudo probe uses 'test -w remoteBase || test -w path.Dir(remoteBase)'; execCmd closure dispatches SudoExec vs sshRun(nil) for all 8 remoteBase ops; /tmp staging cleanup unchanged; path.Dir not filepath.Dir
- 15-01: healthcheckYAML unexported struct for YAML parsing; HealthcheckConfig exported for runtime; four-tier Resolve(opts,file,globalFile,projectName,localDir); no hardcoded defaults (D-04); loadGlobalConfig() helper in main.go tolerates missing file; all three Resolve() call sites updated
- [Phase ?]: retries==0 preserves immediate-fail on unhealthy (backward compat); retries>0 deferred-fail gate via per-container failCount map
- [Phase ?]: 15-02: healthy/no-healthcheck resets failCount[container]=0 per D-09; timeout error uses Duration.String() via %s format
- [Phase 10]: ListHosts uses strings.ContainsAny(pattern, "*?") for wildcard detection — satisfies D-03 silent-fail contract; returns nil (not empty slice) on all error paths
- [Phase 10-02]: Test assertions use cmd.GetFlagCompletionFunc() not flag.Annotations — cobra v1.10.2 stores completion funcs in global map, not as flag annotations
- [Phase 10-02]: buildCompletionCmd() uses cobra.ExactValidArgs(1) + ValidArgs=[bash,zsh] to reject unsupported shells before RunE fires (T-10-02-04)

### Roadmap Evolution

- Phase 7 added: v2 — Leftovers (expanded default excludes, --skip-env / skip_env setting, --verbose flag)
- Phase 7 edited: renamed from "Skip .env Override Option" to "Leftovers"; skip_env_override → skip_env; --verbose split into its own Wave 2
- Phase 8 added: Integration Tests (testcontainers-based suite verifying all requirements against real SSH daemon)
- Phase 9 added: Documentation (README.md — motivation, install, use-cases, comparison table, prerequisites, troubleshooting, badges, feedback section)
- Phase 10 added: add phase autosuggestion
- Phase 13 edited: renamed from "Small Code Fixes" to "CLI Subcommands & Deploy UX" (edited fields: title)
- Phase 16 edited: added Wave 0 pre-release checks (go test, test-ci, golangci-lint) gating tag+push; updated goal, renumbered criteria 1-9, added 16-00-PLAN.md
- Phase 16 edited: added Wave 3 — extend linter coverage (gosec, ineffassign, unused, bodyclose, noctx, gocritic, revive, errorlint, wrapcheck, gocognit, nestif, prealloc); added 16-03-PLAN.md; criteria 10-16
- Phase 16 edited: Wave 0 lint gate now runs lint-fix on failure then re-checks; only non-auto-fixable issues abort; 16-01-PLAN.md scope updated

### Pending Todos

- **Add ssh_dial_timeout to TargetConfig** (general) — 2026-05-26
- **Add flag to do clean-up on remote** (general) — 2026-06-02
- **[Phase 12 — Docs]** Add shell completion install instructions to README/INSTALL.md — cover Intel (`/usr/local/share/zsh/site-functions/`), Apple Silicon (`/opt/homebrew/share/zsh/site-functions/`), and portable user-local fallback (`~/.zsh/completion/` + fpath). Include bash equivalent. Surfaced during Phase 10 UAT — 2026-06-02
~~**Replace codecov with tj-actions coverage badge** (tooling) — 2026-06-03~~ ✓ done (379562e)
- **Add make test command with pass/fail summary output** (tooling) — 2026-06-03

### Blockers/Concerns

None yet.

### Quick Tasks Completed

| # | Description | Date | Commit | Directory |
|---|-------------|------|--------|-----------|
| 260519-c216 | Fix hanging goroutines in RunCompose test — refactor mock server, fix defer ordering, fix command quoting | 2026-05-19 | 833cf07 | [260519-c216-fix-hanging-goroutines](./quick/260519-c216-fix-hanging-goroutines/) |
| 260519-oax | Deduplicate 'passwordless sudo not configured' warning — show once by default, configurable via --verbose flag or config | 2026-05-19 | 40dc518 | [260519-oax-deduplicate-passwordless-sudo-not-config](./quick/260519-oax-deduplicate-passwordless-sudo-not-config/) |
| 260519-q01 | Verify docker-compose detached mode (-d flag) — confirmed already implemented in Phase 4, all tests passing | 2026-05-19 | 2150b80 | [260519-q01-docker-compose-detached-mode](./quick/260519-q01-docker-compose-detached-mode/) |
| 260519-q02 | Fix health check docker inspect exit status 1 — nil .State.Health template fix for containers with no HEALTHCHECK | 2026-05-19 | 703d819 | [260519-q02-fix-health-check-docker-inspect-ssh](./quick/260519-q02-fix-health-check-docker-inspect-ssh/) |
| 260521-afl | fix deploy complete status message: omit colon when no custom port | 2026-05-21 | 1f6103a | [260521-afl-fix-deploy-complete-status-message-omit-](./quick/260521-afl-fix-deploy-complete-status-message-omit-/) |
| 260523-n24 | Fix Node.js 20 deprecation warnings in GitHub Actions CI pipeline | 2026-05-23 | 97485a8 | [260523-n24-fix-ci-node24-actions](./quick/260523-n24-fix-ci-node24-actions/) |
| 260523-fix | Fix CI integration tests: TestDial_Timeout and TestDial_UnknownHost_TOFU_Accepted | 2026-05-23 | bc109de | [260523-fix-ci-integration-dial-tests](./quick/260523-fix-ci-integration-dial-tests/) |
| 260523-cos | Fix goreleaser cosign not found in CI release pipeline | 2026-05-23 | 4aef2ac | [260523-cos-fix-goreleaser-cosign-ci](./quick/260523-cos-fix-goreleaser-cosign-ci/) |
| 260523-ins | Fix install.sh curl URL — /main/ → /master/ (repo has no main branch) | 2026-05-23 | 53f9340 | [260523-ins-fix-install-sh-main-to-master](./quick/260523-ins-fix-install-sh-main-to-master/) |
| 260523-hkr | Fix CI DinD SSH host key capture race condition — retry loop for sshd readiness | 2026-05-23 | cd98298 | [260523-hkr-fix-ci-dind-host-key-capture](./quick/260523-hkr-fix-ci-dind-host-key-capture/) |
| 260523-lnt | Fix linting — migrate golangci-lint config to v2, fix errcheck/staticcheck/goimports across 12 files | 2026-05-23 | f6837ee | — |
| 260524-brw | Fix brew install symlink — lib/docker/cli-plugins symlink in install + system ln -sf in post_install for ~/.docker/cli-plugins | 2026-05-24 | 61a6513 | [260524-brw-fix-brew-install-symlink](./quick/260524-brw-fix-brew-install-symlink/) |
| 260527-brw | Fix brew install missing ~/.docker/cli-plugins symlink — restore post_install + sandbox_allowlist? = true (correctly combined) | 2026-05-27 | 8b4ef05 | [260527-brw-brew-symlink-post-install](./quick/260527-brw-brew-symlink-post-install/) |
| 260527-bsi | Add Apple Silicon Homebrew note to INSTALL.md — Docker does not search /opt/homebrew path by default, document symlink and cliPluginsExtraDirs options | 2026-05-27 | ff6ab48 | [260527-bsi-brew-silicon-install-note](./quick/260527-bsi-brew-silicon-install-note/) |
| 260530-hkb | Fix SSH host key capture in DinD integration tests — bound full SSH handshake per attempt via net.DialTimeout + conn.SetDeadline | 2026-05-30 | d65cccc | — |
| 260602-001 | Fix brew install: goreleaser archive missing _docker-deploy at root — add strip_parent: true to contrib files | 2026-06-02 | 0dd82b4 | [260602-001-fix-brew-install-goreleaser](./quick/260602-001-fix-brew-install-goreleaser/) |
| 260603-abc | Promote codecov replacement todo to implementation — close pending TODO (implementation in 379562e) | 2026-06-03 | — | [260603-abc-close-codecov-todo](./quick/260603-abc-close-codecov-todo/) |

## Deferred Items

Items acknowledged and deferred at milestone close on 2026-06-05:

| Category | Item | Status |
|----------|------|--------|
| quick_task | 260519-c216-fix-hanging-goroutines | missing (completed — state file absent) |
| quick_task | 260519-oax-deduplicate-passwordless-sudo-not-config | missing (completed — state file absent) |
| quick_task | 260519-q01-docker-compose-detached-mode | missing (completed — state file absent) |
| quick_task | 260519-q02-fix-health-check-docker-inspect-ssh | missing (completed — state file absent) |
| quick_task | 260521-afl-fix-deploy-complete-status-message-omit- | missing (completed — state file absent) |
| quick_task | 260523-cos-fix-goreleaser-cosign-ci | missing (completed — state file absent) |
| quick_task | 260523-fix-ci-integration-dial-tests | missing (completed — state file absent) |
| quick_task | 260523-hkr-fix-ci-dind-host-key-capture | missing (completed — state file absent) |
| quick_task | 260523-ins-fix-install-sh-main-to-master | missing (completed — state file absent) |
| quick_task | 260523-n24-fix-ci-node24-actions | missing (completed — state file absent) |
| quick_task | 260524-brw-fix-brew-install-symlink | missing (completed — state file absent) |
| quick_task | 260527-brw-brew-symlink-post-install | missing (completed — state file absent) |
| quick_task | 260527-bsi-brew-silicon-install-note | missing (completed — state file absent) |
| quick_task | 260602-001-fix-brew-install-goreleaser | missing (completed — state file absent) |
| quick_task | 260603-abc-close-codecov-todo | missing (completed — state file absent) |
| verification | 02-VERIFICATION.md | human_needed (SSH transport — live host verification deferred) |
| verification | 07-VERIFICATION.md | human_needed (verbose flag live SSH output deferred) |
| verification | 16-VERIFICATION.md | human_needed (release tooling — deferred, covered by CI) |
| uat | 09-HUMAN-UAT.md | partial (Homebrew E2E smoke test requires live tagged release) |
| todo | 2026-06-02-add-cleanup-flag-remote.md | open (future feature — not v1.0 scope) |
| todo | 2026-06-02-ssh-include-directive-support.md | open (future feature — not v1.0 scope) |
| todo | 2026-06-03-add-make-test-summary-command.md | open (tooling improvement — deferred to v1.1) |
| todo | 2026-06-03-makefile-test-cov-target.md | open (low priority tooling — deferred to v1.1) |

## Session Continuity

Last session: 2026-06-01T08:30:00.000Z
Stopped at: Phase 10 Plan 02 complete (internal/completion package + main.go wiring)
Resume file: None
Last activity: 2026-05-30 - Completed quick task 260530-hkb: Fix SSH host key capture in DinD integration tests
| 2026-05-24 | fast | add /release-tag skill | ✅ |
| 2026-05-29 | fast | fix DinD DNS - add 8.8.8.8/1.1.1.1 to daemon.json to resolve registry pull timeouts | ✅ |
| 2026-05-29 | fast | pre-pull nginx:alpine and busybox in entrypoint to avoid Docker Hub rate limits during IT | ✅ |
| 2026-06-02 | fast | fix DinD container startup timeout | ✅ |
| 2026-06-02 | fast | add package comment to completion/bash.go | ✅ |
| 2026-06-02 | fast | fix gocritic badCall: filepath.Join single arg in knownhosts_test.go | ✅ |
| 2026-06-04 | fast | Make CHECK-05 unconditional and sanitize sudo password from errors | ✅ |
