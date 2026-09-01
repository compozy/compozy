---
id: RT-session-prompt-cancel
area: RT
title: Cancel one prompt without stopping its session
persona: Ada
journey: J-15
expected: compozy session prompt-cancel and compozy__session_prompt_cancel use the same idempotent cancellation path as HTTP/UDS, return the canceled turn once, report nothing-in-flight on repeat, and keep the session alive for a later prompt.
entry_points: compozy session prompt-cancel; compozy__session_prompt_cancel; POST /api/workspaces/{workspace_id}/sessions/{session_id}/prompt/cancel over HTTP and UDS
qa_status: pass
bug_ids:
fix_status: fixed
retest_status: pass
fix_commits:
evidence: docs/qa/reports/2026-08-16-herdr-parity.md; /Users/pedronauck/dev/qa-labs/compozy-northstar-pay-20260816-141901-835450-lab/qa-artifacts/qa/bootstrap-manifest.json;docs/qa/reports/2026-09-01-issue-521-522-session-cancel-clear.md;/Users/pedronauck/dev/qa-labs/compozy-issue-521-522-session-cancel-clear-20260901-183338-907244-lab/qa-artifacts/qa
last_report: docs/qa/reports/2026-09-01-issue-521-522-session-cancel-clear.md
overlaps: RT-020; ET-web-session-transcript-calm-grammar
---

Start a blocking prompt, cancel it through each public surface, and submit a later prompt to the same
session. Confirm the first result carries `canceled` plus the exact turn ID, a repeat returns
`nothing-in-flight` with CLI exit `66`, concurrent cancel attempts converge on one provider cancel,
a failed provider cancel can be retried, self and foreign-workspace native targets are denied, and
the transcript does not render duplicate warning noise.

QA impact 2026-08-16: Task 04 added the prompt-cancel CLI verb and scoped native tool while preserving
the existing route's single production cancellation path. Flag only; task_08 owns execution.

QA 2026-08-16 Herdr parity: The full runtime E2E exercised the public HTTP, UDS, CLI, and native-tool paths, including matching persisted projections, restart recovery, scoped denials, bounded wait/notify/cancel/stop races, and stable negative outcomes (65/66/69/75/78, agent_scope_denied, and queue-full).

QA impact 2026-09-01: ACP prompt cancellation is now request-scoped, while session-scoped cancellation
is reserved for whole-session Stop. Reset for repeat-cancel and next-turn integrity across public
HTTP, UDS, CLI, native-tool, and provider-backed runtime surfaces.

QA 2026-09-01: Codex-backed session `sess-a63b87a64ffa9285` canceled active turn
`turn-8a40d61cac8706d7`, returned `nothing-in-flight` for repeated CLI, HTTP, and native-tool
requests, and completed `NEXT_TURN_OK` on the same durable session without a late session cancel.
