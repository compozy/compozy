# BUG-20261009-subagent-crash-no-wake: After a daemon crash the subagent fails silently and the parent is never told

- **Status:** verified
- **Impact (user-side):** Trust-Damage
- **Severity:** Medium · **Priority:** P2
- **Persona Affected:** Dora
- **Journey Step:** see scenario
- **Scenarios:** RT-subagent-restart
- **Found:** 2026-10-09 · **Report:** docs/qa/reports/2026-10-09-subagents.md

## Summary

After `kill -9` mid child turn, boot reconciles the row to `failed` (`daemon crashed while session active`), delivery `disposed`, logs `subagent.recovered{reason=child_reconciled}`; every session is `stopped/failed`, so nothing recovers or wakes.

## Reproduction

- **Environment:** isolated lab (daemon from branch `sa-qa` @ c17af94ec), real Claude (claude-agent-acp, Sonnet 5.5) parent and Codex (gpt-5.6-sol) children, headless Chromium 1440×900 @2x, light theme.

1. Delegate a long Codex task.
2. `kill -9` the daemon; start it again.

**Expected:** Scenario step 3: the child recovers through ordinary session recovery, then the row finalizes and wakes the parent once.
**Actual:** Row failed + disposed; no session recovery happens after a crash in this build.

## Evidence

- Needs a decision: either the scenario/spec assumes a recovery behavior this product does not have, or crash recovery should keep the row `pending` delivery so the parent is woken when it is resumed. Filed as blocked-decision input.
- Report: `docs/qa/reports/2026-10-09-subagents.md`

## Retest 2026-10-09

Fixed in 5b33ec550 (controller decision). Re-walked on the stock build at 980d51fbe: kill -9: row `failed` (`daemon crashed while session active`), delivery `claimed` (not disposed); the crashed parent is read-only (`session not attachable … dead runtime`), as decided. Report: `docs/qa/reports/2026-10-09-subagents-r2.md`.
