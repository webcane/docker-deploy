<purpose>
Verify threat mitigations for a completed phase. Confirm PLAN.md threat register dispositions are resolved. Update SECURITY.md.
</purpose>

<required_reading>
@/Users/mniedre/git/docker-deploy/.claude/get-shit-done/references/ui-brand.md
</required_reading>

<available_agent_types>
Valid GSD subagent types (use exact names — do not fall back to 'general-purpose'):
- gsd-security-auditor — Verifies threat mitigation coverage
</available_agent_types>

<process>

## 0. Initialize

```bash
INIT=$(gsd-sdk query init.phase-op "${PHASE_ARG}")
if [[ "$INIT" == @file:* ]]; then INIT=$(cat "${INIT#@file:}"); fi
AGENT_SKILLS_AUDITOR=$(gsd-sdk query agent-skills gsd-security-auditor)
```

Parse: `phase_dir`, `phase_number`, `phase_name`, `phase_slug`, `padded_phase`.

```bash
AUDITOR_MODEL=$(gsd-sdk query resolve-model gsd-security-auditor --raw)
SECURITY_CFG=$(gsd-sdk query config-get workflow.security_enforcement --raw 2>/dev/null || echo "true")
```

If `SECURITY_CFG` is `false`: exit with "Security enforcement disabled. Enable via /gsd-settings."

Display banner: `GSD > SECURE PHASE {N}: {name}`

## 1. Detect Input State

```bash
SECURITY_FILE=$(ls "${PHASE_DIR}"/*-SECURITY.md 2>/dev/null | head -1)
PLAN_FILES=$(ls "${PHASE_DIR}"/*-PLAN.md 2>/dev/null)
SUMMARY_FILES=$(ls "${PHASE_DIR}"/*-SUMMARY.md 2>/dev/null)
```

- **State A** (`SECURITY_FILE` non-empty): Audit existing
- **State B** (`SECURITY_FILE` empty, `PLAN_FILES` and `SUMMARY_FILES` non-empty): Run from artifacts
- **State C** (`SUMMARY_FILES` empty): Exit — "Phase {N} not executed. Run /gsd-execute-phase {N} first."

## 2. Discovery

### 2a. Read Phase Artifacts

Read PLAN.md — extract `<threat_model>` block: trust boundaries, STRIDE register (`threat_id`, `category`, `component`, `disposition`, `mitigation_plan`).

### 2b. Read Summary Threat Flags

Read SUMMARY.md — extract `## Threat Flags` entries.

### 2c. Build Threat Register

Per threat: `{ threat_id, category, component, disposition, mitigation_pattern, files_to_check }`

Also set `register_authored_at_plan_time: true` if **at least one** PLAN file contained a parseable `<threat_model>` block; `false` if no PLAN files had any `<threat_model>` block (legacy phase authored before formal threat modelling was standard).

## 3. Threat Classification

Classify each threat:

| Status | Criteria |
|--------|----------|
| CLOSED | mitigation found OR accepted risk documented in SECURITY.md OR transfer documented |
| OPEN | none of the above |

Build: `{ threat_id, category, component, disposition, status, evidence }`

**Short-circuit rule:**
- If `threats_open: 0 AND register_authored_at_plan_time: true` → skip to Step 6 directly. All plan-time threats are verified CLOSED.
- If `threats_open: 0 AND register_authored_at_plan_time: false` → **do NOT skip**. Empty-by-no-planning must not rubber-stamp a clean SECURITY.md. Proceed to Step 5 in **retroactive-STRIDE mode** — the auditor builds a register from implementation files first, then verifies mitigations.
- If `threats_open > 0` → proceed to Step 4 (present threat plan to user).

## 4. Present Threat Plan


**Text mode (`workflow.text_mode: true` in config or `--text` flag):** Set `TEXT_MODE=true` if `--text` is present in `$ARGUMENTS` OR `text_mode` from init JSON is `true`. When TEXT_MODE is active, replace every `AskUserQuestion` call with a plain-text numbered list and ask the user to type their choice number. This is required for non-Claude runtimes (OpenAI Codex, Gemini CLI, etc.) where `AskUserQuestion` is not available.
Call AskUserQuestion with threat table and options:
1. "Verify all open threats" → Step 5
2. "Accept all open — document in accepted risks log" → add to SECURITY.md accepted risks, set all CLOSED, Step 6
3. "Cancel" → exit

## 5. Spawn gsd-security-auditor

**Auditor constraint — varies by register origin:**

- `register_authored_at_plan_time: true` — **Verify mitigations exist** — do not scan for new threats. The register is complete; verify each threat's mitigation is present in the implementation.
- `register_authored_at_plan_time: false` (retroactive-STRIDE mode) — **Retroactive-STRIDE: build a STRIDE register from implementation files first, then verify mitigations.** The phase was authored before formal threat modelling; the auditor must construct the register from scratch before verifying.

```
Agent(
  prompt="Read /Users/mniedre/git/docker-deploy/.claude/agents/gsd-security-auditor.md for instructions.\n\n" +
    "<files_to_read>{PLAN, SUMMARY, impl files, SECURITY.md}</files_to_read>" +
    "<threat_register>{threat register}</threat_register>" +
    "<config>asvs_level: {SECURITY_ASVS}, block_on: {SECURITY_BLOCK_ON}</config>" +
    "<constraints>Never modify implementation files. Verify mitigations exist — do not scan for new threats. Escalate implementation gaps.</constraints>" +
    "${AGENT_SKILLS_AUDITOR}",
  subagent_type="gsd-security-auditor",
  model="{AUDITOR_MODEL}",
  description="Verify threat mitigations for Phase {N}"
)
```

> **ORCHESTRATOR RULE — CODEX RUNTIME**: After calling Agent() above, stop working on this task immediately. Do not read more files, edit code, or run tests related to this task while the subagent is active. Wait for the subagent to return its result. This prevents duplicate work, conflicting edits, and wasted context. Only resume when the subagent result is available.

Handle return:
- `## SECURED` → record closures → Step 6
- `## OPEN_THREATS` → record closed + open, present user with accept/block choice → Step 6
- `## ESCALATE` → present to user → Step 6

## 6. Write/Update SECURITY.md

**State B (create):**
1. Read template from `/Users/mniedre/git/docker-deploy/.claude/get-shit-done/templates/SECURITY.md`
2. Fill: frontmatter, threat register, accepted risks, audit trail
3. Write to `${PHASE_DIR}/${PADDED_PHASE}-SECURITY.md`

**State A (update):**
1. Update threat register statuses, append to audit trail:

```markdown
## Security Audit {date}
| Metric | Count |
|--------|-------|
| Threats found | {N} |
| Closed | {M} |
| Open | {K} |
```

**ENFORCING GATE:** If `threats_open > 0` after all options exhausted (user did not accept, not all verified closed):

```
GSD > PHASE {N} SECURITY BLOCKED
{K} threats open — phase advancement blocked until threats_open: 0
▶ Fix mitigations then re-run: /gsd-secure-phase {N}
▶ Or document accepted risks in SECURITY.md and re-run.
```

Do NOT emit next-phase routing. Stop here.

## 7. Commit

```bash
gsd-sdk query commit "docs(phase-${PHASE}): add/update security threat verification"
```

## 8. Results + Routing

**Secured (threats_open: 0):**
```
GSD > PHASE {N} THREAT-SECURE
threats_open: 0 — all threats have dispositions.
▶ /gsd-validate-phase {N}    validate test coverage
▶ /gsd-verify-work {N}       run UAT
```

Display `/clear` reminder.

</process>

<success_criteria>
- [ ] Security enforcement checked — exit if false
- [ ] Input state detected (A/B/C) — state C exits cleanly
- [ ] PLAN.md threat model parsed, register built
- [ ] SUMMARY.md threat flags incorporated
- [ ] threats_open: 0 AND register_authored_at_plan_time: true → skip directly to Step 6
- [ ] threats_open: 0 AND register_authored_at_plan_time: false → retroactive-STRIDE mode (Step 5), not skipped
- [ ] User gate with threat table presented
- [ ] Auditor spawned with complete context
- [ ] All three return formats (SECURED/OPEN_THREATS/ESCALATE) handled
- [ ] SECURITY.md created or updated
- [ ] threats_open > 0 BLOCKS advancement (no next-phase routing emitted)
- [ ] Results with routing presented on success
</success_criteria>
