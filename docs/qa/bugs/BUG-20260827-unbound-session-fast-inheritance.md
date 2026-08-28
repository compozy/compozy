# BUG-20260827-unbound-session-fast-inheritance: First prompt drops inherited Fast mode

- **Status:** fixed, pending remediation commit
- **Impact (user-side):** Trust-Damage
- **Severity:** Major · **Priority:** P1
- **Persona Affected:** Dora
- **Journey Step:** J-17 Send the first prompt with the agent's runtime defaults
- **Scenarios:** MS-web-session-simple-advanced-launch; ET-web-session-prompt-runtime-and-create-navigation; RT-063; RT-cursor-logical-runtime-options
- **Found:** 2026-08-27 · **Report:** docs/qa/reports/2026-08-27-acp-runtime-catalog.md

## Summary

The agent and immutable creation profile both selected Fast, but the Web initialized an unbound session's prompt speed as Normal. The first prompt therefore replaced the inherited value with Normal.

## Reproduction

1. Create an agent with Cursor Grok 4.5, High reasoning, and Fast.
2. Start a durable session from that agent.
3. Inspect the composer runtime and send the first prompt.

**Expected:** the unbound composer and the first bound runtime both retain Fast.
**Actual:** the agent overview said Fast, while the composer and persisted effective runtime said Normal.

## Fix

- **Root cause:** the Web correctly fell back from an absent unbound `runtime.effective` to the agent for provider, model, reasoning, and ACP options, but speed had a separate fallback hardcoded to Normal.
- **Fix:** derive unbound speed from the matching agent's effective runtime and switch to daemon-effective speed after binding.
- **Fix commit:** pending; included in the single remediation commit
- **Regression test:** `use-session-prompt-runtime.test.tsx` — `Should inherit the agent speed before the first runtime bind`.

## Verification

- **Focused automated result:** the canonical hook suite passes through Turborepo.
- **Browser result:** a fresh composer displayed Fast before dispatch, answered `QA_RUNTIME_FAST_OK`, and still displayed Fast after bind.
- **Daemon result:** CLI readback for `sess-d16d8d05ef89ffd9` reported `grok-4.5`, `high`, `fast`, and ready `initial_bind`.
- **Evidence:** `/Users/pedronauck/dev/qa-labs/compozy-acp-runtime-catalog-20260828-004625-083662-lab/qa-artifacts/qa/evidence/web-session-first-prompt-grok45-fast-pass.png`.
