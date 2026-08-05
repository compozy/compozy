# BUG-20260805-command-menu-hook-order: Opening the session command menu leaves React in an unstable state

- **Status:** verified
- **Impact (user-side):** Trust-Damage
- **Severity:** High · **Priority:** P1
- **Persona Affected:** Théo
- **Journey Step:** J-use-session-slash-commands, step 1
- **Scenarios:** ET-session-slash-commands-inline
- **Found:** 2026-08-05 · **Report:** docs/qa/reports/2026-08-05-session-slash-commands.md

## Summary

The command menu opened and inserted the requested skill, but opening and closing it changed React's Hook order in two menu bridge components. The composer remained visible during this walk, yet every later render ran from an invalid component state and the browser console reported runtime errors.

## Reproduction

- **Charter:** CH-session-inline-slash-commands · **Tour:** Feature Tour
- **Environment:** desktop / 1280-class viewport / wifi-fast / en-US

1. Enter the app through the workspace switcher, open the existing `Command drafting` session, and focus the composer.
2. Type `/`, open the Built-in section, press Escape, then type `Revisão 😊 /bro before launch` and move the caret before the suffix.
3. Select `/browser-qa`, submit a real prompt containing the inline command, and inspect the public browser console.

**Expected:** The assistant-ui popover may mount and unmount without changing Hook order or emitting runtime errors.
**Actual:** React reported Hook-order changes for `CommandCatalogOpenReporter` and `CommandCatalogTriggerRangeBridge` after the popover state changed.

## Evidence

- `/Users/pedronauck/dev/qa-labs/compozy-session-slash-commands-20260805-035316-316748-lab/qa-artifacts/qa/console-before-fix.txt`
- `/Users/pedronauck/dev/qa-labs/compozy-session-slash-commands-20260805-035316-316748-lab/qa-artifacts/qa/screenshots/02-inline-command-preserves-text.png`

## Fix

- **Root cause:** The assistant-ui scope Hook was called through `ComposerPrimitive.unstable_useTriggerPopoverScopeContext`. React Compiler did not classify that property access as a Hook and reused it across renders, so the following `useRef` or `useLayoutEffect` became the first observed Hook. Importing the library export with the local `useTriggerPopoverScopeContext` name makes the compiler preserve the call on every render.
- **Fix commit:** `f54e62b`
- **Regression test:** `SessionThread composer running semantics / Should preserve hook order when the command menu closes and reopens`

## Verification

- **Retested:** 2026-08-05 in a fresh browser session against the isolated daemon and Web lab.
- **Result:** pass. The menu opened, closed, and reopened before inserting `/browser-qa` into `Revisão 😊 /bro before launch`; the exact suffix and Unicode prefix remained intact, with no page or console errors.
- **Evidence:** `/Users/pedronauck/dev/qa-labs/compozy-session-slash-commands-20260805-035316-316748-lab/qa-artifacts/qa/console-after-fix.txt`; `/Users/pedronauck/dev/qa-labs/compozy-session-slash-commands-20260805-035316-316748-lab/qa-artifacts/qa/screenshots/05-inline-command-hook-fix.png`
