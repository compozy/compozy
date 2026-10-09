# BUG-20261009-subagent-running-count-settled: “N agents running” counts subagents that already finished

- **Status:** verified
- **Impact (user-side):** Trust-Damage
- **Severity:** Medium · **Priority:** P2
- **Persona Affected:** Bruno
- **Journey Step:** see scenario
- **Scenarios:** ET-web-subagent-card
- **Found:** 2026-10-09 · **Report:** docs/qa/reports/2026-10-09-subagents.md

## Summary

After all three subagents completed, the status line and inspector Activity still read `3 agents running`; with one live it also read 3.

## Reproduction

- **Environment:** isolated lab (daemon from branch `sa-qa` @ c17af94ec), real Claude (claude-agent-acp, Sonnet 5.5) parent and Codex (gpt-5.6-sol) children, headless Chromium 1440×900 @2x, light theme.

1. Delegate three subagents; let them settle.
2. Read the composer status line and inspector Activity during the wake turn.

**Expected:** Only live subagents count (S7).
**Actual:** `3 agents running` while `subagent_summary.live` is 0.

## Evidence

- `session.supervision.work_signals` keeps one `active_child` per settled subagent child session (children stay `active`/`idle`), and `runningAgentCount` (`web/src/systems/session/lib/session-working-status.ts:128`) unions them with live roster rows. Evidence: `/Users/pedronauck/Dev/compozy/_worktrees/subagents/.compozy/tasks/subagents/orchestration/screens/pr/04-hover-card.png`, `08-inspector-roster.png`.
- Report: `docs/qa/reports/2026-10-09-subagents.md`

## Retest 2026-10-09

Fixed in 1ca1f1a38. Re-walked on the stock build at 980d51fbe: The status line dropped to `Working for …` with no count once both children settled mid-turn; the inspector Activity matched. Report: `docs/qa/reports/2026-10-09-subagents-r2.md`.
