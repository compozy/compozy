---
id: RT-session-mark-seen
area: RT
title: Mark a session's Needs you as seen and see it come back on the next occurrence
persona: Théo
journey: J-15
expected: Mark as seen clears a dismissible Needs you (a turn that ended early, a return from Later, a stopped session's failure) on every surface and persists across reload and daemon restart; the next occurrence raises Needs you again; a session waiting on a permission answer, a question, a sign-in, a stop, or a work signal it cannot check is refused with attention_requires_action and keeps its badge; creators are not woken by the dismissal.
entry_points: Web session row menu, Kanban card menu and context menu ("Mark as seen"), session header button; CLI session mark-seen; HTTP+UDS POST /api/workspaces/{workspace_id}/sessions/{session_id}/mark-seen; native tool session_mark_seen
qa_status: pending
bug_ids:
fix_status:
retest_status: pending
fix_commits:
evidence:
last_report:
overlaps: RT-session-attention-catalog; RT-session-later; ET-web-sessions-board-card-actions-group-sort
---

Mark as seen acknowledges one occurrence; it never mutes a session. The payload field
`attention_dismissible` decides where the control shows, so a pending answer is never hidden.

## Dismissible sources

1. Stop a session so it fails (kill its agent process) and confirm it reads Failed in Needs you,
   counts in the Needs you facet, and sits in the Kanban Needs you column.
2. From the row `…` menu choose "Mark as seen". The row reads Stopped, leaves Needs you and the
   column, and the Needs you count drops. Reload the web app and restart the daemon: it stays seen.
3. Resume the session and fail it again: Failed and Needs you return.
4. Repeat from the Kanban card menu and the session header button with a child session whose turn
   ended without a pull request ("Stopped before delivering") and with a session back from Later.
   The next early stop or the next return from Later raises Needs you again.

## Refused sources

1. Leave a permission prompt and a clarify question pending. Neither row, card, nor header offers
   "Mark as seen"; the badge and the decision dock stay.
2. `compozy session mark-seen <id>` and `compozy__session_mark_seen` on that session fail with
   `attention_requires_action` and name the answer still owed; HTTP returns 409.
3. A session with nothing to dismiss returns unchanged (200, no new attention event).

## Side effects

The dismissal publishes one attention edge (catalog, hooks) and never wakes the creator session.
