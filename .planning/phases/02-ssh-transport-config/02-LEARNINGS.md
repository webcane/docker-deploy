---
phase: 2
phase_name: "ssh-transport-config"
project: "docker-deploy"
generated: "2026-06-05"
counts:
  decisions: 7
  lessons: 5
  patterns: 5
  surprises: 3
missing_artifacts: []
---

# Phase 2 Learnings: SSH Transport & Config

## Decisions

### deploy.yaml uses versioned target subsection, not flat top-level keys
The YAML schema uses `version: 1` and a `target:` subsection (`target.host`, `target.path`) rather than flat top-level `host` and `path` keys.

**Rationale:** A `targets:` plural key can be added later for named multi-target support without breaking existing `target:` (singular) configs. Flat keys would require a breaking schema change.
**Source:** 02-01-PLAN.md, 02-01-SUMMARY.md

---

### Config resolution implemented as manual Resolve() — no Viper
Config precedence (flag > deploy.yaml > defaults) is implemented as an explicit three-tier switch in `Resolve()`. No third-party config library is used.

**Rationale:** Viper has flag-override bugs with certain flag types. Manual resolution gives full control and is easy to test with table-driven cases.
**Source:** 02-01-PLAN.md, CLAUDE.md (referenced in plan)

---

### SSH/SFTP deps pinned before source files exist using go mod edit -require
`golang.org/x/crypto` and `github.com/pkg/sftp` were pinned as direct deps in go.mod via `go mod edit -require` before any source files imported them. `go mod tidy` would otherwise demote them to indirect or remove them.

**Rationale:** Plans 02-02 and 03 depend on these deps; pinning them upfront prevents `go mod tidy` from removing them during intermediate states where no source file yet imports them.
**Source:** 02-01-SUMMARY.md

---

### SSH host key loading extracted to internal/sshconfig package
The `loadSSHConfigKeys` helper (originally specified as a private function in client.go) was extracted into its own `internal/sshconfig` package.

**Rationale:** Single-responsibility separation and testability. The sshconfig package can be tested in isolation without requiring the full SSH dial machinery.
**Source:** 02-02-SUMMARY.md

---

### TOFU and fingerprint mismatch handling live in client.go (Dial layer), not knownhosts.go
Interactive UX (prompting, printing warnings) is in `Dial()` in `client.go`. The `knownhosts.go` file provides pure mechanics (buildHostKeyCallback, appendKnownHost, error types) without any I/O.

**Rationale:** Keeps the mechanics layer testable without needing I/O mocks; keeps the UX logic in one place with access to cfg.Stdin and cfg.Stdout.
**Source:** 02-02-SUMMARY.md

---

### Auth method indicator in --dry-run is best-effort: SSH_AUTH_SOCK presence
The dry-run output line "Auth method: ssh-agent / key file" checks whether `$SSH_AUTH_SOCK` is set, not which auth method actually succeeded the handshake.

**Rationale:** The SSH library does not expose which auth method succeeded post-dial. $SSH_AUTH_SOCK presence is a reasonable heuristic for the common case.
**Source:** 02-03-SUMMARY.md

---

### runDryRun() extracted as a named helper function, keeping RunE readable
The `--dry-run` logic was moved into a `runDryRun()` helper rather than inlined in the cobra `RunE` closure.

**Rationale:** RunE bodies grow large when they contain full config resolution + SSH dial + output. A named helper function is readable and allows the cobra plumbing to remain minimal.
**Source:** 02-03-SUMMARY.md

---

## Lessons

### ClientConfig.Timeout only covers TCP handshake — SSH handshake can still hang indefinitely
Setting `ssh.ClientConfig.Timeout` protects against slow TCP connection establishment but does NOT enforce a timeout on the SSH handshake itself (key exchange, auth). A non-responsive server can stall the handshake indefinitely even after TCP connects.

**Context:** CLAUDE.md Rule 2. The fix is to wrap `ssh.Dial` in a goroutine and use `select` with `time.After(timeout)` and `ctx.Done()` to enforce the full timeout.
**Source:** 02-02-PLAN.md, 02-02-SUMMARY.md

---

### golang.org/x/crypto gets demoted to indirect by go mod tidy when no source imports it yet
`go mod tidy` is aggressive about removing or marking packages as `// indirect` when no Go source file in the module imports them. This happens during incremental plan execution where deps are added before the source files that use them exist.

**Context:** Solved by using `go mod edit -require` to explicitly mark deps as direct regardless of source imports. They get promoted automatically once plan 02-02 writes SSH source files.
**Source:** 02-01-SUMMARY.md

---

### Comment wording matters for security grep gates
A documentation comment in `client.go` that referenced the forbidden function name (`InsecureIgnoreHostKey`) caused the security grep gate to emit a false positive. Rewording to "no trust-on-first-use bypass" satisfied the gate without compromising documentation clarity.

**Context:** Security audits that scan for forbidden patterns must account for legitimate documentation references. Plan grep gates should use exclusion patterns or the codebase must use euphemisms in comments.
**Source:** 02-02-SUMMARY.md

---

### deploy.yaml silent fallback can mislead users about why --dry-run errors
When `deploy.yaml` is absent and no `--host` flag is set, the error is "no host configured: use --host flag or set target.host in deploy.yaml". A user may not realize the file was not found — the tool silently falls back to defaults without indicating whether a file was read.

**Context:** UAT test 5 revealed this confusion: user created deploy.yaml in a different directory than where `docker deploy` was run. The code was correct; the UX was insufficient.
**Source:** 02-UAT.md

---

### Integration test build tag //go:build integration is required for testcontainers tests
Tests that spin up Docker containers via testcontainers-go must be guarded with `//go:build integration` at the top of the test file. Without this tag, `go test ./...` in CI tries to run them, which requires a running Docker daemon and can timeout.

**Context:** Normal CI runs `go test ./...` without `-tags=integration`. Integration tests are run separately with `go test -tags=integration ./internal/ssh/...`.
**Source:** 02-02-PLAN.md, 02-02-SUMMARY.md

---

## Patterns

### Goroutine + select + time.After for full SSH handshake timeout
```go
go func() { ch <- result{gossh.Dial("tcp", addr, cfg)} }()
select {
case <-ctx.Done(): ...
case <-time.After(timeout): return fmt.Errorf("SSH connection timed out after %v", timeout)
case r := <-ch: ...
}
```
Also set `ClientConfig.Timeout` as belt-and-suspenders for the TCP layer.

**When to use:** Any SSH dial operation where the remote server may be slow or non-responsive. The goroutine pattern is necessary because `ssh.Dial` has no native context cancellation.
**Source:** 02-02-PLAN.md, 02-02-SUMMARY.md

---

### knownhosts.New() with error type inspection for TOFU vs. hard-fail
Wrap `knownhosts.New()` in a callback that inspects `*knownhosts.KeyError`: if `len(e.Want) > 0` the key changed (hard fail); if `len(e.Want) == 0` the host is unknown (TOFU prompt). Return typed error structs (`UnknownHostError`, `KeyMismatchError`) so the caller can branch cleanly.

**When to use:** Any SSH client that needs to distinguish "host not seen before" from "host key has changed" to implement TOFU correctly.
**Source:** 02-02-PLAN.md, 02-02-SUMMARY.md

---

### SSH agent → config key file auth chain (no password fallback)
Build auth methods as: (1) SSH agent via `$SSH_AUTH_SOCK`, silently skipped if unavailable; (2) key files from `~/.ssh/config` IdentityFile blocks via a dedicated sshconfig package; (3) fallback to default key paths (~/.ssh/id_ed25519, id_rsa, id_ecdsa). Never add `ssh.Password` or `ssh.KeyboardInteractive`.

**When to use:** SSH clients that should work with standard developer setups (ssh-agent or key files) without requiring password input. Avoids password exposure in automated contexts.
**Source:** 02-02-PLAN.md, 02-02-SUMMARY.md

---

### Three-tier config resolution: Resolve(flagHost, flagPath, file FileConfig, projectName)
Implement config precedence as an explicit function: flag values override file values which override built-in defaults. Validate and parse each layer independently (ParseHost for SSH URIs, LoadFile for YAML). Return a flat Config struct that callers use without knowing the precedence rules.

**When to use:** Any CLI tool with layered configuration (flags, file, defaults). Avoid implicit merging or library magic that can silently override values.
**Source:** 02-01-PLAN.md, 02-01-SUMMARY.md

---

### net/url for SSH URI parsing with scheme enforcement
Parse `ssh://user@host:port` URIs using `net/url.Parse()`. Enforce `scheme == "ssh"`, require non-empty hostname, and default port to 22 when absent. Return a typed `Host` struct rather than passing raw strings to the SSH dial.

**When to use:** Any configuration that accepts a connection URI. Reject early with clear error messages rather than letting malformed input propagate to the dial layer.
**Source:** 02-01-PLAN.md, 02-01-SUMMARY.md

---

## Surprises

### Live UAT discovered deploy.yaml loading appears broken (working-directory confusion)
UAT test 5 reported "no host configured" when deploy.yaml was present and no --host flag was set. Root cause was a working-directory mismatch — the file was created in a different directory than where `docker deploy` was run. The code (`LoadFile(os.Getwd())`) was correct.

**Impact:** Identified a UX gap: the tool provides no feedback when deploy.yaml is absent. An optional "No deploy.yaml found in <cwd>" message when no file config is loaded and no --host is set would have prevented the confusion.
**Source:** 02-UAT.md

---

### Dry-run against a real host produced exact expected output on first attempt
The `--dry-run` path worked end-to-end against a real Ubuntu SSH host (192.168.1.99, OpenSSH 9.6p1) on the first live test run. All three Phase 2 human verification items passed without any rework.

**Impact:** Validates that the goroutine timeout, knownhosts verification, and config resolution were all implemented correctly. The automated structural verification prior to human testing was effective.
**Source:** 02-03-SUMMARY.md, 02-HUMAN-UAT.md

---

### TOFU prompt requires exact "yes" — empty input or "y" alone is rejected
The TOFU confirmation prompt only accepts the exact string `"yes"`. Typing `"y"` alone or pressing Enter is treated as rejection. This is stricter than the OpenSSH client which accepts `"yes"` or just the interactive default.

**Impact:** Deliberate security design (per D-05 / T-02-09): only an explicit "yes" proceeds. Operators who expect `"y"` to work may be surprised. Worth documenting in the help text or prompt wording.
**Source:** 02-02-PLAN.md, 02-VERIFICATION.md
