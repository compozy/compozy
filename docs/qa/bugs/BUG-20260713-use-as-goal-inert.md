# BUG-20260713-use-as-goal-inert: Use as Goal does nothing on a real assistant response

- **Status:** verified
- **Impact (user-side):** Blocks-Completion
- **Severity:** High · **Priority:** P1
- **Persona Affected:** Lea
- **Journey Step:** J-26 start a conversational Goal, step 1
- **Scenarios:** GL-use-response-as-goal
- **Found:** 2026-07-13 · **Report:** docs/qa/reports/2026-07-13-automation-features.md
- **Origin:** n/a

## Summary

Lea received a substantial real Cursor/Grok response and clicked `Use as Goal`. Both pointer activation and keyboard Enter left the session unchanged: no Goal composer, draft, confirmation, navigation, toast, active chip, or error appeared. The response remained reusable only by manually copying it, so the visible Goal entry point is non-functional.

## Reproduction

- **Charter:** CH-use-response-as-goal · **Tour:** Feature Tour
- **Environment:** laptop / wifi-fast / en-US; live Cursor/Grok 4.5 session with a completed assistant response.

1. Submit a realistic workspace-analysis task and wait for the assistant response to finish.
2. Click the response action `Use as Goal`.
3. Activate the same control with keyboard Enter.
4. Inspect the composer, session header, route, notifications, and Goal state.

**Expected:** The action creates or prefills a truthful Goal draft from the selected response, gives immediate visible feedback, and leaves no hidden Goal when cancelled.
**Actual:** The action only receives focus; every visible and durable Goal surface remains unchanged and no error is explained.

## Evidence

- `/Users/pedronauck/dev/qa-labs/agh-automation-features-20260713-20260713-044543-173594-lab/qa-artifacts/qa/screenshots/ch-new-session-grok-transcript.dom.txt`
- `/Users/pedronauck/dev/qa-labs/agh-automation-features-20260713-20260713-044543-173594-lab/qa-artifacts/qa/journey-log.jsonl`
- Session `sess-b1c980b86709053d`; the action was activated twice with no route or Goal-state change.

## Fix

- **Root cause:** The response action did not hold the mutable outer assistant-ui composer authority, and its entire action row was still gated by `opacity: 0` plus `pointer-events: none` before hover. The button could render in the accessibility tree while a real pointer activation never reached the handler.
- **Fix commit:** uncommitted QA remediation batch
- **Regression test:** The canonical Session runtime integration now covers exact prefill, focus, existing-draft protection, discard, keyboard activation, and zero prompt POST; the production SessionThread story covers pointer and keyboard activation without forced-visibility CSS.

## Verification

- Same-persona in-app browser replay on live Cursor/Grok session `sess-7842125cce618d86` passed a real pointer activation. The selected response was prefixed exactly once as `/goal …`, the textarea received focus, `Goal command draft` appeared, and no prompt or Goal was submitted.
- An existing authored draft remained unchanged and received the actionable warning. `Discard Goal command` cleared the draft without adding a transcript item or Goal state.
- Keyboard activation is covered by the production Storybook play and runtime integration contract. The in-app browser driver's direct `press` API did not focus this button, so it was not counted as independent live-keyboard evidence.
- Evidence: `goal-use-as-goal-live-fixed.dom.txt` and `goal-use-as-goal-protects-draft.dom.txt` in the isolated QA screenshot directory.
