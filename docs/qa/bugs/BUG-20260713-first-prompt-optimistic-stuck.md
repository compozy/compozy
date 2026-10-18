# BUG-20260713-first-prompt-optimistic-stuck: First prompt can remain optimistic without reaching the session

- **Status:** verified
- **Impact (user-side):** Blocks-Completion
- **Severity:** Critical · **Priority:** P0
- **Persona Affected:** Bruno
- **Journey Step:** J-17, create a live session and send its first prompt
- **Scenarios:** RT-new-session-fast-feedback; GL-004; RT-013
- **Found:** 2026-07-13 · **Report:** docs/qa/reports/2026-07-13-automation-features.md
- **Origin:** Post-fix live Cursor/Grok Goal acceptance

## Summary

Immediately after a fresh Cursor/Grok 4.5 session becomes usable, the first prompt can render optimistically as `Working…` forever without reaching the daemon. The UI disables normal submission controls and presents a stop-generation state, while the authoritative session remains idle with no active prompt, no Goal, and no persisted user message.

## Reproduction

1. Open Agents and create a `general` session with Cursor Agent and `Grok 4.5 (High, Fast)`.
2. Wait for the startup dialog to disappear and the session composer to render.
3. Send `/goal Reply in exactly one sentence: "Compozy keeps agent work local-first and durable." The goal is complete when the response contains both local-first and durable. Do not use tools or modify files.`
4. Observe the optimistic message and `Working…` state for more than 60 seconds.
5. Compare the durable session inspection, Goal, and transcript reads.

**Expected:** The first prompt reaches the daemon exactly once, becomes a durable Goal command, and either completes or returns a truthful runtime failure.
**Actual:** No prompt request reaches the daemon. The session inspection reports `active_prompt=false`, the Goal read returns `null`, and the transcript contains only the two session-creation hook events.

## Evidence

- Session `sess-e74df4386f8d5a77`, workspace `ws_30f28bfa2ef7ac98`.
- Browser DOM checkpoints retained `Working…` at 6.3 s, 21.6 s, 42.1 s, and 64.6 s.
- A second modal-to-session replay reproduced the same failure in session `sess-8eb726b62df96bb3`. The prompt was sent 37.190 seconds after `Start session`, after the destination composer and dismissed modal had been stable, yet the durable session again remained healthy/idle with no Goal or prompt POST.
- The second renderer's console showed the expected React StrictMode SSE lifecycle (`sse_open`, cleanup, `sse_open`) with no console error or warning before submission.
- `GET /api/workspaces/ws_30f28bfa2ef7ac98/sessions/sess-e74df4386f8d5a77/inspect`: healthy/idle, `active_prompt=false`.
- `GET /api/workspaces/ws_30f28bfa2ef7ac98/sessions/sess-e74df4386f8d5a77/goal`: `{"goal":null}`.
- The authoritative transcript remained at `epoch=0`, `generation=0`, `max_sequence=2`, with only `session.post_create` hook events.
- Daemon request logs contain the 201 session creation and subsequent reads but no prompt POST for this interaction.

## Fix

- **Root cause:** The app opened one session-catalog `EventSource` per registered workspace. On the affected three-workspace route, the Vite console stream, workspace log stream, three catalog streams, and session transcript stream occupied all six browser HTTP/1.1 connections. Assistant UI created the optimistic row, but the prompt fetch remained queued below the global fetch boundary until a socket became available; the daemon therefore received no request. StrictMode replay and the hook-only transcript were not causal.
- **Fix commit:** `514de24d6` for the document-wide transport follow-up below.
- **Regression test:** The canonical catalog-stream hook and app-layout suites require one global catalog stream, authoritative `workspace_id` filtering, and fan-out only to matching workspace query keys. The canonical destination runtime suite retains the exact hook-only StrictMode first-submit and local-cancellation contract.

## Verification

- A temporary canonical diagnostic reproduced the exact authoritative transcript shape: two assistant-only `data-compozy-event` rows (`hook.dispatch.start` and `hook.dispatch.complete`), `epoch=0`, `generation=0`, `max_sequence=2`, enabled destination composer, and no prior user/provider text. A forced Turbo run passed with zero prompt fetches before submit and exactly one after the first `/goal`.
- The canonical replay was repeated under real `React.StrictMode` with an explicit fake EventSource open/cleanup/open lifecycle. It still reached exactly one prompt fetch, so neither the authoritative transcript shape nor StrictMode/SSE replay alone reproduces the live zero-fetch failure.
- The rejected navigation patch was removed. No speculative prompt transport, navigation, router, backend, or startup change remains from this investigation.
- The coupled local Stop failure is tracked separately as `BUG-20260713-stop-generation-local-stuck`; its correction does not claim to fix the missing prompt POST.
- Browser network instrumentation first reproduced the failure with five non-transcript SSE connections and a sixth transcript connection, with the prompt queued before network dispatch.
- After replacing only the three per-workspace catalog streams with `/api/sessions/catalog-stream`, fresh Cursor/Grok 4.5 session `sess-2a768148b6106dc3` held exactly four active streams: Vite console, workspace logs, one global catalog, and its transcript.
- Its first `/goal` click at `1783993289004` started exactly one prompt POST four milliseconds later, completed with HTTP 202 at `1783993289030`, and produced approved Goal Run `looprun-a6a4368bf1fc8c49` with durable `status=complete` and Run `status=done`.
- Evidence: `/Users/pedronauck/dev/qa-labs/compozy-automation-features-post-onboarding-fix-20260713-20260713-203513-816377-lab/qa-artifacts/qa/network/catalog-global-goal-acceptance.json` and `qa/screenshots/catalog-global-goal-approved.png` in the same lab.

## Regressed (2026-07-29)

The same browser-level completion failure returned in CH-016 through a different mutation. Two literal
tabs opened the same idle session and rendered it in the foreground, but the first tab's public
`POST .../sessions/:sid/stop` remained queued inside Chrome and never reached the daemon. Browser
request `1092.1925` had no response, while the daemon log had no matching POST and continued to report
the session as `active`.

Each document retained its Window Manager WebSocket, global session-catalog SSE, and focused-session
transcript SSE; the Vite candidate additionally retained its HMR and console transports. The Web has
no document-visibility authority, so moving one tab to the background released none of its product
connections. Closing only the owned second tab at `2026-07-29T16:50:00.109Z` closed its transcript
stream; the already-pending stop immediately reached the daemon, returned 204 at
`2026-07-29T16:50:01.255Z`, and produced the durable `session_stopped` plus transcript marker events.

The previous one-global-catalog-stream correction remains valid but incomplete for multiple browser
documents. A structural document-transport ownership design is required before another implementation
change under the two-touch rule. Evidence:
`qa/evidence/046-session-lifecycle-browser/rt013-fixed-tabs-a.png`,
`qa/evidence/046-session-lifecycle-browser/rt013-fixed-tabs-b.png`, and the primary lab daemon log.

## Re-found during profile entry (2026-10-02)

- With multiple owned Chrome tabs on the primary lab's Vite origin, navigation produced a blank
  page with only the notifications container. Reload and a fresh tab could not complete;
  browser requests for the document and `/api/onboarding`, `/api/workspaces`, and `/api/status`
  remained without responses, while independent requests through the same proxy returned 200.
- Closing only the older owned lab tabs released the pending page: it rendered the desktop
  immediately with unchanged daemon state. Other browser tabs and processes were untouched.
- The current code already bounds covered session-window streams. Document-wide catalog and
  profile subscriptions still need investigation, including the developer console's extra SSE.
  The daemon-served production build confirms the same failure: two documents hold six
  HTTP/1.1 SSE connections and the third document never receives a response. Closing only the
  first two owned tabs immediately loads the third, without changing daemon state.
- **Evidence:** `profile-ui-blank.png`, `profile-ui-blank-*`, `profile-ui-tab-network-diagnostic.json`,
  `profile-ui-two-tab-observation.json`, `profile-ui-one-tab-control.json`, and
  `profile-ui-recovered-network.json` in `docs/qa/evidence/2026-10-02-untested/`.
- **Report:** `docs/qa/reports/2026-10-02-untested.md`. Profile identity/dialog walks had not begun;
  this precondition failure does not count as coverage of those controls.

### Transport repair verified (2026-10-02)

- Root cause is the three document-wide streams (session catalog, profile lifecycle logs, and
  worktree catalog) retaining the browser's HTTP/1.1 connection pool across tabs. Covered-window
  budgeting does not own these streams. Pausing them would lose background attention delivery
  and profile lifecycle sweeps.
- Add WebSocket upgrades to these existing stream routes, carrying the same named SSE frames
  and cursors. Retain HTTP/UDS SSE compatibility, scope validation, redaction, gateway tickets,
  and explicit socket shutdown. The three Web consumers opt in; no feature or notification is
  disabled to make navigation work.
- Canonical coverage: `TestBaseHandlersStreamSessionCatalog` owns scope/cursor/disconnect,
  `TestObserveStreamAndParseObserveQuery` owns log replay, `TestWorktreeStreams` owns worktree
  attribution, and `ticketed-event-source.test.ts` owns named delivery, tickets and reconnect.
- Production control evidence: `production-web-third-tab.json`,
  `production-three-tab-stall-control.json`, `production-three-tab-control-api.json`.

- Production-browser replay opens three fresh documents in 0.524, 0.362, and 0.500 seconds.
  Browser network evidence records 101 upgrades for all three document-wide route families.
  A real first prompt in `sess-80997493cb1ab1db` completes useful work, writes
  `founder-launch-email.md`, and persists exactly one user message among 42 events.
- A separate owner-profile omission initially prevents Web stop; BUG-20261002-session-web-stop-profile
  is fixed in the same commit. The replay stops the named-profile session with all three documents
  open, converges in a hidden peer, retains history across refresh and HTTP/UDS reads, and removes
  the session from the active catalog.
- A hidden Editorial document receives a profile identity change. While offline it retains the old
  identity; after network restoration it catches up and establishes new 101 connections. A later
  identity restoration arrives on the new log socket, proving resumed live delivery. No claim of
  browser cursor replay is made: the independent wire and canonical reconnect tests own that proof.
- Evidence: `multitab-fixed-three-reloaded.json`, `multitab-fixed-hidden-profile-entry.json`,
  `multitab-fixed-hidden-update-offline.json`, `multitab-fixed-reconnect-hidden-restored.json`,
  `multitab-fixed-profile-reconnected-live.json`, and `multitab-fixed-replay-summary.json`.
  The recording is `compozy-untested-20261002-isolated` (58 frames). Root gate passes; the dedicated
  Goal and startup-latency scenarios retain their separate pending coverage.
