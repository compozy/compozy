# BUG-20260713-new-session-modal-lingers: successful session creation keeps the blocking modal mounted

- **Status:** verified
- **Impact (user-side):** Friction
- **Severity:** High · **Priority:** P0
- **Persona Affected:** Bruno
- **Journey Step:** J-17 create a session, step 2
- **Scenarios:** RT-new-session-fast-feedback
- **Found:** 2026-07-13 · **Report:** docs/qa/reports/2026-07-13-automation-features.md
- **Origin:** BUG-20260713-cursor-model-startup-contract retest

## Summary

After the daemon returned HTTP 201 and the destination session route/composer was ready, the global `Start a new session` dialog remained mounted with `Starting session…` for roughly another 17.7 seconds. Its overlay and focus trap blocked the already-usable session.

## Reproduction

- **Charter:** CH-new-session-latency-title · **Tour:** Network Tour
- **Environment:** desktop / wifi-fast / en-US; isolated daemon at `http://127.0.0.1:58941`; in-app browser; live Cursor/Grok 4.5 ACP.

1. Open the `general` agent and click `New session`.
2. Select Cursor Agent and `Grok 4.5 (High, Fast)`.
3. Click `Start session` once and measure the POST, route readiness, composer readiness, and dialog lifetime separately.

**Expected:** Pending feedback remains visible while ACP startup is genuinely in flight. After HTTP 201, the dialog releases its overlay/focus trap before destination navigation begins and does not remount while route loaders settle.
**Actual before fix:** The daemon returned HTTP 201 in 5.255 seconds and the session route/composer existed at 5.545 seconds, but the blocking modal remained until 22.940 seconds.

## Evidence

- `/Users/pedronauck/dev/qa-labs/agh-automation-features-20260713-20260713-044543-173594-lab/qa-artifacts/qa/screenshots/new-session-modal-truthful-at-six-seconds.dom.txt`
- `/Users/pedronauck/dev/qa-labs/agh-automation-features-20260713-20260713-044543-173594-lab/qa-artifacts/qa/screenshots/new-session-modal-dismissed-after-post.dom.txt`
- `/Users/pedronauck/dev/qa-labs/agh-automation-features-20260713-20260713-044543-173594-lab/qa-artifacts/qa/screenshots/new-session-modal-timing.json`
- Verified live session: `sess-c09b90c914321946`.

## Fix

- **Root cause:** The dialog's successful submit path awaited TanStack navigation, so the creation mutation and its pending UI ownership outlived the successful POST and destination-route readiness.
- **Fix commit:** pending
- **Regression test:** The canonical `useSessionCreateDialog` suite holds navigation unresolved and proves the dialog/pending state has already cleared and the destination composer can mount.

## Verification

On the fixed live replay, the modal remained truthfully mounted at 6.027 seconds because the provider POST had not completed. HTTP 201 arrived 6.452 seconds after the click, and the first destination-route GET began 7 ms later. The destination session rendered a focusable composer with neither the creation dialog nor runtime selector mounted. The focused Web suite passed 47/47, with exact format/lint and React Doctor clean; visual helper teardown reports `clean: true`.
