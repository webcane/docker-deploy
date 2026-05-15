# Phase 4: Core Deploy Loop - Discussion Log

> **Audit trail only.** Do not use as input to planning, research, or execution agents.
> Decisions are captured in CONTEXT.md — this log preserves the alternatives considered.

**Date:** 2026-05-15
**Phase:** 4-core-deploy-loop
**Areas discussed:** Output streaming, Compose command flags, Command invocation pattern, Compose failure UX

---

## Output Streaming

| Option | Description | Selected |
|--------|-------------|----------|
| Merged to stdout | Both stdout/stderr forwarded to os.Stdout | |
| Separate stdout/stderr | Compose stdout → os.Stdout, stderr → os.Stderr | |
| PTY allocation | RequestPty() for colors and progress | |

**User's choice:** Freeform — "if local terminal is TTY, then use colors and progress as usual. if stdout is no TTY — then no colors. progress is simplified. by default stdout and stderr are separated"

**Notes:** PTY when TTY (inherit local terminal size). In PTY mode, streams merge (accepted as PTY limitation, both go to os.Stdout). In non-TTY mode: separate stdout/stderr pipes. TTY detection via `term.IsTerminal`.

---

| Option | Description | Selected |
|--------|-------------|----------|
| Inherit local terminal size | Read columns × rows from os.Stdout, pass to RequestPty | ✓ |
| Fixed size (80×24) | Hardcoded columns | |
| You decide | Leave to planner | |

**User's choice:** Inherit local terminal size

---

| Option | Description | Selected |
|--------|-------------|----------|
| Accept merged in PTY mode | Both streams go to os.Stdout in PTY mode | ✓ |
| Capture stderr pre-PTY | Not actually possible with PTY | |
| You decide | Leave to planner | |

**User's choice:** Accept merged in PTY mode

---

## Compose Command Flags

| Option | Description | Selected |
|--------|-------------|----------|
| Yes, always (--remove-orphans) | Always remove orphan containers on deploy | ✓ |
| No, never | User controls container lifecycle | |
| Configurable | Default off, opt-in via deploy.yaml/flag | |

**User's choice:** Yes, always — `--remove-orphans` is always included

---

| Option | Description | Selected |
|--------|-------------|----------|
| No pull by default | Use cached images on remote | ✓ |
| Always pull (--pull always) | Always pull latest before starting | |
| Pull only on missing | Same as compose default behavior | |

**User's choice:** No pull by default

---

## Command Invocation Pattern

| Option | Description | Selected |
|--------|-------------|----------|
| cd <path> && docker compose up -d | Auto-discovers compose file in working directory | |
| docker compose -f <path>/compose.yaml up -d | Explicit file path | ✓ |
| You decide | Leave to planner | |

**User's choice:** `docker compose -f <path>/compose.yaml up -d`

---

| Option | Description | Selected |
|--------|-------------|----------|
| Detect with prior SSH command | Extra round-trip, always correct | |
| Use local filename | Check locally, no extra round-trip | |
| Try compose.yaml, fall back to docker-compose.yml | Two potential round-trips on fallback | |

**User's choice (interrupted):** User clarified mid-question: compose file is a required input. Resolution order: `--compose-file` flag → `compose_file` in deploy.yaml → local auto-detect (compose.yaml then docker-compose.yml). If none found → fail with clear error.

---

| Option | Description | Selected |
|--------|-------------|----------|
| --compose-file / compose_file | Mirrors docker compose -f convention | ✓ |
| --file / file | Shorter but ambiguous | |
| You decide | Leave to planner | |

**User's choice:** `--compose-file` / `compose_file`

---

## Compose Failure UX

| Option | Description | Selected |
|--------|-------------|----------|
| Compose output already streamed + error summary | Terse error line on failure; output already visible | ✓ (modified) |
| Re-print captured output on failure | Buffer silently, print on failure | |
| You decide | Leave to planner | |

**User's choice:** Option 1 + show detailed error message when `--verbose` is set (Phase 7). Error summary on stderr.

---

| Option | Description | Selected |
|--------|-------------|----------|
| stderr | Error messages on stderr | ✓ |
| stdout | Everything on stdout | |
| You decide | Leave to planner | |

**User's choice:** stderr

---

## Claude's Discretion

- PTY terminal type string: `"xterm-256color"`
- Session execution model: `session.Start()` + goroutine drains + `session.Wait()` (not `session.Run()`)
- Exit code extraction: `(*gossh.ExitError).ExitStatus()`

## Deferred Ideas

- `--pull always` / image pull control → Phase 7 (v2 leftovers)
- `--verbose` flag implementation → Phase 7 (but failure UX must accommodate the extension structurally)
- `--wait` flag (compose 2.1+) → Phase 5 discussion (health polling domain)
