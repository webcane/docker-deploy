---
phase: 14
phase_name: "SSH Config Host Alias Resolution"
project: "docker-deploy"
generated: "2026-06-05"
counts:
  decisions: 7
  lessons: 5
  patterns: 4
  surprises: 2
missing_artifacts:
  - "14-UAT.md (no user-acceptance testing file — all verification was automated)"
---

# Phase 14 Learnings: SSH Config Host Alias Resolution

## Decisions

### Bare host string with no ssh:// prefix is treated as an alias
Any value for `--host` or `deploy.yaml target.host` that lacks an `ssh://` prefix is interpreted as a `~/.ssh/config` alias. This applies to both the CLI flag and the YAML key (D-01, D-02).

**Rationale:** Users already have short aliases configured in their SSH config; requiring a full `ssh://user@hostname:port` URL duplicates that configuration. Prefix detection is the simplest disambiguation: `ssh://` URIs are already unambiguous.
**Source:** 14-01-PLAN.md

---

### HostEntry.HostName falls back to the alias label when HostName directive is absent
When a `Host` block is matched but contains no `HostName` directive, `LookupHost()` returns the alias string itself as `HostEntry.HostName` (D-07).

**Rationale:** This mirrors OpenSSH's behaviour: if no `HostName` is specified, the alias label is used as the connection target. Callers do not need to special-case a missing HostName.
**Source:** 14-01-PLAN.md

---

### Port=0 and User="" are deliberately not defaulted by LookupHost
`LookupHost()` returns zero values when the `Port` or `User` directives are absent from the matched Host block (D-08, D-09). Defaulting to port 22 and OS username is the responsibility of the caller (runDeploy).

**Rationale:** Keeping the parser free of defaults makes it composable — callers can apply their own layered defaults without fighting the parser.
**Source:** 14-01-PLAN.md, 14-01-SUMMARY.md

---

### parseIdentityFiles removed; LoadSigners becomes a thin wrapper over LookupHost
The private `parseIdentityFiles()` function was removed entirely. `LoadSigners()` now calls `LookupHost()` and uses `HostEntry.IdentityFiles` (D-10).

**Rationale:** `LookupHost()` already performs the same single-pass scan needed by `parseIdentityFiles`. Having two parallel scanners over the same file was duplication; `LoadSigners` as a thin wrapper preserves the public API while eliminating the redundancy.
**Source:** 14-01-PLAN.md, 14-01-SUMMARY.md

---

### Include directives silently skipped with a TODO comment
`~/.ssh/config` `Include` directives are ignored — the parser continues past them without error (D-11).

**Rationale:** Implementing recursive Include parsing was out of scope for Phase 14. Skipping silently (rather than erroring) means users with Include lines still get alias resolution for directives in the primary config file. The TODO comment documents the known limitation.
**Source:** 14-01-PLAN.md

---

### LoadFile returns (FileConfig, bool, error); the bool signals whether the file existed
`LoadFile()` was changed from a two-return to a three-return signature to allow callers to distinguish "file absent" from "file present but unconfigured".

**Rationale:** The previous generic "no host configured" error left users wondering whether their deploy.yaml was being found at all. The bool enables two distinct, actionable error messages.
**Source:** 14-02-PLAN.md, 14-02-SUMMARY.md

---

### NoHostError is exported from the config package; message strings live in one place
`NoHostError(fileExisted bool, dir string) error` is an exported helper so that both error message strings are defined once in the config package and can be unit-tested there, without duplicating string literals in main.go (D-15).

**Rationale:** Co-locating the error logic with the config package allows testing the exact strings without involving the main package. Callers simply pass through `fileExisted`.
**Source:** 14-02-PLAN.md, 14-02-SUMMARY.md

---

## Lessons

### LookupHost must use HostEntry.HostName (not the alias) as the known_hosts key
The resolved `HostEntry.HostName` (the real hostname from the `HostName` directive) must flow to `DialConfig.Hostname` and thence to known_hosts verification. Passing the alias label would mean the host key check is done against a name that may never appear in `known_hosts`.

**Context:** Security requirement T-14-01-01: a spoofed alias could bypass host key checking if the alias label (not the real hostname) were used as the verification key.
**Source:** 14-01-PLAN.md (threat model), 14-VERIFICATION.md

---

### resolveHostString() helper cleanly separates alias from URL paths
Introducing a private `resolveHostString(raw, configPath string) (Host, error)` helper — rather than inline branching inside `Resolve()` — keeps the alias detection logic testable in isolation (same package, callable directly from config_test.go).

**Context:** The helper is unexported but in package `config`, so white-box tests can call it directly. This approach avoided the need for integration-level tests just to cover the alias path.
**Source:** 14-01-PLAN.md, 14-01-SUMMARY.md

---

### Non-NotExist read errors in LoadFile return fileExisted=false conservatively
If `os.ReadFile` fails for a reason other than `os.IsNotExist` (e.g. permission denied), `LoadFile` returns `fileExisted=false`. This means the caller emits the "file not found" message variant, which is conservative but non-misleading.

**Context:** A permission error on a file that does exist is unusual; treating it as "we don't know if it existed" avoids an incorrect "target.host is not set" message that implies the file was parsed.
**Source:** 14-02-PLAN.md

---

### runValidate uses a blank identifier for fileExisted
`runValidate()` already performs its own `os.Stat` check to detect file presence; it uses `_` for the `fileExisted` bool from `LoadFile`. This avoids a conflicting dual-source of truth.

**Context:** When updating a three-return-value signature across multiple call sites, each call site must be audited for what it actually needs from each return value.
**Source:** 14-02-PLAN.md, 14-02-SUMMARY.md

---

### SSH config parsing with bufio.Scanner requires explicit handling of the last block
When scanning `~/.ssh/config` line by line, the end-of-file condition must set `found = true` if the active block matched. Without this, the last `Host` block in the file would never be returned.

**Context:** A second `Host` keyword triggers the "block ended" check for the previous block. With no trailing `Host` keyword, only the EOF path triggers it.
**Source:** 14-01-PLAN.md

---

## Patterns

### Synthetic URL construction from HostEntry fields
Convert a `HostEntry` (HostName, User, Port) into a synthetic `ssh://[user@]hostname[:port]` URL string, then pass it through the existing `ParseHost()` function. This reuses the existing URL parser for both alias and direct-URL code paths.

**When to use:** Any time a non-URL structured value needs to flow into a pipeline that expects a URL. Constructing the URL and parsing it is safer than building the target struct by hand.
**Source:** 14-01-PLAN.md, 14-01-SUMMARY.md

---

### Three-return-value signature for "may-not-exist" file loaders
`func LoadFile(dir string) (FileConfig, bool, error)` — the bool communicates existence independently of the error. This pattern is cleaner than using sentinel errors or special struct fields.

**When to use:** When callers need to distinguish "file absent (normal)" from "file present but failed to parse" from "file present and parsed successfully".
**Source:** 14-02-PLAN.md

---

### White-box testing of unexported helpers in the same package
Unexported helpers like `resolveHostString` and `sshConfigPath` can be tested directly in `_test.go` files in the same package (`package config`, not `package config_test`). This avoids the need to export test-only symbols.

**When to use:** When a helper is too important to leave without unit tests but too low-level to warrant exporting.
**Source:** 14-01-PLAN.md

---

### Exported error helper with boolean parameter for variant messages
`NoHostError(fileExisted bool, dir string) error` — a single exported function returns one of two distinct messages depending on a boolean. The strings are defined once, testable via the config package alone.

**When to use:** When multiple callers need to produce one of a small set of context-variant error messages and you want to keep the strings DRY and testable.
**Source:** 14-02-PLAN.md, 14-02-SUMMARY.md

---

## Surprises

### Phase 14 originally scoped only alias resolution but grew to include error message improvements
The phase plan included a second plan (14-02) covering `LoadFile`'s three-return signature and context-aware "no host" errors. These are logically related but were not part of the initial alias resolution objective.

**Impact:** The phase delivered more UX value than its name suggests. The two-plan structure (14-01: alias resolution, 14-02: error messages) kept each plan focused, but the scope expansion added approximately 8 minutes of additional execution time.
**Source:** 14-02-PLAN.md, 14-02-SUMMARY.md

---

### No deviations from plan across either plan — executed exactly as written
Both 14-01 and 14-02 report "None — plan executed exactly as written." This is notable for a phase involving refactoring a signature that has multiple call sites across packages.

**Impact:** Confirms that reading all call sites before changing a function signature (as specified in the plan's read_first sections) is sufficient to prevent mid-execution surprises. The explicit list of call sites in the plan (runDeploy, runDryRun, runValidate) caught the three-site update before coding started.
**Source:** 14-01-SUMMARY.md, 14-02-SUMMARY.md
