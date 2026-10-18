# BUG-20261009-native-subagent-title-task: Claude's own subagent card is titled “Task” instead of its description

- **Status:** verified
- **Impact (user-side):** Cosmetic
- **Severity:** Low · **Priority:** P3
- **Persona Affected:** Rafa
- **Journey Step:** see scenario
- **Scenarios:** ET-web-native-subagent
- **Found:** 2026-10-09 · **Report:** docs/qa/reports/2026-10-09-subagents.md

## Summary

The native card and record read `Task` although the Agent call's description was `Survey BRIEF.md risks`.

## Reproduction

- **Environment:** isolated lab (daemon from branch `sa-qa` @ c17af94ec), real Claude (claude-agent-acp, Sonnet 5.5) parent and Codex (gpt-5.6-sol) children, headless Chromium 1440×900 @2x, light theme.

1. Ask Claude to use its built-in Agent tool with a description.
2. Inspect the card and `compozy session subagents <parent> --origin provider_native --json`.

**Expected:** Title = the Agent call's description.
**Actual:** `title: "Task"`.

## Evidence

- The first `tool_call` event carries title `Task`; the description arrives in a later update (same tool_call_id) and is not applied to the row or the projected part. Evidence: `/Users/pedronauck/Dev/compozy/_worktrees/subagents/.compozy/tasks/subagents/orchestration/screens/pr/09a-native-card.png`.
- Report: `docs/qa/reports/2026-10-09-subagents.md`

## Retest 2026-10-09

Fixed in e7e9b276a. Re-walked on the stock build at 980d51fbe: Native card and record titled `Survey BRIEF.md risks`. Report: `docs/qa/reports/2026-10-09-subagents-r2.md`.
