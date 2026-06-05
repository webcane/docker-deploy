# Phase 3: File Copy - Context

**Gathered:** 2026-05-14
**Status:** Ready for planning

<domain>
## Phase Boundary

Local project files are transferred to the remote host via SFTP (wrapping the
existing `*ssh.Client` from Phase 2). Files are staged atomically under
`/opt/<project>/.deploy-tmp-<timestamp>` and then moved into place as the
target directory. Smart exclude defaults apply when no `exclude:` list is
configured; users can extend the exclude list via deploy.yaml or `--exclude`.
No compose execution — that belongs to Phase 4.

</domain>

<decisions>
## Implementation Decisions

### File Filter Model

- **D-01:** Copy-everything-minus-excludes model. All files in the current
  working directory are uploaded by default. There is no `include:` field —
  the exclude list is the only filter control. This replaces the "default
  include list" interpretation of FILES-01: the default behavior is broad
  (everything), not a curated whitelist.
- **D-02:** Built-in default excludes (always active, cannot be removed):
  `.git/`, `node_modules/`, `vendor/`, `*.log`, `.DS_Store`, `__pycache__/`
- **D-03:** User-defined `exclude:` in deploy.yaml EXTENDS the built-in list
  (never replaces it). You cannot accidentally copy `.git/` by omitting it
  from your custom list.
- **D-04:** `--exclude` CLI flag also supported. Follows the established
  precedence: `--flag > deploy.yaml > built-in defaults`. Flag-supplied
  excludes extend the combined (built-in + deploy.yaml) list.
- **D-05:** Glob patterns supported in exclude entries (e.g., `*.tmp`,
  `**/*.bak`). Built-in defaults already use `*.log` — the implementation
  must handle glob matching consistently.
- **D-06:** `.gitignore` is NOT read. The tool uses its own exclude list only.
  `.env` is always copied (it is not in the built-in excludes — this is
  intentional and is the core value proposition of the tool).

### Project Name Resolution

- **D-07:** Project name is `filepath.Base(cwd)` — the basename of the
  current working directory at deploy time. This determines both the default
  remote path (`/opt/<project>`) and the staging directory name
  (`.deploy-tmp-<timestamp>`). No compose.yaml parsing required.

### Repeat Deploy Strategy

- **D-08:** Clean-slate behavior: the entire target directory is wiped and
  replaced by the staged upload on each deploy. This maximizes reproducibility
  and avoids stale file accumulation.
- **D-09:** When the target directory already exists on the remote, the plugin
  prompts for confirmation: `"Target /opt/<project> exists on <host>. Replace
  all contents? [y/N]"` before proceeding. First deploys (target absent)
  proceed silently.
- **D-10:** `--force` CLI flag skips the confirmation prompt (allows scripted /
  non-interactive deploys).
- **D-11:** `force: true` under `target:` in deploy.yaml permanently skips the
  confirmation prompt — equivalent to always passing `--force`.

### deploy.yaml Schema Extension

- **D-12:** File config fields (`exclude:`, `force:`) live under `target:`
  alongside `host:` and `path:` (consistent with Phase 2 D-10: forward-
  compatible with future named-target nesting). Example schema:

  ```yaml
  version: 1
  target:
    host: ssh://user@host:port
    path: /opt/myapp       # optional — defaults to /opt/<cwd-basename>
    force: true            # optional — skip replace-confirmation
    exclude:               # optional — extends built-in defaults
      - "*.tmp"
      - logs/
  ```

### Claude's Discretion

- Exact output format during upload (per-file log vs. summary count vs.
  progress bar) — open to standard CLI conventions.
- SSH `mv`/`rename` strategy for the atomic directory swap (e.g., rename
  old target to a side-backup then rename staging dir, then remove backup) —
  implementation detail for the planner.
- Whether to show a file count before prompting on repeat deploy (e.g.,
  `"N files to upload"`) — planner's call.

</decisions>

<canonical_refs>
## Canonical References

**Downstream agents MUST read these before planning or implementing.**

### Project Decisions & Requirements
- `.planning/REQUIREMENTS.md` — DEPLOY-02, DEPLOY-03, FILES-01, FILES-02,
  FILES-03 are the requirements for this phase. Note: FILES-01's "default
  include list" is superseded by D-01 (exclude-only model) — the phase
  implements "copy everything minus excludes", not a curated whitelist.
- `.planning/PROJECT.md` §Key Decisions — locked decisions including the
  SFTP staging-dir pattern and file copy semantics.
- `CLAUDE.md` — Critical Implementation Rules, especially:
  - Rule 3: Atomic file copy (stage to `.deploy-tmp-<timestamp>`, move
    atomically; never leave partial state)
  - Rule 1: No InsecureIgnoreHostKey (still applies — reusing Phase 2 client)

### SSH / SFTP Implementation
- `go.mod` — current dependency versions; `github.com/pkg/sftp` must be
  added here before any SFTP code is written.
- `internal/ssh/client.go` — `Dial()` returns `*gossh.Client`; SFTP wraps
  this client (no second TCP connection).

### Config Integration
- `internal/config/config.go` — `Config` struct (currently `Host`, `Path`,
  `DryRun`) must be extended with `Excludes []string` and `Force bool`.
  `FileConfig.TargetConfig` must be extended with `Exclude []string` and
  `Force bool`.
- `internal/config/config_test.go` — existing tests; new fields must not
  break existing test cases.

### Existing Code
- `cmd/docker-deploy/main.go` — entry point; `--exclude` (repeatable flag)
  and `--force` flags are wired into the cobra root command here.

</canonical_refs>

<code_context>
## Existing Code Insights

### Reusable Assets
- `internal/ssh/client.go` — `Dial(ctx, DialConfig)` returns
  `*gossh.Client`. Phase 3 wraps this with `github.com/pkg/sftp` to open an
  SFTP session — no new TCP dial needed.
- `internal/config/config.go` — `Resolve()`, `LoadFile()`, `ParseHost()` are
  all reusable. Phase 3 extends `Config` and `TargetConfig` structs; the
  `Resolve()` function gains `Excludes` and `Force` outputs.

### Established Patterns
- Three-tier config precedence (`--flag > deploy.yaml > defaults`) is
  established and must be followed for `--exclude`/`force` exactly as it is
  for `--host`/`--path`.
- `TargetConfig` struct in `config.go` uses yaml struct tags — new fields
  must follow the same pattern (`yaml:"exclude"`, `yaml:"force"`).
- Tests in `internal/config/config_test.go` follow table-driven style —
  new Resolve() tests should follow the same pattern.

### Integration Points
- `cmd/docker-deploy/main.go` cobra `RunE` is where the deploy sequence
  runs: config resolve → SSH dial → (Phase 3) SFTP upload → (Phase 4+)
  compose exec. Phase 3 inserts the SFTP upload step between dial and
  compose.
- `go.mod` must have `github.com/pkg/sftp` added before any SFTP code
  compiles.

</code_context>

<specifics>
## Specific Ideas

- `.env` is always copied regardless of `.gitignore` — this is the core value
  proposition. Never add `.env` to the built-in default excludes.
- `force: true` in deploy.yaml is explicitly requested as a way to make
  non-interactive deploys easy without having to always pass `--force` on
  the CLI.
- The replace confirmation prompt should default to No (`[y/N]`) — destructive
  operations should require an affirmative choice, not accidentally trigger on
  Enter.

</specifics>

<deferred>
## Deferred Ideas

- `.gitignore` integration (use .gitignore as base excludes) — explored and
  rejected due to `.env` conflict. Noted for future consideration only if
  users request it specifically.
- `include:` whitelist mode — explored and rejected in favor of exclude-only
  model. Could be added in v2 if users managing monorepos need it.

</deferred>

---

*Phase: 3-File Copy*
*Context gathered: 2026-05-14*
