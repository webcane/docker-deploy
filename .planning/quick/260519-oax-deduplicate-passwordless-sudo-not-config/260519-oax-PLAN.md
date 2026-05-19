---
phase: quick
plan: 260519-oax
type: execute
wave: 1
depends_on: []
files_modified:
  - internal/filetransfer/upload.go
  - cmd/docker-deploy/main.go
autonomous: true
requirements: []
user_setup: []

must_haves:
  truths:
    - "Warning 'passwordless sudo not configured' appears only once per deploy by default"
    - "User can see warning on every password prompt via --verbose flag"
    - "Password-handling logic preserved; warning does not affect sudo attempts"
  artifacts:
    - path: "internal/filetransfer/upload.go"
      provides: "Warning deduplication flag passed to sudoRunWithFallback"
    - path: "cmd/docker-deploy/main.go"
      provides: "Warning state tracking across Upload() call"
  key_links:
    - from: "cmd/docker-deploy/main.go"
      to: "internal/filetransfer/upload.go"
      via: "Upload() signature extended with warningFlag parameter"
      pattern: "filetransfer.Upload.*warningFlag"
---

<objective>
Prevent the "WARNING: passwordless sudo not configured; you may be prompted for a password" message from appearing multiple times during a single deploy operation.

Purpose: Users are confused by repeated identical warnings during file transfers when interactive sudo is required. One clear warning per deploy is sufficient; --verbose allows one-per-prompt for debugging.

Output: Warning appears once per deploy by default, with toggle for verbose mode to show on each attempt.
</objective>

<execution_context>
@/Users/mniedre/git/docker-deploy/.planning/STATE.md
@/Users/mniedre/git/docker-deploy/CLAUDE.md
</execution_context>

<context>
Project state: Phase 5 (Preflight) complete; Phase 6 underway. The warning is printed in `internal/filetransfer/upload.go:181` inside the `sudoRunWithFallback` closure, which can be called multiple times per Upload() invocation (mkdir -p, mv operations, rm -rf).

Current behavior:
- tryPasswordlessSudo() fails → warning printed once
- Same closure called 1-3 more times for subsequent sudo ops → warning printed again

Expected behavior:
- First sudo attempt that requires password → warning printed
- Subsequent sudo attempts in same deploy → no warning (unless --verbose)
- User maintains password prompt behavior (no changes to auth attempt logic)

Implementation model:
- Add `warnOnce` bool parameter to Upload()
- Pass a pointer to a flag tracking "warned in this deploy" (similar to sudoPw pattern)
- Print warning only if flag is false AND tryPasswordlessSudo fails
- After first warning, set flag to true
- In main.go, initialize the flag before calling Upload()
- Optional: accept --verbose flag to override deduplication (deferred to Phase 7, per STATE.md)
</context>

<tasks>

<task type="auto">
  <name>Task 1: Add warning deduplication flag to Upload() signature</name>
  <files>internal/filetransfer/upload.go</files>
  <action>
Update the Upload() function signature to accept a pointer to a bool tracking whether the passwordless sudo warning has already been printed in the current deploy. This follows the same pattern as sudoPw (*string).

At line 67, change:
  func Upload(ctx context.Context, client *gossh.Client, localDir, remoteBase string, excludes []string, sudoPw *string) (int, error)

To:
  func Upload(ctx context.Context, client *gossh.Client, localDir, remoteBase string, excludes []string, sudoPw *string, warnedOnce *bool) (int, error)

Inside sudoRunWithFallback closure (line 169), update the warning logic at line 181 to:
- Check if *warnedOnce is true; if so, skip warning (don't print)
- If *warnedOnce is false AND tryPasswordlessSudo fails, print warning and set *warnedOnce = true
- Preserve all password prompt retry logic (lines 182-195 unchanged)

Exact change: Replace line 181:
  fmt.Fprintf(os.Stderr, "WARNING: passwordless sudo not configured; you may be prompted for a password\n")

With:
  if !*warnedOnce {
    fmt.Fprintf(os.Stderr, "WARNING: passwordless sudo not configured; you may be prompted for a password\n")
    *warnedOnce = true
  }

This ensures the warning prints exactly once per Upload() call, no matter how many times sudoRunWithFallback executes (mkdir -p, mv, rm operations).
  </action>
  <verify>
    <automated>grep -A 3 "if !.*warnedOnce" /Users/mniedre/git/docker-deploy/internal/filetransfer/upload.go | grep -c "warnedOnce = true"</automated>
  </verify>
  <done>Upload() signature accepts warnedOnce *bool parameter. Warning logic wrapped in if !*warnedOnce block. Flag set to true after first warning. Grep confirms pattern in place.</done>
</task>

<task type="auto">
  <name>Task 2: Initialize warning flag in main.go and pass to Upload()</name>
  <files>cmd/docker-deploy/main.go</files>
  <action>
In main.go, initialize a warnedOnce flag before calling Upload(), following the same pattern as sudoPw initialization (lines 245-247).

After line 247 (*sudoPw = ""), add:
  var warnedOnce *bool
  warnedOnce = new(bool)
  *warnedOnce = false

At line 248 where filetransfer.Upload() is called, append warnedOnce as the final argument:
  fileCount, err := filetransfer.Upload(context.Background(), client, cwd, resolved.Path, resolved.Excludes, sudoPw, warnedOnce)

Verify the call signature matches the updated Upload() function from Task 1.
  </action>
  <verify>
    <automated>grep -n "warnedOnce = new(bool)" /Users/mniedre/git/docker-deploy/cmd/docker-deploy/main.go</automated>
  </verify>
  <done>main.go initializes warnedOnce pointer to false. Upload() call includes warnedOnce as final argument. No syntax errors.</done>
</task>

<task type="auto">
  <name>Task 3: Verify deduplication via existing tests and manual integration test</name>
  <files>internal/filetransfer/upload_test.go</files>
  <action>
Run existing upload tests to ensure no regressions. The deduplication flag is internal to Upload() and does not change the auth fallback sequence (password prompt logic unchanged).

Command: go test -v ./internal/filetransfer -run TestUpload

Verify all tests pass. Tests use mock SSH clients; the warnedOnce flag is a simple bool pointer and does not affect mock behavior.

Additionally, scan upload_test.go for any test assertions checking stderr output. If tests verify the warning message is present, confirm they still pass (they should — the warning still prints on first attempt, just not on subsequent attempts).
  </action>
  <verify>
    <automated>go test -v ./internal/filetransfer -run TestUpload 2>&1 | grep -c "PASS"</automated>
  </verify>
  <done>All upload tests pass. Warning deduplication does not break existing test expectations. Code compiles without errors.</done>
</task>

</tasks>

<threat_model>
No new threat surface introduced. The deduplication flag is a local bool (not user-controlled input, not stored, not transmitted). Password handling and sudo execution logic remain unchanged.
</threat_model>

<verification>
End-to-end: Deploy to a remote requiring interactive sudo. Confirm:
1. Warning "passwordless sudo not configured" appears exactly once in stderr
2. Subsequent file ops (mv, rm) do not repeat the warning
3. User is still prompted for password on each sudo attempt (behavior preserved)
4. After entering correct password, deploy completes successfully
</verification>

<success_criteria>
- Warning appears once per deploy by default (deduplication working)
- All existing tests pass (no regression)
- Code compiles without errors
- Git commit created with message: "fix(04): deduplicate passwordless sudo warning"
</success_criteria>

<output>
After completion, create `.planning/quick/260519-oax-deduplicate-passwordless-sudo-not-config/260519-oax-SUMMARY.md` with:
- Files modified: internal/filetransfer/upload.go, cmd/docker-deploy/main.go
- Changes: Added warnedOnce bool pointer to Upload() signature; warning prints once per deploy
- Tests: All upload tests pass
- Next: Optional --verbose flag support deferred to Phase 7
</output>
