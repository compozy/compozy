# BUG-20261009-subagent-daemon-stop-cancels: A clean daemon stop cancels running subagents

- **Status:** open
- **Impact (user-side):** Data-Loss
- **Severity:** High · **Priority:** P1
- **Persona Affected:** Dora
- **Journey Step:** see scenario
- **Scenarios:** RT-subagent-restart
- **Found:** 2026-10-09 · **Report:** docs/qa/reports/2026-10-09-subagents.md

## Summary

Dora stops the daemon while a delegated Codex child is working. After start, the subagent is `canceled`, delivery `disposed`, error `session canceled by user`, and the parent is never woken.

## Reproduction

- **Environment:** isolated lab (daemon from branch `sa-qa` @ c17af94ec), real Claude (claude-agent-acp, Sonnet 5.5) parent and Codex (gpt-5.6-sol) children, headless Chromium 1440×900 @2x, light theme.

1. Delegate a long Codex task (async).
2. `compozy daemon stop`, then `compozy daemon start`.
3. `compozy session subagents <parent>`.

**Expected:** The row stays live for boot recovery and wakes the parent once when the child settles (ADR-002, Recover).
**Actual:** `canceled` / `disposed` at shutdown time (sub-a575bbacf7056314, settled 09:30:41Z during shutdown).

## Evidence

- The stop path routes the child's shutdown stop through the subagent cancel classification. Parent sessions themselves stay `active` across the clean restart.
- Report: `docs/qa/reports/2026-10-09-subagents.md`
