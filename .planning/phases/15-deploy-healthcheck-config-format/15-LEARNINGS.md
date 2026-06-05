---
phase: 15
phase_name: "Deploy Healthcheck Config Format"
project: "docker-deploy"
generated: "2026-06-05"
counts:
  decisions: 8
  lessons: 6
  patterns: 5
  surprises: 4
missing_artifacts: []
---

# Phase 15 Learnings: Deploy Healthcheck Config Format

## Decisions

### No hardcoded healthcheck defaults in config.go — zero value means "absent"
When all three tiers (flag, local deploy.yaml, global config) are absent or empty, the resulting `HealthcheckConfig` fields are zero values. Health polling is skipped entirely when interval and timeout are zero (D-04).

**Rationale:** Defaults belong in the global config file (`~/.docker/cli-plugins/deploy.yaml`), not in code. Hardcoded defaults make it impossible for users to suppress health polling by omitting the block.
**Source:** 15-01-PLAN.md, 15-01-SUMMARY.md

---

### Separate YAML-parsing struct (healthcheckYAML) from the runtime struct (HealthcheckConfig)
The unexported `healthcheckYAML` struct has string-typed fields for YAML parsing; the exported `HealthcheckConfig` has `time.Duration` fields for runtime use. Conversion happens inside `Resolve()`.

**Rationale:** `yaml.v3` cannot unmarshal directly into `time.Duration` from a string like `"30s"`. A parallel struct with string fields allows `LoadFile()` to be a pure YAML deserializer, keeping duration parsing in `Resolve()` where errors can be attributed to their source.
**Source:** 15-01-PLAN.md, 15-01-SUMMARY.md

---

### Four-tier precedence: flag > local deploy.yaml > global config > zero
`Resolve()` was extended with a `globalFile FileConfig` parameter, giving three data tiers plus the zero fallback. Each field (Interval, Timeout, Retries) is resolved independently through the same four-tier chain (D-06).

**Rationale:** A global config file at `~/.docker/cli-plugins/deploy.yaml` allows operators to set organisation-wide defaults without requiring every project to have a deploy.yaml. Local file and flags always win.
**Source:** 15-01-PLAN.md, 15-01-SUMMARY.md

---

### Negative durations are explicitly rejected after time.ParseDuration
Go's `time.ParseDuration` accepts strings like `"-5s"` and parses them successfully to a negative `time.Duration`. After parsing, each tier checks `if d < 0` and returns a source-naming error (threat T-15-01-02).

**Rationale:** A negative interval would cause `time.NewTicker` to panic. The validation is explicit rather than relying on the caller to catch it later.
**Source:** 15-01-PLAN.md, 15-01-SUMMARY.md

---

### retries==0 preserves the existing immediate-fail behaviour
When `cfg.Healthcheck.Retries == 0`, a single unhealthy result still terminates health polling immediately. The retries gate only activates when `Retries > 0` (backward compatibility decision for Phase 15-02).

**Rationale:** Existing users who have not configured retries should see no behaviour change. Opt-in semantics avoid surprising regressions in deployed setups.
**Source:** 15-02-PLAN.md, 15-02-SUMMARY.md

---

### Per-container failCount map; one container hitting the threshold does not reset others
`pollContainers` receives a `failCount map[string]int` and a `retries int`. Each container has an independent counter. A single container reaching the threshold causes an immediate fail, but the counter for other containers is unaffected.

**Rationale:** D-10 specifies per-container tracking. Project-level fail-fast on the first offending container is the documented behaviour — it matches Docker Compose's own health model.
**Source:** 15-02-PLAN.md, 15-02-SUMMARY.md

---

### loadGlobalConfig() helper extracted in main.go to avoid triplicating load logic
A private `loadGlobalConfig()` helper loads `~/.docker/cli-plugins/deploy.yaml`, tolerates missing files (returns empty `FileConfig{}`), and propagates other errors with the path in the message. All three `Resolve()` call sites use it.

**Rationale:** The load-and-tolerate-missing pattern is six lines; repeating it three times (runValidate, runDryRun, runDeploy) would create three divergence points. Extracting it keeps the sites identical.
**Source:** 15-01-PLAN.md, 15-01-SUMMARY.md

---

### KnownFields(true) on yaml.Decoder replaces yaml.Unmarshal to reject unknown keys
`LoadFile()` was changed from `yaml.Unmarshal` to `yaml.NewDecoder` with `.KnownFields(true)`. Any unknown field in `deploy.yaml` now returns a parse error naming the offending key (Plan 15-03).

**Rationale:** Silent unknown-field acceptance meant a typo like `retrise: 3` would silently produce `retries=0`. The fix was a UAT gap discovered during human testing. Strict parsing is the correct default for a deployment tool where misconfiguration has real consequences.
**Source:** 15-03-PLAN.md, 15-03-SUMMARY.md

---

## Lessons

### Silent unknown-field YAML parsing is a real UX hazard for structured sub-blocks
The Phase 15 UAT identified that `yaml.Unmarshal` silently dropped unknown fields, so a user typing `retrise: 3` in the `healthcheck` block got `retries=0` with no feedback. This was filed as a major gap.

**Context:** This issue did not surface during automated testing because tests only used correct field names. Human UAT was necessary to discover it. The fix (Plan 15-03) required a separate plan and additional commits after the initial verification had already passed.
**Source:** 15-UAT.md, 15-03-PLAN.md

---

### --dry-run output must show resolved config for users to verify flag/file precedence
Without a `Healthcheck:` row in dry-run output, users cannot verify whether their flags and YAML are being applied correctly without performing a real deploy. This was a minor UAT gap.

**Context:** The fix was to extract a `formatHealthcheckRow` helper and call it unconditionally in `runDryRun()`. The helper also got its own unit tests, making the formatting logic independently verifiable.
**Source:** 15-UAT.md, 15-03-PLAN.md, 15-03-SUMMARY.md

---

### poll.go and integration tests broke at build time when config struct fields were removed
Removing `cfg.HealthTimeout` and `cfg.HealthInterval` from the `Config` struct caused `go build` to fail in `internal/health/poll.go` and `integration/compose_test.go` — both referenced the removed fields and were not covered by the initial plan scope.

**Context:** The blocking deviation was caught during Task 2 of 15-01 when `go build ./...` failed. The fix was applied in the same commit (8cb3b4e). This illustrates that removing struct fields in Go requires checking all consumers across the entire module, not just the package being changed.
**Source:** 15-01-SUMMARY.md

---

### time.Duration fields in a config struct cannot be populated directly by yaml.v3
`yaml.v3` does not know how to unmarshal a string like `"30s"` into a `time.Duration` field. A parallel unexported struct with string-typed fields is required for YAML parsing.

**Context:** This was anticipated in the plan design (D-05) and shaped the `healthcheckYAML` / `HealthcheckConfig` split. The lesson is to check yaml.v3's type support before designing structs that mix duration fields with YAML tags.
**Source:** 15-01-PLAN.md

---

### healthcheck retries reset on healthy — a single healthy result zeroes the counter
The per-container `failCount` is reset to zero on any `"healthy"` or `"no-healthcheck"` result (D-09). This means a container that oscillates between unhealthy and healthy can poll indefinitely (bounded by the timeout).

**Context:** Documented in T-15-02-04: the timeout timer always fires, bounding the worst case. The reset-on-healthy semantic mirrors Docker Compose's own health model and was the intended behaviour, but it is worth making explicit that retries provide a threshold for consecutive failures, not a cumulative failure budget.
**Source:** 15-02-PLAN.md, 15-02-SUMMARY.md

---

### Timeout error message format changed from integer seconds to Duration.String()
The old error printed `"Health check timed out after %ds: ..."` (integer seconds). The new form uses `cfg.Healthcheck.Timeout` directly with `%s`, which invokes `Duration.String()` and produces human-readable output like `"30s"` or `"1m30s"`.

**Context:** This is a minor UX improvement but was required anyway because the field type changed from `int` to `time.Duration`. The `%s` verb on a `time.Duration` is idiomatic Go and self-documenting.
**Source:** 15-02-PLAN.md, 15-02-SUMMARY.md

---

## Patterns

### Parallel unexported YAML struct + exported runtime struct for typed config fields
Define an unexported `healthcheckYAML` struct with `string` fields and yaml tags for `LoadFile()`. Define an exported `HealthcheckConfig` struct with `time.Duration` and `int` fields for runtime use. `Resolve()` parses from the YAML form to the runtime form.

**When to use:** Whenever a config field needs a type that yaml.v3 cannot unmarshal directly (durations, URLs, custom types). Keeps `LoadFile()` as a pure deserializer and `Resolve()` as the single location for validation and conversion.
**Source:** 15-01-PLAN.md, 15-01-SUMMARY.md

---

### Four-tier precedence with independent per-field resolution
For each field, check: flag string non-empty? → parse and assign. Local file string non-empty? → parse and assign. Global file string non-empty? → parse and assign. Otherwise leave at zero. Each field resolved independently using the same pattern.

**When to use:** Any config value that should be overridable at multiple levels (flag > project > global > absent). The repeated structure is intentional and makes the precedence immediately readable.
**Source:** 15-01-PLAN.md, 15-01-SUMMARY.md

---

### Per-container state map passed into a polling helper function
`pollContainers(runner, containers, done, failCount, retries)` — both the `done` map and the `failCount` map are declared by the caller (`pollHealthWithRunner`) and passed in. The helper mutates both maps.

**When to use:** When a polling loop needs to maintain per-item state across invocations. Passing maps in rather than returning them keeps the call site simple and makes it easy to inspect state after each poll without returning a complex value.
**Source:** 15-02-PLAN.md, 15-02-SUMMARY.md

---

### formatHealthcheckRow helper for testable output formatting
Extract CLI output formatting into a pure `formatHealthcheckRow(hc config.HealthcheckConfig) string` helper. The helper is unit-tested independently; `runDryRun()` just calls `fmt.Fprintln(os.Stdout, formatHealthcheckRow(...))`.

**When to use:** Any time CLI summary output has non-trivial conditional logic (e.g. "disabled" vs formatted values). Testing the helper directly is faster and more precise than testing full command output.
**Source:** 15-03-PLAN.md, 15-03-SUMMARY.md

---

### KnownFields(true) as the default for all YAML config loading
Replace `yaml.Unmarshal(data, &target)` with `dec := yaml.NewDecoder(bytes.NewReader(data)); dec.KnownFields(true); dec.Decode(&target)`. The change is small but the protection is significant.

**When to use:** All YAML config loading where the schema is known and stable. Operators should receive an immediate, actionable error when they typo a field name rather than silent misconfiguration.
**Source:** 15-03-PLAN.md, 15-03-SUMMARY.md

---

## Surprises

### A plan 15-03 gap-closure plan was required after verification had passed
Plans 15-01 and 15-02 passed automated verification (14/14 truths), but human UAT then found two gaps: silent YAML typo acceptance (major) and missing dry-run healthcheck output (minor). A third plan (15-03) was needed to close them.

**Impact:** The automated verification missed both issues because they required user-facing interaction to surface. Strict YAML parsing and dry-run output completeness are inherently UAT concerns, not unit test concerns. This validated the value of human UAT as a distinct gate from automated verification.
**Source:** 15-UAT.md, 15-03-PLAN.md

---

### poll.go was outside Plan 15-01's stated scope but had to be fixed in the same commit
The plan for 15-01 listed `internal/config/config.go`, `internal/config/config_test.go`, and `cmd/docker-deploy/main.go` as the files modified. But removing `cfg.HealthTimeout` and `cfg.HealthInterval` from `Config` made `poll.go` fail to compile. The fix was applied in 8cb3b4e alongside the config changes.

**Impact:** The plan deviation tracking accurately recorded this as an auto-fixed blocking issue. The lesson: when removing fields from a widely-used struct, plan for consumers outside the stated file scope.
**Source:** 15-01-SUMMARY.md

---

### integration/compose_test.go also needed migration outside the stated plan scope
Three call sites in `integration/compose_test.go` that constructed `config.Config{HealthTimeout: N, HealthInterval: M}` also broke. These were migrated in Plan 15-02, but the need was first discovered during Plan 15-01's `go build ./...` step.

**Impact:** Multi-package builds (`go build ./...`) are essential after struct-field removals. A single-package build would have missed the integration test file.
**Source:** 15-01-SUMMARY.md, 15-02-PLAN.md, 15-02-SUMMARY.md

---

### Duration.String() for partial healthcheck (e.g. interval only) produces "0s" for zero timeout
The `TestFormatHealthcheckRow` test added a third sub-case ("partial") to cover `Interval=30s, Timeout=0, Retries=0`. `time.Duration(0).String()` returns `"0s"`, which is technically correct but may look odd to users.

**Impact:** Low — the "disabled" path requires all three fields to be zero, so a partial config will always show field values including "0s". This is a display quirk rather than a bug, but it is worth noting for future dry-run output refinement.
**Source:** 15-03-SUMMARY.md
