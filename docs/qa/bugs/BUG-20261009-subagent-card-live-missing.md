# BUG-20261009-subagent-card-live-missing: Cards appear only after the delegating turn ends

- **Status:** open
- **Impact (user-side):** Friction
- **Severity:** Medium · **Priority:** P2
- **Persona Affected:** Bruno
- **Journey Step:** see scenario
- **Scenarios:** ET-web-subagent-card
- **Found:** 2026-10-09 · **Report:** docs/qa/reports/2026-10-09-subagents.md

## Summary

While the parent's delegating turn is still running, the transcript shows `Running compozy__subagent_delegate` / `Delegated a subagent 2 times` tool rows; the card or group replaces them only once the turn closes or the page reloads.

## Reproduction

- **Environment:** isolated lab (daemon from branch `sa-qa` @ c17af94ec), real Claude (claude-agent-acp, Sonnet 5.5) parent and Codex (gpt-5.6-sol) children, headless Chromium 1440×900 @2x, light theme.

1. On the patched build, send a prompt that delegates and keeps working.
2. Watch the transcript without reloading during the turn.

**Expected:** The card replaces the delegate row at the delegation point as soon as the delegate result lands (`_uiux.md` S1).
**Actual:** Tool rows during the turn; the card appears after the turn ends.

## Evidence

- Lab evidence: `qa-artifacts/qa/shots/10-single-running.png` and the mid-turn group capture. The live stream path seems not to carry the `data-compozy-subagent` part that the persisted projection emits.
- Report: `docs/qa/reports/2026-10-09-subagents.md`
