# Phase 15: Deploy Healthcheck Config Format - Context

**Gathered:** 2026-05-30
**Status:** Ready for planning

<domain>
## Phase Boundary

Replace the existing flat `health_timeout` / `health_interval` integer keys in `deploy.yaml` with a proper `target.healthcheck:` sub-block that uses Docker-style duration strings and adds a `retries` field. Wire corresponding CLI flags. The same settings apply to all containers in the compose project (no per-service overrides needed — Phase 5 already handles the no-HEALTHCHECK case correctly).

**In scope:**
- New `target.healthcheck:` block in deploy.yaml schema with `interval`, `timeout`, `retries` fields using duration strings (e.g. `10s`, `1m30s`)
- Remove old `health_timeout` / `health_interval` flat integer keys from `TargetConfig` and `Config`; emit a deprecation warning if they appear in deploy.yaml (yaml.v3 silently ignores unknown keys, so the warning must be explicit)
- Four-tier precedence: flag > local `deploy.yaml target.healthcheck` > global config (`~/.docker/cli-plugins/deploy.yaml`) > no healthcheck (section absent = health polling skipped)
- CLI flags: `--healthcheck-timeout`, `--healthcheck-interval`, `--healthcheck-retries` (duration string format for timeout/interval, integer for retries)
- Update `internal/health/poll.go` to use `retries` (stop polling after N consecutive unhealthy results before timeout fires)
- Update `internal/config/config.go`: new `Healthcheck` struct, updated `Resolve()` parsing, duration string → `time.Duration` conversion, negative-value validation
- Update config tests and health poll tests for new types

**Out of scope:**
- Per-service healthcheck overrides (no `target.services` map)
- Disabling health polling per service or via a flag — omitting `target.healthcheck` from all config files is the supported way to skip health polling entirely
- Changes to container enumeration strategy (still uses `com.docker.compose.project` label as in Phase 5)

</domain>

<decisions>
## Implementation Decisions

### Config Format

- **D-01:** New YAML block is `target.healthcheck` with three sub-keys: `interval`, `timeout`, `retries`. Example:
  ```yaml
  target:
    healthcheck:
      interval: 10s
      timeout: 30s
      retries: 5
  ```
- **D-02:** Duration values use Docker-style strings (`10s`, `1m30s`, `2m`). Parsed via `time.ParseDuration`. Plain integers are not accepted in the new block.
- **D-03:** `retries` is a plain integer (not a duration). Represents maximum consecutive unhealthy results before declaring failure.
- **D-04:** No hardcoded defaults. Default values are defined in the global config (`~/.docker/cli-plugins/deploy.yaml`) pre-populated with `interval: 10s`, `timeout: 30s`, `retries: 3` (commented out, discoverable). If the `healthcheck:` block is absent from both global and local config, health polling is skipped entirely.
- **D-05:** ~~yaml.v3 silently ignores unknown fields~~ **REVISED (2026-05-31):** `LoadFile()` uses `yaml.NewDecoder` with `KnownFields(true)` — ALL unknown fields in `deploy.yaml` are a hard parse error. This makes typos in `healthcheck:` keys (e.g. `retrise` instead of `retries`) immediately visible rather than silently producing zero values. Old flat keys (`health_timeout`, `health_interval`) now produce a parse error, prompting users to migrate. This is a deliberate breaking change accepted after UAT gap finding.

### Config Resolution (four-tier precedence)

- **D-06:** Resolution chain: CLI flags > local `deploy.yaml target.healthcheck` > global config `~/.docker/cli-plugins/deploy.yaml target.healthcheck` > absent (no health polling). No hardcoded fallback values in code — defaults live in the global config file.
- **D-07:** CLI flags added to `main.go` and `FlagOpts`: `--healthcheck-timeout` (string), `--healthcheck-interval` (string), `--healthcheck-retries` (int).
- **D-08:** Flag string values are parsed via `time.ParseDuration` in `Resolve()`. Invalid duration strings are rejected with a clear error.

### Retries Semantics

- **D-09:** `retries` = max consecutive unhealthy results before fail. After N consecutive polls where a container is `unhealthy`, `PollHealth` returns a non-nil error immediately without waiting for the timeout. A single `healthy` result resets the consecutive counter.
- **D-10:** Retries counter is per-container. One container hitting `retries` does not affect polling of other containers in the project.

### Backward Compatibility

- **D-11:** ~~yaml.v3 silently ignores unknown fields~~ **REVISED (2026-05-31, aligned with D-05):** With `KnownFields(true)` active on `LoadFile()`, old flat keys (`health_timeout`, `health_interval`) produce a clear parse error rather than being silently ignored. No special deprecation message is needed — the parse error is the signal.
- **D-12:** Existing unit tests for old flat keys should be updated to test the new `healthcheck:` block format.

### Claude's Discretion

- Whether to detect deprecated keys via a second `yaml.Node` scan or via a separate intermediate struct with those fields — pick the simplest approach that avoids duplicating the full YAML parse.
- Error message wording for invalid duration strings (keep consistent with existing error style in `config.go`).
- Whether to store `Healthcheck` as a nested struct on `Config` (e.g. `Config.Healthcheck.Timeout time.Duration`) or flatten the fields back onto `Config` (e.g. `Config.HealthTimeout time.Duration`) — either is fine; prefer whichever keeps `poll.go` changes minimal.

</decisions>

<canonical_refs>
## Canonical References

**Downstream agents MUST read these before planning or implementing.**

### Existing Implementation (read before modifying)
- `internal/config/config.go` — `TargetConfig`, `Config`, `FlagOpts`, `Resolve()` — all need updating; `HealthTimeout` / `HealthInterval` fields are removed and replaced
- `internal/health/poll.go` — `PollHealth()` / `pollHealthWithRunner()` — needs retries counter per container; currently uses `cfg.HealthTimeout` / `cfg.HealthInterval` as `time.Duration` casts; default interval changes from 5s to 10s, timeout from 60s to 30s
- `internal/config/config_test.go` — existing health field tests must be migrated to new format
- `internal/health/poll_test.go` — existing polling tests must be migrated to new `time.Duration` config fields
- `cmd/docker-deploy/main.go` — flag registration for `--healthcheck-timeout`, `--healthcheck-interval`, `--healthcheck-retries`

### Prior Phase Context
- `.planning/phases/05-preflight-health-polling/05-CONTEXT.md` — D-11 through D-13: original health polling design decisions (global timeout/interval, terminal states, no-healthcheck warning-and-pass behaviour)
- `.planning/ROADMAP.md` §Phase 15 — Phase goal and success criteria

### Critical Constraints (from CLAUDE.md)
- `CLAUDE.md` — Rule 3: each SSH command uses a separate `client.NewSession()` — sessions are NOT reusable

</canonical_refs>

<code_context>
## Existing Code Insights

### Reusable Assets
- `internal/config/config.go` `Resolve()` precedence pattern — the same flag > file > default switch pattern used for `HealthTimeout` / `HealthInterval` can be reused for the new fields; replace the integer arithmetic with `time.ParseDuration`
- `internal/health/poll.go` `pollHealthWithRunner()` — the poll loop already tracks `done` per container via a map; a `failCount` map alongside it handles per-container retries
- `filetransfer.ShellQuote()` — already imported in `poll.go`; no change needed

### Established Patterns
- Config validation: negative values rejected with `fmt.Errorf("deploy.yaml: field must be >= 0, got %d")`; apply same pattern to retries
- Duration zero-value guard: `if healthInterval <= 0 { healthInterval = time.Millisecond }` in `poll.go` — update to use the new `time.Duration` field directly
- yaml.v3 struct tags: all `TargetConfig` fields use `yaml:"snake_case"` tag format

### Integration Points
- `cmd/docker-deploy/main.go`: flags are registered in `newDeployCmd()` via `cmd.Flags().*VarP()` calls; `FlagOpts` is populated before `Resolve()` is called
- `internal/config/config.go` `Resolve()`: both flag-based and file-based `HealthTimeout` / `HealthInterval` fields are read here; replace with new duration fields + retries

</code_context>

<specifics>
## Specific Ideas

- User explicitly wants the `healthcheck:` block to mirror Docker Compose HEALTHCHECK syntax (`interval`, `timeout`, `retries`) — familiar to operators already writing docker-compose services.
- Duration string format (`10s`, `1m30s`) matches what operators already write in `compose.yaml` HEALTHCHECK entries.

</specifics>

---

*Phase: 15-deploy-healthcheck-config-format*
*Context gathered: 2026-05-30*
