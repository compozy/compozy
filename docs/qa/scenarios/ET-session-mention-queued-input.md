---
id: ET-session-mention-queued-input
area: ET
title: Mentions survive queue, replace, steer, promote, and a daemon restart
persona: Théo
journey:
expected: A prompt with mentions queued behind a running turn lists `mentions` in the queue (HTTP, `compozy session input list`, compozy__session_inputs_list); replacing it resolves the new tokens only; steering with a token delivers its link into the live turn; promoting resolves on promote; a queued session mention whose target is deleted before dispatch is left out of the block; a queued input with mentions survives a daemon restart and dispatches with the same links.
entry_points: compozy session prompt --queue|--steer; compozy session input list|edit|steer; GET /api/workspaces/{workspace_id}/sessions/{session_id}/prompt/queue; compozy__session_inputs_list; web composer queue strip
qa_status: untested
bug_ids:
fix_status:
retest_status:
fix_commits:
evidence:
last_report:
overlaps: RT-session-live-steer; ET-session-mention-containment
---

US-007.AC-3, US-007.EC-5, US-010.AC-4/AC-5 (IT-004, IT-011). Start a long turn. Queue
`Next: @a.go and @session:<A>` with `--queue`; `compozy session input list <id> -o json`, the HTTP
queue route, and compozy__session_inputs_list each show `mentions` for `a.go` and A beside
`skill_invocations`. Edit the queued input to `Next: @b.go`: the listing shows only `b.go`. Steer
`Also @c.go` into the live turn: the live turn receives `c.go`'s link. When the queued turn
dispatches, only `b.go`'s link arrives.

Deleted-session leg: queue `Compare @session:<D>`, delete D, let it dispatch; the token stays in the
text and the references block omits D (no block when it was the only one). Restart leg: queue
`Read @a.go`, restart the daemon before the turn ends, and confirm the queued input still lists its
mention and dispatches with the same link. Promote a queued input with a token and confirm its link
is delivered.
