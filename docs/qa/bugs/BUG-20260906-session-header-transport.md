# BUG-20260906-session-header-transport: OS session header omits degraded connection state

- **Status:** fixed — integrated browser re-walk passed
- **Impact:** Confusion · **Severity:** Major · **Priority:** P1
- **Persona:** Théo · **Scenario:** RT-visible-session-streaming
- **Found:** 2026-09-06 · **Report:** docs/qa/reports/2026-09-06-sessions-stability.md

In the real OS session window, block only its HTTP SSE endpoint at the browser network boundary, restore the window and prompt through the CLI. The real EventSource retries and reports six failures. The thread preserves its messages and shows Live updates stopped / Try again, but the session header has no Reconnecting or Disconnected chip. DOM query for session-transport-chip returns no element. Snapshot: integrated lab evidence/transport-retries-exhausted.json/png. The standalone Storybook chip passes; the defect is in the live host integration.

Candidate boundary: use-session-window-controller registers a context-consuming SessionTransportChip React node into a topbar rendered outside SessionChatRuntimeProvider. Frontend worker owns proving and repairing the real slot consumer, with per-window isolation and no additional stream. Unblocking the URL and pressing the actual Try again catches up the public transcript while preserving the unsent draft; it does not prove the missing header fixed.

The publisher now captures the session transport snapshot inside its provider and wraps the
published chip with that same scoped value. The real TopbarSlot regression failed before and
passed after the change. Re-walk: transport-fixed-grace.json has no chip within the grace;
transport-fixed-retry-count.json/png shows Reconnecting ·4; transport-fixed-draft-guard.json/png
shows Disconnected after six failures and the Not sent feedback with the exact retained draft.
After fixing the separate current-watermark server defect, actual Try again restores live state
at the same cursor without another event or auto-send (transport-fixed-recovered.json/png).
All three final PNGs were inspected; errors=[] throughout.
