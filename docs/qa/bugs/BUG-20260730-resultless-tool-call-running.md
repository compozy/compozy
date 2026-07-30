# BUG-20260730-resultless-tool-call-running: Settled tool calls remained visibly running

- **Status:** fixed
- **Impact (user-side):** Trust-Damage
- **Severity:** High · **Priority:** P1
- **Persona Affected:** Théo
- **Journey Step:** J-11 return to a session, step 3
- **Scenarios:** RT-018; RT-043; RT-045
- **Found:** 2026-07-30 · **Report:** docs/qa/reports/2026-07-28-untested-full.md

## Summary

Théo could return to a settled turn whose tool call had no explicit result and still see an indefinite running state.

## Reproduction

- **Charter:** CH-session-terminal-state-truth · **Tour:** Switchback Tour
- **Environment:** isolated current-source daemon / Web / en-US

1. Settle a turn after a tool call without a result event.
2. Reload the transcript and inspect the tool row.

**Expected:** The server emits a terminal failed part and the Web fallback renders failure after turn settlement.
**Actual:** Both paths treated result absence as proof that work was still running.

## Evidence

- `/Users/pedronauck/dev/qa-labs/compozy-qa-rt-current-source-20260730-20260730-061631-252740-lab/qa-artifacts/qa`

## Fix

- **Root cause:** Terminalization depended only on a tool-result event instead of the enclosing turn's settled state.
- **Fix commit:** bd0617c; 9904270
- **Regression test:** `internal/api/core/prompt_stream_test.go`; `web/src/systems/session/components/__tests__/tool-call-card.test.tsx`
