# BUG-20260929-derive-refusal-below-fold: A Continue refusal (and "Open new session") lands below the fold

- **Status:** fixed (retested on the rebuilt web)
- **Impact (user-side):** Trust-Damage
- **Severity:** Medium · **Priority:** P2
- **Persona Affected:** Bruno
- **Journey Step:** J-14: Continue with a first message onto an agent whose runtime refuses it after the commit (422 + `child_session_id`)
- **Scenarios:** ET-web-session-continue; RT-session-derive-retry (step 8)
- **Found:** 2026-09-29 · **Report:** docs/qa/reports/2026-09-29-session-continue-fork-r1-rewalk.md

## Summary

In the default 604 px session window, the Continue dialog body with a first message fills the scroll area. After Continue, the refusal text, "The new session was already created." and **Open new session** rendered below Open in, which was out of view (error block top 636 px, scroller bottom 638 px). The primary re-enabled, so it looked as if nothing had happened, and the new session could be reached only by scrolling the dialog.

## Root cause

The dialog body is the scroll owner (window-capped host). The submit outcome was appended at the end of the body and nothing brought it into view.

## Fix

`SessionDeriveSubmitOutcome` (`web/src/systems/session/components/session-derive-committed-child.tsx`) now renders the refusal and the committed-child offer together, and scrolls them into view once when they appear (`scrollIntoView({ block: "nearest" })`). Both the Continue and Fork dialogs use it, and test ids are unchanged. Regression: `session-continue-dialog.test.tsx` "Should bring a post-commit refusal into view when it appears". It failed with `expected [] to include <div…>` when the ref was removed.

## Evidence

docs/qa/evidence/2026-09-29-session-continue-fork-r1-rewalk/w-committed-child.png (before), docs/qa/evidence/2026-09-29-session-continue-fork-r1-rewalk/w-committed-child-after-fix.png (after), docs/qa/evidence/2026-09-29-session-continue-fork-r1-rewalk/w-committed-child-opened.png.
