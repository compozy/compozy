# BUG-20260928-derived-child-empty-state-hidden: A continued or forked child without a message shows a blank thread

- **Status:** fixed
- **Impact (user-side):** Friction
- **Severity:** Low · **Priority:** P3
- **Persona Affected:** Bruno
- **Journey Step:** J-14 derive branch: child opened before its first message
- **Scenarios:** ET-web-session-continue; ET-web-session-fork-from-here
- **Found:** 2026-09-28 · **Report:** docs/qa/reports/2026-09-28-session-continue-fork-exec-b2.md

## Summary

After Continue without a first message, the child showed only the divider over an empty pane. "Nothing said here yet" and its sentence (VC-20) never appeared.

## Root cause

A new session's transcript already holds `hook.dispatch.start/complete` status events projected as assistant messages. They render nothing, but `ThreadMessages` counted them, so `messageCount === 0` never held.

## Fix

`ThreadMessages` (`web/src/components/assistant-ui/session-thread-messages.tsx`) treats a transcript with no narrative message (user message or a message deriving at least one timeline row) as empty for the derived-child branch. Regression: `session-thread.test.tsx` "Should keep the empty state of a derived child whose transcript holds only status events" (fails without the fix). Re-walked (VC-20, both continue and fork children).

## Note

A plain new session has the same blank pane: its generic empty state is suppressed the same way. That's pre-existing and out of this feature's scope; see the report's Decisions for a Human.
