# BUG-20260713-first-prompt-optimistic-stuck: First prompt can remain optimistic without reaching the session

- **Status:** verified
- **Impact (user-side):** Blocks-Completion
- **Severity:** Critical · **Priority:** P0
- **Persona Affected:** Bruno
- **Journey Step:** J-17, create a live session and send its first prompt
- **Scenarios:** RT-new-session-fast-feedback, GL-004
- **Found:** 2026-07-13 · **Report:** docs/qa/reports/2026-07-13-automation-features.md
- **Origin:** Post-fix live Cursor/Grok Goal acceptance

## Summary

Immediately after a fresh Cursor/Grok 4.5 session becomes usable, the first prompt can render optimistically as `Working…` forever without reaching the daemon. The UI disables normal submission controls and presents a stop-generation state, while the authoritative session remains idle with no active prompt, no Goal, and no persisted user message.

## Reproduction

1. Open Agents and create a `general` session with Cursor Agent and `Grok 4.5 (High, Fast)`.
2. Wait for the startup dialog to disappear and the session composer to render.
3. Send `/goal Reply in exactly one sentence: "AGH keeps agent work local-first and durable." The goal is complete when the response contains both local-first and durable. Do not use tools or modify files.`
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
- **Fix commit:** uncommitted QA remediation batch.
- **Regression test:** The canonical catalog-stream hook and app-layout suites require one global catalog stream, authoritative `workspace_id` filtering, and fan-out only to matching workspace query keys. The canonical destination runtime suite retains the exact hook-only StrictMode first-submit and local-cancellation contract.

## Verification

- A temporary canonical diagnostic reproduced the exact authoritative transcript shape: two assistant-only `data-agh-event` rows (`hook.dispatch.start` and `hook.dispatch.complete`), `epoch=0`, `generation=0`, `max_sequence=2`, enabled destination composer, and no prior user/provider text. A forced Turbo run passed with zero prompt fetches before submit and exactly one after the first `/goal`.
- The canonical replay was repeated under real `React.StrictMode` with an explicit fake EventSource open/cleanup/open lifecycle. It still reached exactly one prompt fetch, so neither the authoritative transcript shape nor StrictMode/SSE replay alone reproduces the live zero-fetch failure.
- The rejected navigation patch was removed. No speculative prompt transport, navigation, router, backend, or startup change remains from this investigation.
- The coupled local Stop failure is tracked separately as `BUG-20260713-stop-generation-local-stuck`; its correction does not claim to fix the missing prompt POST.
- Browser network instrumentation first reproduced the failure with five non-transcript SSE connections and a sixth transcript connection, with the prompt queued before network dispatch.
- After replacing only the three per-workspace catalog streams with `/api/sessions/catalog-stream`, fresh Cursor/Grok 4.5 session `sess-2a768148b6106dc3` held exactly four active streams: Vite console, workspace logs, one global catalog, and its transcript.
- Its first `/goal` click at `1783993289004` started exactly one prompt POST four milliseconds later, completed with HTTP 202 at `1783993289030`, and produced approved Goal Run `looprun-a6a4368bf1fc8c49` with durable `status=complete` and Run `status=done`.
- Evidence: `/Users/pedronauck/dev/qa-labs/agh-automation-features-post-onboarding-fix-20260713-20260713-203513-816377-lab/qa-artifacts/qa/network/catalog-global-goal-acceptance.json` and `qa/screenshots/catalog-global-goal-approved.png` in the same lab.
