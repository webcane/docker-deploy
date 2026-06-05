---
phase: 02-ssh-transport-config
verified: 2026-05-14T10:00:00Z
status: human_needed
score: 5/5 must-haves verified
overrides_applied: 0
human_verification:
  - test: "docker deploy --host ssh://user@realhost:22 --dry-run exits 0 and prints Host, Remote path, Auth method, Server version, Status: OK"
    expected: "Dry-run: connectivity check passed with full resolved config printed"
    why_human: "Requires a live SSH-accessible host; cannot verify end-to-end SSH handshake without running the binary against a real server"
  - test: "TOFU prompt: first connection to a new host prints fingerprint and prompts yes/no"
    expected: "Prompt appears; 'yes' appends to known_hosts; 'no' exits non-zero"
    why_human: "Interactive terminal behavior — requires a running SSH server and a clean known_hosts entry to trigger the TOFU path"
  - test: "docker deploy --host ssh://user@192.0.2.1 --dry-run exits non-zero within ~10 seconds with timeout error"
    expected: "Error message contains 'timed out'; process exits within ~10 seconds"
    why_human: "Requires observing wall-clock timeout behavior in a running process"
---

# Phase 2: SSH Transport & Config Verification Report

**Phase Goal:** The plugin can open a verified SSH connection to a remote host and resolve configuration from flags, deploy.yaml, and defaults in the correct precedence order
**Verified:** 2026-05-14T10:00:00Z
**Status:** human_needed
**Re-verification:** No — initial verification

## Goal Achievement

### Observable Truths

All 5 ROADMAP success criteria were verified against the actual codebase.

| # | Truth | Status | Evidence |
|---|-------|--------|----------|
| 1 | `--host ssh://user@host:port` flag is accepted and parsed correctly | VERIFIED | `ParseHost()` in `internal/config/config.go` uses `net/url` with scheme enforcement; 8 table-driven tests all pass including user, hostname, port extraction |
| 2 | SSH dial uses knownhosts verification; unknown host prompts for confirmation rather than failing silently or accepting blindly | VERIFIED | `buildHostKeyCallback()` wraps `knownhosts.New()`; `handleTOFU()` in `client.go` prompts user; only exact `"yes"` proceeds; `//go:build integration` tests `TestDial_UnknownHost` and `TestDial_UnknownHostAccepted` cover this path |
| 3 | SSH handshake timeout is enforced — a non-responsive host does not hang the process indefinitely | VERIFIED | `goroutine + select + time.After(timeout)` pattern confirmed at lines 133-148 of `client.go`; `ctx.Done()` also handled; `ClientConfig.Timeout` set too (belt-and-suspenders) |
| 4 | `deploy.yaml` values are loaded when present; flag values override deploy.yaml; deploy.yaml overrides built-in defaults | VERIFIED | `config.Resolve()` implements three-tier switch; `TestResolveHostPrecedence` and `TestResolvePathPrecedence` cover all 7 precedence cases; all 16 config tests pass |
| 5 | `deploy.yaml` schema accommodates future multi-target keys without breaking existing single-target configs | VERIFIED | `FileConfig` uses `Target TargetConfig` subsection with `version` int field; flat top-level `host`/`path` keys are ignored (confirmed by `TestLoadFile/deploy.yaml_schema_uses_target_subsection_not_flat_keys`) |

**Score:** 5/5 truths verified

### Required Artifacts

| Artifact | Expected | Status | Details |
|----------|----------|--------|---------|
| `internal/config/config.go` | Config struct, Resolve(), ParseHost(), LoadFile() | VERIFIED | All four exports present; 148 lines, fully implemented, no stubs |
| `internal/config/config_test.go` | Unit tests for Resolve() precedence and ParseHost() format | VERIFIED | 16 tests across 4 test functions; all pass |
| `internal/ssh/client.go` | Dial() function, DialConfig struct, goroutine timeout | VERIFIED | 249 lines; exports Dial() and DialConfig; goroutine+select pattern at lines 133-148 |
| `internal/ssh/knownhosts.go` | TOFU prompt, hard-fail, known_hosts write | VERIFIED | UnknownHostError, KeyMismatchError, buildHostKeyCallback(), appendKnownHost(), formatFingerprint() all present |
| `internal/sshconfig/sshconfig.go` | SSH config key parsing (deviation from plan — extracted package) | VERIFIED | Exists; LoadSigners() reads ~/.ssh/config IdentityFile blocks; silently skips failures; falls back to default key paths |
| `internal/ssh/client_test.go` | Integration tests with `//go:build integration` tag | VERIFIED | Build tag confirmed at line 1; TestDial_Timeout, TestDial_UnknownHost, TestDial_UnknownHostAccepted, TestDial_Success all present |
| `cmd/docker-deploy/main.go` | --host, --path, --dry-run flags; config.Resolve + sshpkg.Dial wired | VERIFIED | All three flags registered; runDryRun() calls config.Resolve() at line 69 and sshpkg.Dial() at line 94 |

### Key Link Verification

| From | To | Via | Status | Details |
|------|----|-----|--------|---------|
| `internal/ssh/client.go` | `internal/ssh/knownhosts.go` | `wrappedCallback` calls `buildHostKeyCallback()` | WIRED | Lines 94-116 of client.go call buildHostKeyCallback and wrap the result |
| `internal/ssh/client.go` | `golang.org/x/crypto/ssh/agent` | `agent.NewClient(agentConn).Signers` | WIRED | Lines 159-165 of client.go; SSH_AUTH_SOCK socket used |
| `cmd/docker-deploy/main.go` | `internal/config.Resolve` | `config.Resolve(host, path, fileConfig, projectName)` | WIRED | Line 69 of main.go |
| `cmd/docker-deploy/main.go` | `internal/ssh.Dial` | `sshpkg.Dial(context.Background(), dialCfg)` | WIRED | Line 94 of main.go |
| `internal/ssh/client.go` | `internal/sshconfig.LoadSigners` | `loadSSHConfigKeys()` calls `sshconfig.LoadSigners()` | WIRED | Line 169 of client.go; `internal/sshconfig` package imported at line 17 |

### Data-Flow Trace (Level 4)

| Artifact | Data Variable | Source | Produces Real Data | Status |
|----------|--------------|--------|-------------------|--------|
| `main.go runDryRun` | `resolved` (Config) | `config.Resolve(host, path, fileConfig, projectName)` | Yes — fileConfig loaded from deploy.yaml via LoadFile(); flags from cobra | FLOWING |
| `main.go runDryRun` | `client` (*ssh.Client) | `sshpkg.Dial(context.Background(), dialCfg)` | Yes — real SSH dial with knownhosts verification | FLOWING |
| `main.go runDryRun` | `client.Conn.ServerVersion()` | Live SSH server response via `client.Conn` | Yes — SSH server banner from live connection | FLOWING |

### Behavioral Spot-Checks

| Behavior | Command | Result | Status |
|----------|---------|--------|--------|
| Config unit tests pass | `go test ./internal/config/... -v -count=1` | 16 tests PASS | PASS |
| Build succeeds | `go build ./...` | exit 0 | PASS |
| Vet clean | `go vet ./...` | exit 0 | PASS |
| No InsecureIgnoreHostKey anywhere | `grep -r 'InsecureIgnoreHostKey' internal/` | 0 matches | PASS |
| No password auth methods | `grep 'gossh.Password\|KeyboardInteractive' internal/ssh/client.go` | 0 matches | PASS |
| goroutine+select timeout pattern | `grep -n 'select {' && grep -n 'time.After'` | lines 138, 141 | PASS |
| TOFU: only exact "yes" accepted | `grep 'answer != "yes"'` | line 209 of client.go | PASS |

### Probe Execution

No probe scripts declared for this phase. Step 7c: SKIPPED (no probe-*.sh files found).

### Requirements Coverage

| Requirement | Source Plan | Description | Status | Evidence |
|-------------|------------|-------------|--------|----------|
| CFG-01 | 02-01, 02-02, 02-03 | `--host` flag accepts `ssh://user@host:port` format | SATISFIED | ParseHost() validates scheme, hostname, port; cobra flag registered; Resolve() consumes it |
| CFG-02 | 02-01, 02-03 | `--path` flag overrides default remote target directory | SATISFIED | cobra flag registered; Resolve() path precedence: flagPath > filePath > "/opt/"+projectName |
| CFG-03 | 02-01, 02-03 | `deploy.yaml` in project root stores persistent config | SATISFIED | LoadFile() reads deploy.yaml from os.Getwd(); called in runDryRun() |
| CFG-04 | 02-01, 02-03 | Flag > deploy.yaml > defaults precedence | SATISFIED | Resolve() implements three-tier switch; all table-driven precedence tests pass |
| CFG-05 | 02-01 | `deploy.yaml` schema forward-compatible with multi-target | SATISFIED | FileConfig uses versioned `Target TargetConfig` subsection; flat keys ignored; `targets` (plural) can be added without breaking `target` (singular) |

All 5 requirements claimed by Phase 2 plans are satisfied.

**Orphaned requirements check:** REQUIREMENTS.md maps CFG-01 through CFG-05 to Phase 2 — all 5 are covered by the plans. No orphaned requirements.

### Anti-Patterns Found

| File | Line | Pattern | Severity | Impact |
|------|------|---------|----------|--------|
| `cmd/docker-deploy/main.go` | 36 | `return fmt.Errorf("deploy not implemented yet")` | Info | Intentional stub for non-dry-run path; planned for Phase 4 |

No TBD, FIXME, XXX, or blocking anti-patterns found. The "deploy not implemented yet" stub is intentional and explicitly expected per 02-03-PLAN.md acceptance criteria.

**Security scan (CLAUDE.md critical rules):**
- `InsecureIgnoreHostKey`: 0 occurrences anywhere in the codebase (full scan confirmed)
- SSH dial timeout: goroutine + `select` + `time.After(timeout)` pattern confirmed; `ctx.Done()` also handled
- `knownhosts.New()` used as base callback; no bypass path exists

### Human Verification Required

Automated checks passed. The following items require a live SSH server to verify end-to-end behavior.

#### 1. Successful dry-run against a real host

**Test:** `docker deploy --host ssh://youruser@yourhost:22 --dry-run`
**Expected:**
```
Dry-run: connectivity check passed
  Host:        youruser@yourhost:22
  Remote path: /opt/<project-dir-name>
  Auth method: ssh-agent / key file
  Server:      SSH-2.0-OpenSSH_...
  Status:      OK
```
**Why human:** Requires a live SSH-accessible host; cannot verify the full SSH handshake path (including server version string) without running against a real server. (Note: 02-03-SUMMARY.md reports this was verified live against 192.168.1.99 with OpenSSH 9.6p1 during plan execution — this is supporting context but not verifier evidence.)

#### 2. TOFU prompt on first connection to unknown host

**Test:** Run `docker deploy --host ssh://user@newhost --dry-run` with an empty or host-absent `~/.ssh/known_hosts`
**Expected:** Fingerprint printed, `[yes/no]` prompt appears; "yes" proceeds and appends to known_hosts; "no" exits non-zero
**Why human:** Interactive terminal path; requires clean known_hosts state and a responding SSH server

#### 3. Timeout enforcement against non-responsive host

**Test:** `docker deploy --host ssh://user@192.0.2.1 --dry-run`
**Expected:** Error containing "timed out" within approximately 10 seconds; non-zero exit code
**Why human:** Requires observing wall-clock process behavior; `192.0.2.1` is a TEST-NET address guaranteed to be non-routable

### Gaps Summary

No gaps. All 5 ROADMAP success criteria are verified in the codebase. The phase goal is structurally achieved — the plugin can open a verified SSH connection and resolve configuration with correct precedence.

The only items requiring human attention are live-connectivity behaviors that depend on a running SSH server: successful handshake output, TOFU interactive prompt, and timeout wall-clock behavior. These are observational confirmations of working code, not missing implementations.

---

_Verified: 2026-05-14T10:00:00Z_
_Verifier: Claude (gsd-verifier)_
