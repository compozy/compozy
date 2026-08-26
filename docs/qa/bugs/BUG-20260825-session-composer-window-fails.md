# BUG-20260825-session-composer-window-fails: Opening a session replaces the composer with an error

- **Status:** verified
- **Impact (user-side):** Blocks-Completion
- **Severity:** Critical · **Priority:** P0
- **Persona Affected:** Bruno
- **Journey Step:** J-17 Launch a session, then choose its next-prompt runtime, step 3
- **Scenarios:** ET-web-session-composer-text-entry
- **Found:** 2026-08-25 · **Report:** docs/qa/reports/2026-08-25-lexical-composer-context.md

## Summary

After creating or opening a session, Bruno sees “Session failed to render” instead of the prompt composer and cannot continue the session journey.

## Reproduction

- **Charter:** CH-session-composer-text-entry · **Tour:** Feature Tour
- **Environment:** desktop / 1920×963 / wifi-fast / en-US

1. Open the Compozy workspace in the Web app.
2. Create a session with the `general` agent.
3. Wait for the destination session window to render.

**Expected:** The session window shows the prompt composer and accepts text.
**Actual:** The window error boundary shows `LexicalComposerContext.useLexicalComposerContext: cannot find a LexicalComposerContext`.

## Evidence

- User-provided browser stack trace in the originating issue.
- `web/src/components/assistant-ui/__tests__/session-thread.test.tsx` reproduced the same editor-mount failure before the fix.

## Fix

- **Root cause:** The app loaded Lexical 0.48 while `@assistant-ui/react-lexical` loaded Lexical 0.49, so the provider and consumer used different React context identities.
- **Fix commit:** current working tree (uncommitted)
- **Regression test:** `web/src/components/assistant-ui/__tests__/session-thread.test.tsx` — failed at editor mount before the fix and passes through the real composer after dependency alignment.

## Verification

- **Retested:** 2026-08-25, same persona/journey · **Report:** docs/qa/reports/2026-08-25-lexical-composer-context.md
- **Result:** A new session and a fresh deep-link return both rendered the Lexical composer, accepted exact Unicode text with repeated spaces, and showed no session-window error.
