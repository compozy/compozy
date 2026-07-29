---
id: ET-workspace-access-prompt-outcomes
area: ET
title: Resolve a cross-workspace prompt and expire its session answer
persona: Ada
journey: J-operate-workspace-context
expected: An approve-reads session hitting the native-tool boundary raises one pending permission offering allow_once, allow_session, reject_once, and reject_session; once answers apply to that call only, session answers apply to every seam for the rest of the session, and stopping the session clears the answer so the next crossing prompts again.
entry_points: compozy__workspace_info; compozy session approve; POST /api/workspaces/:workspace_id/sessions/:session_id/approve; compozy logs
qa_status: untested
bug_ids:
fix_status:
retest_status:
fix_commits:
evidence:
last_report:
overlaps: ET-workspace-access-mode-matrix; ET-native-tool-approval-grants
---

Start an `approve-reads` session in workspace A and call a native tool naming workspace B. Confirm one
pending permission appears with the four daemon-computed options `allow_once`, `allow_session`,
`reject_once`, and `reject_session`, labeled Allow once, Allow for this session, Reject once, and
Reject for this session.

Answer `allow-once` and confirm the call proceeds and the next crossing prompts again. Answer
`reject-once` and confirm the call is denied with the permission-mode hint while a later crossing
still prompts.

Answer `allow-always` and confirm it resolves to `allow_session`: later crossings by that session
succeed with no prompt, and crossings at the task, spawn, and coordination seams — which never prompt
— now succeed too. Confirm no approval record appears in any list or revoke surface: the answer is
daemon memory only. Stop the session, start a new one for the same agent, and confirm the first
crossing prompts again. Repeat the expiry check across a daemon restart.

Answer `reject-always` and confirm it resolves to `reject_session` and denies every later crossing by
that session at every seam.

Leave one prompt unanswered past `[tools.policy] approval_timeout_seconds` and confirm the request
denies and stores no session answer. Inspect `compozy logs`: the initial prompt-eligible policy miss
is `workspace.access_denied`, while later evaluations admitted by `allow_session` are
`workspace.access_granted`. Confirm target, seam, source, and mode remain attributable.

QA impact 2026-07-29: new behavior from the cross-workspace access program (ADR-007). Planning flag
only; no QA replay ran in this documentation slice.
