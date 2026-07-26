---
id: RT-session-clarification-roundtrip
area: RT
title: Answer a live agent clarification
persona: Théo
journey: J-answer-agent-requests
expected: A live session shows one truthful clarification card, accepts an offered choice or free text through Web, CLI, HTTP, or UDS, resumes the waiting tool with the same structured answer, and keeps resolved, timed-out, or canceled evidence after reload without exposing another workspace.
entry_points: Web session timeline; agh__clarify; agh session clarify pending/answer; GET/POST /api/workspaces/:workspace_id/sessions/:session_id/clarifications
qa_status: untested
bug_ids:
fix_status:
retest_status:
fix_commits:
evidence:
last_report:
overlaps: RT-021
---

Start a managed session with hosted native tools, invoke `agh__clarify` with choices, and answer from
the Web timeline. Repeat with a free-text extension-tool question and answer through a different
public surface. Confirm the live pending projection is exact, a second simultaneous question is
rejected, choice numbering is one-based only in CLI presentation, and another workspace cannot list
or answer the request. Reload the transcript after resolved, timeout, session-stop cancellation, and
daemon-shutdown cancellation transitions; verify each receipt remains truthful and distinct from a
permission request. Exercise keyboard focus, narrow layout, submission failure recovery, and refresh
while pending as the experiential and edge-state sweep.

QA impact 2026-07-15: new native and extension clarification flow, Web question card, CLI/HTTP/UDS
answer surfaces, restart-required timeout config, and durable session evidence. Planning flag only;
no QA session ran in this implementation slice.

Phase C planning 2026-07-19: journey moved J-11 → J-answer-agent-requests (the interaction journey
now owns approvals + clarify); settles US-002 (D7, ADR-001).

Phase D remediation 2026-07-19: keep the durable pending question visible when the live
clarification read fails, with an explicit retry before answer controls return. Status remains
`untested` for the next QA cycle.

Forensic evidence contract (SD-006) — each item cites timestamp, exact command, observed output:

- SSE question-event capture and the answer commands (CLI and HTTP variants) with the tool-result
  payload carrying the resolved choice.
- A timeout run showing the explicit unanswered sentinel (Choice=nil, Text="", Fallback=true)
  treated as a non-answer.
- A >4-choices validation error, a rejected second concurrent question, and a cross-workspace list
  probe returning nothing.

src: .compozy/tasks/hermes-comparison/_user_stories.md#us-003-human-clarification-during-agent-work
