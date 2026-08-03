# QA Run Report — 2026-08-02 — composer-space

- **Scope:** Session composer regression where sequential keyboard input dropped spaces after the assistant-ui update.
- **Cadence tier:** sanity
- **Build:** d30b810a + working-tree fix · **Environment:** isolated real daemon at `127.0.0.1:63871`, Vite at `localhost:3000`; Browser Use diagnosis plus Playwright replay; provider prompt dispatch out of scope.
- **Started:** 2026-08-02T20:08:39Z · **Completed:** 2026-08-02T20:19:24Z · **Status:** PASS

## Personas

| Persona | Base | Device / Network / Locale | Sessions |
|---|---|---|---|
| Bruno | Power User | desktop / wifi-fast / en-US | CH-session-composer-text-entry |

## Flows in Scope

- `J-17` — Create a durable session, arrive at its composer, and enter an exact multi-word draft (`../journeys/J-17-session-create-unified-selector.md`).

## Session Matrix & Results

| # | Charter | Journey / Scenario | Persona | Tour | Status | Issue | Fix commit |
|---|---|---|---|---|---|---|---|
| 1 | CH-session-composer-text-entry | J-17 / ET-web-session-composer-text-entry | Bruno | Feature Tour | Pass | | working tree |

Status legend: `Pending | Pass | Fixed | Skipped | Blocked (needs human verify) | Blocked (human decision)`

## Session Debriefs

### CH-session-composer-text-entry

- Created a durable session through the agent-detail session flow and targeted its own composer surface.
- Entered `space works now` one key at a time and observed an exact value match.
- Opened and closed the Next prompt runtime selector; the draft remained unchanged.
- Refreshed the page, then entered leading, repeated, and trailing spaces plus `café`; the exact DOM value was preserved.
- Returned through the session deep link and entered `Olá mundo com espaços`; the exact DOM value was preserved.
- The final replay produced no browser console warnings, console errors, or page errors.
- Behavioral evidence: `docs/qa/evidence/2026-08-02-composer-space/01-exact-draft.png`, `docs/qa/evidence/2026-08-02-composer-space/02-after-refresh.png`, and `docs/qa/evidence/2026-08-02-composer-space/03-deep-link.png`.

## What Was Fixed

### Session composer drops spaces during sequential typing

- **Symptom:** A normal multi-word prompt collapsed into one word while the user typed.
- **Root cause:** `@assistant-ui/store@0.3.2` created its notification manager with `useMemo`. React StrictMode can replay the initial mount with a new memo value, leaving composer state updates attached to a manager that no longer notifies the rendered subscriber. The textarea accepted a space briefly, then React restored the stale controlled value.
- **Fix:** Patch `@assistant-ui/store@0.3.2` to create the notification manager with lazy `useState`, matching the assistant-ui upstream correction while the fixed package release is not yet published.
- **Regression test:** The canonical `SessionThread` integration suite now renders the real assistant-ui runtime under `StrictMode` and asserts the exact multi-word composer value and session draft before and after remount.
- **Retested:** Browser Use reproduced the original input path and confirmed normal spaces after the patch. The isolated Playwright replay then covered runtime-selector interaction, refresh, deep-link return, repeated spaces, and accented text.

## Paper Cuts

None.

## Runtime Errors Observed

- No application runtime or browser console errors were observed in the final replay.
- Vite logged `ws proxy EPIPE` while short-lived automation browsers disconnected; the targeted inputs and final browser console remained clean.
- Chrome required a new local remote-debugging permission after the Browser Use daemon restarted, so the final deterministic replay used the repository's Playwright browser. This affected the automation connection only, not the application.

## Human Verifications Needed

None.

## Decisions for a Human

None.

## Learnings

- A controlled input can accept a keyboard event and still lose the character on the next React render; exact-value assertions are required for this class of regression.
- The upstream assistant-ui source already replaces `useMemo` with lazy `useState`, which gave a narrow and dependency-owned repair instead of a composer-local workaround.

## Final Status

PASS — the behavior-first replay passed. Final verify: `.cache/gate/final-make-verify.log`. Lab teardown: `/Users/pedronauck/dev/qa-labs/compozy-composer-space-20260802-195131-770494-lab/qa-artifacts/qa/teardown.json` (`clean: true`).
