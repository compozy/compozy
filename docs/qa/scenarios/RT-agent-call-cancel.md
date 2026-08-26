---
id: RT-agent-call-cancel
area: RT
title: Cancel a running agent call idempotently
persona: Bruno
journey: J-delegate-work-to-an-agent
expected: Cancel fences activation, stops the managed child, settles canceled once, and a repeated cancel returns the same terminal state.
entry_points: compozy call cancel call_01JBD8H9PW2M --reason "superseded by rev-02"; HTTP and UDS POST /api/workspaces/{workspace_id}/calls/{call_id}/cancel with {"reason":"superseded by rev-02"}; compozy__call_cancel with {"call_id":"call_01JBD8H9PW2M","reason":"superseded by rev-02"}; compozy call show call_01JBD8H9PW2M; HTTP and UDS GET /api/workspaces/{workspace_id}/calls/{call_id}/superseded
qa_status: untested
bug_ids:
fix_status:
retest_status:
fix_commits:
evidence:
last_report:
overlaps: RT-agent-call-golden-path; RT-agent-call-deadline-timeout; RT-session-stop-subtree
---

Cancel a long-running call, probe the child process, repeat the cancellation, and inspect the stored call.

Cancel a call whose activation run has not yet been claimed and one whose claim already won: the
first must be fenced out of the claimable set through the task authority, the second must fall back
to a managed stop. Then let a late child result arrive after cancellation and confirm it lands only
in superseded evidence — post-terminal writes are rejected and the terminal is never mutated. A
cancel on an already-terminal call is an idempotent success, never `call_already_settled`.
