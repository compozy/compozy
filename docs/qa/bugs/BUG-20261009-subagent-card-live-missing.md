# BUG-20261009-subagent-card-live-missing: Cards appear only after the delegating turn ends

- **Status:** verified
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

## Re-found 2026-10-09 (retest after 776aacc76)

Partly fixed. With the prompt sent from the CLI, the group card appeared mid-turn as soon as the delegation
landed. With the prompt sent from the open session window's composer, the turn still showed
`Delegated a subagent 2 times, used 2 tools` with both rows `running` (36 s after the delegation in one run, about
40 s in another), and the group card replaced it later in the same turn. The persisted transcript and the stream
snapshot already carried both `data-compozy-subagent` parts at that moment, so the gap is in how the composer-submitted
live turn appends parts. Report: `docs/qa/reports/2026-10-09-subagents-r2.md`.

## Retest 2026-10-09 round 3

Fixed in f4a2d1918. Re-walked on the stock build at 1d7f690e5 with a real Claude parent and Codex children: Prompt sent from the open window's composer: the group card `2 subagents · 2 working` appeared within 5 s of delegation while the parent turn was still running (`2 agents running`), and followed the children to `2 done`. Report: `docs/qa/reports/2026-10-09-subagents-r2.md`.
