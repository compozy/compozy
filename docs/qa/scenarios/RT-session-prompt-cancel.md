---
id: RT-session-prompt-cancel
area: RT
title: Cancel one prompt without stopping its session
persona: Ada
journey: J-15
expected: compozy session prompt-cancel and compozy__session_prompt_cancel use the same idempotent cancellation path as HTTP/UDS, return the canceled turn once, report nothing-in-flight on repeat, and keep the session alive for a later prompt.
entry_points: compozy session prompt-cancel; compozy__session_prompt_cancel; POST /api/workspaces/{workspace_id}/sessions/{session_id}/prompt/cancel
qa_status: untested
bug_ids:
fix_status:
retest_status:
fix_commits:
evidence:
last_report:
overlaps: RT-020; ET-web-session-transcript-calm-grammar
---

Start a blocking prompt, cancel it through each public surface, and submit a later prompt to the same
session. Confirm the first result carries `canceled` plus the exact turn ID, a repeat returns
`nothing-in-flight` with CLI exit `66`, concurrent cancel attempts converge on one provider cancel,
a failed provider cancel can be retried, self and foreign-workspace native targets are denied, and
the transcript does not render duplicate warning noise.

QA impact 2026-08-16: Task 04 added the prompt-cancel CLI verb and scoped native tool while preserving
the existing route's single production cancellation path. Flag only; task_08 owns execution.
