# BUG-20260929-rewind-draft-lost-on-row-unmount: Web rewind never restores the rewound prompt as a draft

- **Status:** fixed (retested on the rebuilt web)
- **Impact (user-side):** Trust-Damage
- **Severity:** Medium · **Priority:** P2
- **Persona Affected:** Théo
- **Journey Step:** J-rewind-conversation: "Rewind to here" on a durable user message in the session window
- **Scenarios:** RT-conversation-rewind; ET-web-session-fork-from-here (shared message-action row)
- **Found:** 2026-09-29 · **Report:** docs/qa/reports/2026-09-29-session-continue-fork-r1-rewalk.md

## Summary

Confirming "Rewind to here" cut the transcript (epoch 0 → 1, the daemon answered `200` with `draft_text: "Second step"`), but the composer stayed empty and no toast appeared. The rewound prompt was lost, so the operator has to retype it. The expected result is "restores the selected prompt as a draft".

## Root cause

`useSessionRewindMessageAction` (`web/src/systems/session/hooks/use-session-rewind-message-action.ts`) aborted its request controller in an unmount cleanup. The hook lives on the rewound message's row. The live transcript drops that row as soon as the daemon cuts it, and that happens before the POST response is read. The browser trace shows `rewind net::ERR_ABORTED` about 300 ms after the POST was sent, after the server had already applied it. Because the signal was aborted, `confirm` returned before `resetRuntime()` and `composerPrefill(draft_text)`. This predates the feature (the effect dates from #310). The web leg was first walked to a completed rewind here; the 2026-08-04 report only exercised cancellation.

## Fix

Only dismissing the dialog aborts the request now. The row-unmount abort was removed, so an accepted rewind still resets the runtime and prefills the draft through the thread-level providers, which stay mounted. Regression: `session-chat-runtime-provider.test.tsx` "Should restore the rewound prompt as the draft after its message leaves the transcript". The rewind response is held until the live read has dropped the row, and the composer must then hold the draft. This case failed before the fix with `expected '' to be 'Summarize the launch blockers…'`.

## Evidence

docs/qa/evidence/2026-09-29-session-continue-fork-r1-rewalk/w-rewind-after.png (before), docs/qa/evidence/2026-09-29-session-continue-fork-r1-rewalk/w-rewind-retest-after.png (after: composer "Second step"), docs/qa/evidence/2026-09-29-session-continue-fork-r1-rewalk/rewind-retest.txt.
