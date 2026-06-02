---
phase: 2
slug: ssh-transport-config
status: verified
threats_open: 0
asvs_level: 1
created: 2026-06-02
---

# Phase 2 — Security

> Per-phase security contract: threat register, accepted risks, and audit trail.

---

## Trust Boundaries

| Boundary | Description | Data Crossing |
|----------|-------------|---------------|
| deploy.yaml → config.LoadFile | File content is user-controlled but local; must not panic on malformed YAML | YAML config (no secrets) |
| --host flag → ParseHost | User-supplied SSH URI; validated before use in SSH dial | hostname, port, user |
| network → ssh.Dial | Remote host sends public key; verified against known_hosts | SSH host public key |
| user Stdin → TOFU prompt | User types "yes"/"no"; must not auto-accept on empty or ambiguous input | user confirmation string |
| ~/.ssh/config → key loading | Local file may reference keys with passphrases; failed loads silently skipped | key paths (no key material) |
| ~/.ssh/known_hosts → write | Appending new host entries; O_APPEND to avoid truncation | host fingerprints |
| os.Getwd() → projectName | Working directory determines default remote path; user controls cwd | directory basename |
| --dry-run stdout | Prints resolved config including hostname to stdout; no secrets printed | host, path, auth method |

---

## Threat Register

| Threat ID | Category | Component | Disposition | Mitigation | Status |
|-----------|----------|-----------|-------------|------------|--------|
| T-02-01 | Tampering | deploy.yaml | accept | `yaml.Unmarshal` returns error on malformed YAML; no panic path — file is local and user-owned | closed |
| T-02-02 | Spoofing | ParseHost | mitigate | Rejects non-`ssh://` schemes (`u.Scheme != "ssh"` → error); requires non-empty hostname; defaults port to 22. See `internal/config/config.go:151` | closed |
| T-02-03 | Information Disclosure | Config.Path | accept | Path stored in plain config; no secrets; user-visible by design in `--dry-run` output (D-12) | closed |
| T-02-04 | Spoofing | HostKeyCallback | mitigate | `knownhosts.New()` enforces strict host key verification (`internal/ssh/knownhosts.go:66`); MITM triggers KeyMismatchError hard fail | closed |
| T-02-05 | Spoofing | InsecureIgnoreHostKey | mitigate | Zero production uses; test-only occurrences marked `//nolint — test-only`. See CLAUDE.md Rule 1. | closed |
| T-02-06 | Elevation of Privilege | SSH agent socket | accept | Agent socket is user-owned; plugin inherits user's agent access — appropriate privilege level | closed |
| T-02-07 | Denial of Service | Dial hang | mitigate | Goroutine + `time.After(timeout)` + `context.Done()` cancel in `Dial()` (`internal/ssh/client.go:122`); hard timeout per CLAUDE.md Rule 2 | closed |
| T-02-08 | Information Disclosure | TOFU fingerprint | accept | Fingerprint displayed to user before confirmation — intended security UX; no silent acceptance | closed |
| T-02-09 | Spoofing | TOFU auto-accept | mitigate | `strings.TrimSpace(answer) != "yes"` check at `internal/ssh/client.go:234,236`; empty or non-`"yes"` response rejects | closed |
| T-02-10 | Information Disclosure | --dry-run output | accept | Output shows host/path/auth method by design (D-12); no passwords or key material printed | closed |
| T-02-11 | Tampering | projectName from cwd | accept | cwd is user-controlled; `/opt/<cwd-basename>` is a reasonable default; user overrides with `--path` | closed |

*Status: open · closed*
*Disposition: mitigate (implementation required) · accept (documented risk) · transfer (third-party)*

---

## Accepted Risks Log

| Risk ID | Threat Ref | Rationale | Accepted By | Date |
|---------|------------|-----------|-------------|------|
| AR-02-01 | T-02-01 | deploy.yaml is a local, user-owned file. Malformed YAML returns an error — no panic path exists. Risk is equivalent to a user misconfiguring their own tool. | gsd-secure-phase workflow | 2026-06-02 |
| AR-02-03 | T-02-03 | Config.Path is displayed in `--dry-run` output by explicit design requirement D-12. No secrets or key material are stored in this field. | gsd-secure-phase workflow | 2026-06-02 |
| AR-02-06 | T-02-06 | SSH agent socket access inherits the user's own privilege level. The plugin running as the user is the intended access model; no privilege escalation occurs. | gsd-secure-phase workflow | 2026-06-02 |
| AR-02-08 | T-02-08 | TOFU fingerprint display is the intended security UX — the user sees the fingerprint before deciding to trust. Silent acceptance is explicitly prevented (T-02-09 mitigates the companion risk). | gsd-secure-phase workflow | 2026-06-02 |
| AR-02-10 | T-02-10 | `--dry-run` output intentionally reveals host/path/auth method per D-12. Passwords are never stored; private key material is never printed. Hostname exposure is expected in a deployment tool. | gsd-secure-phase workflow | 2026-06-02 |
| AR-02-11 | T-02-11 | Remote path derived from cwd basename is a convenience default the user explicitly controls via `--path`. No security boundary is crossed — the user chooses where to deploy. | gsd-secure-phase workflow | 2026-06-02 |

---

## Security Audit Trail

| Audit Date | Threats Total | Closed | Open | Run By |
|------------|---------------|--------|------|--------|
| 2026-06-02 | 11 | 11 | 0 | gsd-secure-phase (short-circuit: register_authored_at_plan_time=true, all threats CLOSED in summaries; critical mitigations spot-verified in code) |

---

## Sign-Off

- [x] All threats have a disposition (mitigate / accept / transfer)
- [x] Accepted risks documented in Accepted Risks Log
- [x] `threats_open: 0` confirmed
- [x] `status: verified` set in frontmatter
