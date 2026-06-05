---
phase: 3
phase_name: "file-copy"
project: "docker-deploy"
generated: "2026-06-05"
counts:
  decisions: 8
  lessons: 6
  patterns: 6
  surprises: 4
missing_artifacts:
  - "03-UAT.md (no separate UAT file — human UAT results are in 03-HUMAN-UAT.md)"
---

# Phase 3 Learnings: File Copy

## Decisions

### Stage uploads in /tmp/docker-deploy-<timestamp>, not sibling-of-target
Staging directory moved from the originally planned `/opt/<project>/.deploy-tmp-<ts>` to `/tmp/docker-deploy-<ts>` on the remote.

**Rationale:** /tmp is always writable by the SSH user. Staging inside /opt requires write permission on /opt itself, which is root-owned on a typical VPS. Moving staging to /tmp shifts the permission challenge solely to target directory creation, which sudo handles.
**Source:** 03-03-SUMMARY.md

---

### Interactive sudo with term.ReadPassword, up to 3 attempts, graceful fallback
Target directory creation uses `mkdir -p` first (no sudo). On failure, prompt for password via `term.ReadPassword` locally, pipe to `sudo -S -p ''` on remote, retry up to 3 times. On exhaustion: leave staged files in /tmp and print exact manual recovery commands.

**Rationale:** Matches familiar sudo UX (3 attempts). Graceful fallback ensures the operator can always recover without re-uploading files. Using `term.ReadPassword` avoids the password appearing in process args.
**Source:** 03-03-SUMMARY.md, 03-04-PLAN.md

---

### sudoPw captured as a local variable in Upload() and reused via sudoRun closure
The sudo password collected during directory creation is captured in a local `sudoPw string` variable. A `sudoRun` closure routes all subsequent mv/rm commands through `echo <pw> | sudo -S -p '' sh -c <cmd>` when `sudoPw` is non-empty.

**Rationale:** Avoids prompting the user more than once per deploy invocation. The password is held only on the stack within Upload(), never logged or written to disk.
**Source:** 03-04-PLAN.md, 03-04-SUMMARY.md

---

### ShellQuote exported (not unexported) so main.go can use it without duplication
`shellQuote` was renamed `ShellQuote` (exported) so `cmd/docker-deploy/main.go` can call `filetransfer.ShellQuote(resolved.Path)` for the remote existence check, rather than duplicating the escaping logic.

**Rationale:** Single implementation of shell quoting; avoids two diverging copies of the same security-critical function.
**Source:** 03-04-PLAN.md, 03-04-SUMMARY.md

---

### Backup-dir cleanup failure is non-fatal in the atomic swap
After the three-step atomic swap succeeds (new version placed at remoteBase), the `rm -rf <backup>` cleanup step prints a warning to stderr on failure but does not return an error.

**Rationale:** The deploy succeeded; an orphaned backup directory is a cleanup annoyance, not a correctness failure. Returning an error would incorrectly signal deploy failure to the operator.
**Source:** 03-04-PLAN.md, 03-04-SUMMARY.md

---

### mergeExcludes: built-ins always first, append-only, dedup by string equality
Exclude list merge order: `defaultExcludes` (6 built-ins) → `file.Target.Exclude` → `flagExcludes`. Deduplication uses a seen-map (O(n), insertion-order preserving). User input can extend but never remove built-in excludes.

**Rationale:** Default excludes (.git/, node_modules/, vendor/, etc.) are unconditional safety rails. An append-only model with dedup means users cannot accidentally expose .git/ by supplying a conflicting exclude list.
**Source:** 03-01-PLAN.md, 03-01-SUMMARY.md

---

### Replace-confirmation default is No ([y/N]) — only explicit "y" or "yes" proceeds
The repeat-deploy confirmation prompt defaults to No. Pressing Enter cancels without error. Only case-insensitive "y" or "yes" proceeds.

**Rationale:** Accidental Enter must never trigger an overwrite of the live remote target. This is a safety-first UX default that users can bypass permanently with `--force` or `force: true` in deploy.yaml.
**Source:** 03-03-PLAN.md, 03-03-SUMMARY.md

---

### In-process SSH server + pkg/sftp InMemHandler for Upload unit tests
Upload() integration tests use an in-process SSH server (golang.org/x/crypto/ssh server API) with `pkg/sftp`'s `NewRequestServer` + `InMemHandler()` for file uploads. No testcontainers or external Docker daemon required.

**Rationale:** Logic-level tests for Upload() (command ordering, first-deploy vs. repeat-deploy paths) do not require a real SSH daemon. An in-process mock captures exec commands, makes assertions on their order, and avoids CI infrastructure dependencies.
**Source:** 03-05-SUMMARY.md

---

## Lessons

### /opt is root-owned on typical VPS — staging sibling-of-target fails at the permission level
The original plan assumed staging as a sibling directory under /opt (e.g., `/opt/myapp/.deploy-tmp-<ts>`). Creating any directory under /opt requires the SSH user to have write permission on /opt itself, which they typically do not have. This failed on the first real-host test.

**Context:** The permission model on a standard Ubuntu VPS is: /opt owned by root, user-owned subdirectories granted by admin. The tool must not assume the user can write to /opt directly.
**Source:** 03-03-SUMMARY.md

---

### mkdir -p creates the target directory before mv, causing Unix mv to nest stagingDir inside it
Step 8 of Upload() runs `mkdir -p remoteBase` to create the target directory. When `mv stagingDir remoteBase` runs in the first-deploy else branch, the target directory already exists, so Unix `mv` moves stagingDir *inside* remoteBase rather than renaming it. Files land at `remoteBase/<staging-name>/`.

**Context:** The fix is to `rm -rf remoteBase` immediately before the `mv` in the first-deploy path, so `mv` performs a clean rename. The `existsBefore=true` (repeat-deploy) path does not have this problem because it uses the three-step atomic swap.
**Source:** 03-05-PLAN.md, 03-05-SUMMARY.md, 03-HUMAN-UAT.md

---

### Resolve() signature change breaks all existing call sites immediately
Extending `Resolve()` from 4 to 6 parameters caused compilation failures in both `config_test.go` and `main.go` before any new tests were written. Call sites had to be updated in the same commit as the signature change.

**Context:** Planned for Task 2 but required in Task 1 to allow Task 1 verification to compile. Atomic commits should include all call site updates when changing a widely-used function signature.
**Source:** 03-01-SUMMARY.md

---

### sudoRun lazy approach: try without sudo first, collect password on failure
The initial implementation of `sudoRun` only used sudo when `sudoPw` was already non-empty. On repeat deploy, `mkdir -p` succeeds without sudo (user now owns the directory from the first deploy), leaving `sudoPw` empty. Subsequent mv ops against root-owned /opt then failed silently.

**Context:** The fix was to make `sudoRun` try each command without sudo first; on failure, collect the password interactively and retry. This also handles edge cases where some ops need sudo and others do not.
**Source:** 03-04-SUMMARY.md

---

### go.sum entry needed even when go.mod already lists a dep as direct
`github.com/pkg/sftp` was listed as a direct dependency in go.mod from Phase 2, but go.sum lacked the entry needed for a direct package import. Running `go get github.com/pkg/sftp@v1.13.10` explicitly was required to populate go.sum even though the module was already listed.

**Context:** Go distinguishes between having a module listed in go.mod and having it in go.sum as a directly imported package. The go.sum entry for direct imports is more specific than for transitive deps.
**Source:** 03-02-SUMMARY.md

---

### WalkFiles uses filepath.SkipDir on excluded directories for efficiency
When `ShouldExclude` returns true for a directory entry in WalkFiles, returning `filepath.SkipDir` prevents `filepath.WalkDir` from descending into it. Without this, a 500MB `node_modules/` directory would be walked entry by entry before being excluded.

**Context:** Performance optimization that must be explicit. The standard `filepath.WalkDir` callback pattern does not skip directories automatically even when they are excluded.
**Source:** 03-02-PLAN.md, 03-02-SUMMARY.md

---

## Patterns

### Three-level directory pattern matching in ShouldExclude
For patterns ending in "/" (directory patterns): check (1) exact match — `relPath == strings.TrimSuffix(pattern, "/")`, (2) prefix match — `strings.HasPrefix(relPath, pattern)`, (3) path component scan — any segment of relPath equals the directory name. This handles `.git/config` (prefix), `deep/node_modules/pkg/index.js` (component), and bare `.git` (exact).

**When to use:** File filters that need to exclude entire directory trees at any depth, not just top-level directories.
**Source:** 03-02-PLAN.md, 03-02-SUMMARY.md

---

### Atomic file deployment via /tmp staging and SSH mv
1. Stage all files in `/tmp/docker-deploy-<timestamp>` via SFTP
2. Run `mkdir -p remoteBase` (+ sudo if needed, + chown + chmod)
3. First deploy: `rm -rf remoteBase`, `mv stagingDir remoteBase`
4. Repeat deploy: `mv remoteBase remoteBase-old-<ts>`, `mv stagingDir remoteBase`, `rm -rf remoteBase-old-<ts>`
5. On step 3/4 failure: rollback by restoring from backup

**When to use:** Any remote deployment where the target must never be left in a partial state. /tmp staging decouples the upload permission model from the target permission model.
**Source:** 03-02-PLAN.md, 03-04-PLAN.md, 03-05-PLAN.md

---

### ShellQuote: wrap in single quotes and escape embedded single quotes
```go
func ShellQuote(s string) string {
    return "'" + strings.ReplaceAll(s, "'", "'\\''") + "'"
}
```
Export the function so it can be used consistently wherever paths are interpolated into SSH exec commands.

**When to use:** Any SSH exec command that includes user-supplied or config-derived path strings. Prevents shell injection when paths contain special characters.
**Source:** 03-04-PLAN.md, 03-04-SUMMARY.md

---

### Remote path operations use path (not filepath) package
All remote path construction uses `path.Join`, `path.Dir`, `path.Base` from the standard `path` package (not `filepath`). This ensures forward-slash separators regardless of the local OS.

**When to use:** Any code that constructs paths for execution on a Linux remote, regardless of whether the local machine is macOS, Windows, or Linux.
**Source:** 03-02-SUMMARY.md

---

### Exclude merge: built-ins + file + flag, dedup by insertion-order map
```go
func mergeExcludes(flagExcludes []string, fileExcludes []string) []string {
    seen := make(map[string]bool)
    result := make([]string, 0)
    for _, e := range append(append(defaultExcludes, fileExcludes...), flagExcludes...) {
        if !seen[e] {
            seen[e] = true
            result = append(result, e)
        }
    }
    return result
}
```
Built-ins always win on ordering; user entries extend; duplicates are silently dropped.

**When to use:** Any exclude/ignore list configuration that has mandatory defaults plus user extension, where the defaults must always be present.
**Source:** 03-01-PLAN.md, 03-01-SUMMARY.md

---

### Force precedence: flagForce || file.Target.Force (boolean OR, no switch needed)
```go
cfg.Force = flagForce || file.Target.Force
```
Either the CLI flag or the YAML field can enable force mode; neither can disable it once the other sets it.

**When to use:** Boolean options where the flag should be sticky regardless of file config — "once set, always on" within a single invocation.
**Source:** 03-01-PLAN.md, 03-01-SUMMARY.md

---

## Surprises

### First deploy nesting bug: staging directory appears inside target instead of becoming it
After Phase 3 Plan 03 human checkpoint was approved, UAT revealed that files were landing at `/opt/test-deploy/docker-deploy-<ts>/` instead of `/opt/test-deploy/`. The root cause was `mkdir -p remoteBase` in step 8 creating an empty target directory before the mv, causing Unix `mv` to nest stagingDir inside it.

**Impact:** Required a dedicated gap-closure plan (03-05) with a TDD regression test. The fix was a single `rm -rf remoteBase` inserted before the mv in the first-deploy else branch. The bug was latent in all Phase 3 plans and only surfaced via real-host testing.
**Source:** 03-HUMAN-UAT.md, 03-05-PLAN.md, 03-05-SUMMARY.md

---

### Executable script permissions are lost on SFTP upload
Files uploaded via `sftpClient.OpenFile(remotePath, os.O_WRONLY|os.O_CREATE|os.O_TRUNC)` do not preserve the source file's permission bits. Executable scripts (e.g., entrypoint.sh with +x) lose the execute bit on the remote.

**Impact:** Known limitation (WR-01 from code review). Entrypoint scripts may fail on the remote if they need to be executable. Not fixed in Phase 3 — identified as a future gap-closure candidate.
**Source:** 03-VERIFICATION.md, 03-HUMAN-UAT.md

---

### Sudo password can appear in cleartext in SSH exec error messages
When a `sudoRun` command fails, the error returned by `sshExec` may include the full command string with the sudo password embedded as part of the `echo <pw> | sudo -S` pattern.

**Impact:** If the error propagates to stderr or logs, the password is visible. Identified as CR-01 in code review, not fixed in Phase 3. Future mitigation: sanitize error messages to redact the password, or use a different sudo auth mechanism.
**Source:** 03-VERIFICATION.md

---

### Four-step atomic swap reverted to three steps to avoid pre-move sudo ordering issue
Plan 03-04 introduced a four-step swap (staging→new in /opt, remoteBase→old, new→remoteBase, rm old) to enable rollback at every step. During human verification, this was reverted to three steps because the `/tmp → /opt` pre-move (step 10.1) required sudo before the password had been collected by the mkdir step.

**Impact:** The final atomic swap is three steps (oldDir backup, stagingDir rename to remoteBase, rm old). Rollback is still present for the second step (restoring from oldDir on step-3 failure). The four-step design was architecturally sound but required a different ordering to be practical.
**Source:** 03-04-SUMMARY.md
