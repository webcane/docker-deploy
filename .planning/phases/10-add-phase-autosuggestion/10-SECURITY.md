---
phase: 10
slug: add-phase-autosuggestion
status: verified
threats_open: 0
asvs_level: 1
created: 2026-06-02
---

# Phase 10 — Security

> Per-phase security contract: threat register, accepted risks, and audit trail.

---

## Trust Boundaries

| Boundary | Description | Data Crossing |
|----------|-------------|---------------|
| filesystem → ListHosts | Reads user-owned `~/.ssh/config`; no untrusted input crosses this boundary | SSH host aliases (user-controlled config) |
| user shell → completion func | Tab-press triggers cobra to call the registered completion function; no network I/O | Shell completion candidates (display-only) |
| filesystem → completion func | Reads user-owned `deploy.yaml` and `~/.ssh/config` at static-script generation time; both files are user-controlled | Completion script generation (build-time only) |

---

## Threat Register

| Threat ID | Category | Component | Disposition | Mitigation | Status |
|-----------|----------|-----------|-------------|------------|--------|
| T-10-01-01 | Spoofing | ListHosts parsing malicious `~/.ssh/config` | accept | File is user-owned; `bufio.Scanner` with no dynamic execution; `ListHosts` returns nil on error (`sshconfig.go:152-153`, `174-175`) | closed |
| T-10-01-02 | Tampering | Path traversal via `configPath` | accept | `configPath` is always `~/.ssh/config` constructed by the caller; no user input flows into the path at completion time; D-03 removed dynamic completion hooks entirely | closed |
| T-10-02-01 | Tampering | Shell injection via `--host` completion candidate from `deploy.yaml` | accept | Static pre-generated scripts only; completion candidates are printed to stdout as text, never executed; cobra handles display protocol; no dynamic `RegisterFlagCompletionFunc` (D-03) | closed |
| T-10-02-02 | Tampering | Path traversal via `--compose-file` completion | accept | Static scripts only; no runtime `os.Stat` calls on user-supplied paths; D-03 removed dynamic completion hooks | closed |
| T-10-02-03 | Spoofing | Malicious `~/.ssh/config` content causing panic in ListHosts | mitigate | `ListHosts` uses `bufio.Scanner`; returns nil on any open or scan error (`sshconfig.go:152-153`, `174-175`); D-03 eliminated dynamic `HostCompletionFunc` entirely — ListHosts is not invoked at runtime completion | closed |
| T-10-02-04 | Denial of Service | `completion` subcommand accepts unsupported shell value | mitigate | `cobra.MatchAll(cobra.ExactArgs(1), cobra.OnlyValidArgs)` + `ValidArgs: []string{"bash", "zsh"}` rejects unsupported shells before `RunE` fires (`main.go:168-169`) | closed |
| T-10-02-05 | Information Disclosure | `deploy.yaml` host value exposed in shell completion candidates | accept | Static scripts do not read `deploy.yaml` at completion time; file is user-owned; candidates shown only to the user who already has read access | closed |

*Status: open · closed*
*Disposition: mitigate (implementation required) · accept (documented risk) · transfer (third-party)*

---

## Accepted Risks Log

| Risk ID | Threat Ref | Rationale | Accepted By | Date |
|---------|------------|-----------|-------------|------|
| AR-10-01 | T-10-01-01 | `~/.ssh/config` is user-owned; bufio.Scanner parse-only (no execution); nil-on-error contract prevents panic propagation | plan-time decision | 2026-06-02 |
| AR-10-02 | T-10-01-02 | configPath is a fixed caller-constructed path; no user-controlled input reaches `os.Open`; D-03 further removed all dynamic hooks | plan-time decision | 2026-06-02 |
| AR-10-03 | T-10-02-01 | Completion candidates are display-only text written to stdout; cobra's completion protocol does not execute them | plan-time decision | 2026-06-02 |
| AR-10-04 | T-10-02-02 | Static scripts only; no runtime path operations on user input | plan-time decision | 2026-06-02 |
| AR-10-05 | T-10-02-05 | `deploy.yaml` is user-owned; completion candidates not exposed at runtime (static scripts) | plan-time decision | 2026-06-02 |

*Accepted risks do not resurface in future audit runs.*

---

## Security Audit Trail

| Audit Date | Threats Total | Closed | Open | Run By |
|------------|---------------|--------|------|--------|
| 2026-06-02 | 7 | 7 | 0 | gsd-secure-phase (automated) |

---

## Sign-Off

- [x] All threats have a disposition (mitigate / accept / transfer)
- [x] Accepted risks documented in Accepted Risks Log
- [x] `threats_open: 0` confirmed
- [x] `status: verified` set in frontmatter

**Approval:** verified 2026-06-02
