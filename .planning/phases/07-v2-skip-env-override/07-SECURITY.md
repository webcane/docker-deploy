---
phase: 07
slug: 07-v2-skip-env-override
status: verified
threats_open: 0
asvs_level: 1
created: 2026-05-23
---

# Phase 07 — Security

> Per-phase security contract: threat register, accepted risks, and audit trail.

---

## Trust Boundaries

| Boundary | Description | Data Crossing |
|----------|-------------|---------------|
| deploy.yaml → Resolve() | User-controlled config file; `skip_env: true` can suppress .env upload silently | SkipEnv bool — controls whether .env is excluded from transfer |
| operator → --skip-env flag | Operator intentionally suppresses .env upload; remote secrets silently diverge from local | Boolean flag, no credential data |
| operator → --verbose flag | Detailed output (file names, SSH commands) written to stderr; if stderr is logged, file paths are exposed | Remote file paths, SSH command strings |
| warning rollup → operator | Non-verbose mode hides individual warnings; operator may miss actionable issues | Warning messages |
| --verbose → SSH command log | SSH command strings (including remote paths) are printed to stderr; interactive sudo command is redacted | Remote paths, command strings (sudo password redacted) |

---

## Threat Register

| Threat ID | Category | Component | Disposition | Mitigation | Status |
|-----------|----------|-----------|-------------|------------|--------|
| T-07-01-01 | Information Disclosure | skip_env / .env exclude | mitigate | `WARNING: .env not uploaded — remote .env left unchanged` printed to stderr whenever SkipEnv=true; surfaces inline with --verbose, via rollup without | closed |
| T-07-01-02 | Tampering | defaultExcludes expansion | accept | New entries are dev-tooling directories only; no production artifact inadvertently excluded; .env always uploaded by default | closed |
| T-07-01-03 | Information Disclosure | FlagOpts struct | accept | Struct holds flag values in process memory only; no persistence; no new attack surface beyond existing positional params | closed |
| T-07-02-01 | Information Disclosure | --skip-env | mitigate | Warning always printed to stderr (inline or via rollup); `main.go:246` confirmed present | closed |
| T-07-02-02 | Repudiation | warning rollup | mitigate | `WARN: there are some warnings during deployment. For more details use --verbose flag` printed when warnings > 0 and --verbose not set; `main.go:352` confirmed present | closed |
| T-07-02-03 | Information Disclosure | --verbose stderr output | accept | Per-file paths and SSH commands written to operator-controlled stderr; acceptable for a deploy tool used by the operator themselves | closed |
| T-07-02-04 | Elevation of Privilege | Upload() verbose param | accept | verbose param is a read-only behavior switch; does not change SSH commands, paths, or auth logic; no privilege escalation risk | closed |
| T-07-02-05 | Information Disclosure | --verbose sudo command log | mitigate | Interactive sudo commands contain literal sudo password; `upload.go:269` prints `[ssh] (sudo password cmd redacted)` instead of full command string when verbose=true | closed |

*Status: open · closed*
*Disposition: mitigate (implementation required) · accept (documented risk) · transfer (third-party)*

---

## Accepted Risks Log

| Risk ID | Threat Ref | Rationale | Accepted By | Date |
|---------|------------|-----------|-------------|------|
| AR-07-01 | T-07-01-02 | Expanded defaultExcludes contains dev-tooling dirs only (.github/, .claude/, .planning/, .idea/, .vscode/, .terraform/, dist/, coverage/, *.swp, *.swo). No production artifact is inadvertently excluded. The tool's core value proposition (.env always uploaded by default unless --skip-env) is preserved. | plan author | 2026-05-20 |
| AR-07-02 | T-07-01-03 | FlagOpts struct holds flag values in process memory only (Host, Path, Excludes, Force, ComposeFile, HealthTimeout, HealthInterval, SkipEnv, Verbose). No persistence to disk, no new network exposure beyond existing flag handling. | plan author | 2026-05-20 |
| AR-07-03 | T-07-02-03 | Per-file paths and SSH command strings written to stderr are operator-controlled. For a CLI deploy tool used by the operator themselves, stderr capture is intentional and expected. No third-party data exposure. | plan author | 2026-05-20 |
| AR-07-04 | T-07-02-04 | The verbose bool parameter added to Upload() and RunCompose() is a read-only behavior switch that controls logging output only. It does not alter SSH commands, authentication logic, remote paths, or file permissions. | plan author | 2026-05-20 |

---

## Security Audit Trail

| Audit Date | Threats Total | Closed | Open | Run By |
|------------|---------------|--------|------|--------|
| 2026-05-23 | 8 | 8 | 0 | gsd-secure-phase (plan-time register, short-circuit verified) |

---

## Sign-Off

- [x] All threats have a disposition (mitigate / accept / transfer)
- [x] Accepted risks documented in Accepted Risks Log
- [x] `threats_open: 0` confirmed
- [x] `status: verified` set in frontmatter

**Approval:** verified 2026-05-23
