---
title: Canceling a prompt and clearing a conversation no longer collide
type: fix
---

Two timing failures met in the same flow — cancel an active prompt, clear the conversation, keep using the session. Both are fixed. (#523)

- A late prompt cancel could cancel the next turn. Prompt cancellation now owns only the active request and becomes a no-op once that prompt has settled; the whole-session `session/cancel` is sent exactly once, and only by Stop.
- Because of that, an ACP agent process no longer receives a `session/cancel` notification when a single prompt is canceled — only the SDK's request-scoped `$/cancel_request`. An agent that aborted a turn by listening for `session/cancel` must handle the request-scoped cancellation instead.
- A transcript read could land inside the conversation-clear replacement window, report `session not found`, and drive the clear endpoint to HTTP 500. The finalization barrier is now published as soon as the conversation-operation lock is taken and held through stop, backup, database replacement, and restart, so readers wait for the clear instead of seeing a session that appears to be missing.
