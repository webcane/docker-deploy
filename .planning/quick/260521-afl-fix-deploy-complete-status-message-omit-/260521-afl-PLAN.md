---
phase: quick-260521-afl
plan: 01
type: execute
wave: 1
depends_on: []
files_modified:
  - cmd/docker-deploy/main.go
  - cmd/docker-deploy/main_test.go
autonomous: true
requirements: []
must_haves:
  truths:
    - "Deploy complete message omits the colon when using default port 22"
    - "Deploy complete message includes :PORT when a non-default port is used"
  artifacts:
    - path: cmd/docker-deploy/main.go
      provides: "Fixed deploy complete status message formatting"
      contains: "formatHostTarget"
    - path: cmd/docker-deploy/main_test.go
      provides: "Tests for both default-port and custom-port message formats"
  key_links:
    - from: "runDeploy()"
      to: "fmt.Fprintf(os.Stdout, Deploy complete...)"
      via: "formatHostTarget() helper"
      pattern: "formatHostTarget"
---

<objective>
Fix the deploy completion status message to omit the colon separator when the default SSH port (22) is used.

Purpose: The current format `host:/path` is confusing — it looks like a host:port prefix with an empty port. The correct formats are `host/path` (default port) and `host:2222/path` (custom port).
Output: Updated main.go with a `formatHostTarget` helper and a test covering both cases.
</objective>

<execution_context>
@/Users/mniedre/git/docker-deploy/.claude/get-shit-done/workflows/execute-plan.md
@/Users/mniedre/git/docker-deploy/.claude/get-shit-done/templates/summary.md
</execution_context>

<context>
@/Users/mniedre/git/docker-deploy/.planning/PROJECT.md
@/Users/mniedre/git/docker-deploy/.planning/ROADMAP.md
@/Users/mniedre/git/docker-deploy/.planning/STATE.md
@/Users/mniedre/git/docker-deploy/cmd/docker-deploy/main.go
@/Users/mniedre/git/docker-deploy/cmd/docker-deploy/main_test.go
</context>

<tasks>

<task type="auto" tdd="true">
  <name>Task 1: Fix deploy complete message format and add tests</name>
  <files>cmd/docker-deploy/main.go, cmd/docker-deploy/main_test.go</files>
  <behavior>
    - formatHostTarget("192.168.1.99", 22, "/opt/test-deploy") -> "192.168.1.99/opt/test-deploy"
    - formatHostTarget("192.168.1.99", 2222, "/opt/test-deploy") -> "192.168.1.99:2222/opt/test-deploy"
    - formatHostTarget("192.168.1.99", 0, "/opt/test-deploy") -> "192.168.1.99/opt/test-deploy" (0 treated as default)
  </behavior>
  <action>
    In main_test.go, add TestFormatHostTarget with three table-driven sub-tests covering the three cases above. Run the tests — they will fail (RED) because formatHostTarget does not exist yet.

    In main.go, add a package-level helper function:

      func formatHostTarget(hostname string, port int, path string) string {
          if port == 0 || port == 22 {
              return hostname + path
          }
          return fmt.Sprintf("%s:%d%s", hostname, port, path)
      }

    Replace line 346 in runDeploy():
      // Before:
      fmt.Fprintf(os.Stdout, "Deploy complete: %d files copied to %s:%s\n", fileCount, resolved.Host.Hostname, resolved.Path)
      // After:
      fmt.Fprintf(os.Stdout, "Deploy complete: %d files copied to %s\n", fileCount, formatHostTarget(resolved.Host.Hostname, port, resolved.Path))

    The variable `port` is already in scope at line 346 (resolved in step 5, assigned to dialCfg).
  </action>
  <verify>
    <automated>cd /Users/mniedre/git/docker-deploy && go test ./cmd/docker-deploy/... -run TestFormatHostTarget -v</automated>
  </verify>
  <done>
    TestFormatHostTarget passes for all three cases. go test ./... passes with no regressions.
  </done>
</task>

</tasks>

<threat_model>
## Trust Boundaries

| Boundary | Description |
|----------|-------------|
| stdout output | Informational message only — no user-controlled data in the format string itself |

## STRIDE Threat Register

| Threat ID | Category | Component | Disposition | Mitigation Plan |
|-----------|----------|-----------|-------------|-----------------|
| T-afl-01 | Information Disclosure | formatHostTarget stdout | accept | Port number in success message is benign — user already knows their own port config |
</threat_model>

<verification>
Run full test suite to confirm no regressions:

```
cd /Users/mniedre/git/docker-deploy && go test ./...
```

Manual smoke-check of output format:
- Default port (22 or 0): message ends with `host/path` (no colon before path)
- Custom port (e.g. 2222): message ends with `host:2222/path`
</verification>

<success_criteria>
- `go test ./cmd/docker-deploy/... -run TestFormatHostTarget` passes (3 sub-tests)
- `go test ./...` passes with no regressions
- Line 346 of main.go uses `formatHostTarget()` instead of bare `%s:%s`
- Default port 22 produces `host/path` format (no colon)
- Custom port produces `host:PORT/path` format
</success_criteria>

<output>
After completion, create `.planning/quick/260521-afl-fix-deploy-complete-status-message-omit-/260521-afl-SUMMARY.md`
</output>
