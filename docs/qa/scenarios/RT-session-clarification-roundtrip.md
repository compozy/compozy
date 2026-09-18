---
id: RT-session-clarification-roundtrip
area: RT
title: Answer a live agent clarification
persona: Théo
journey: J-answer-agent-requests
expected: A live session derives `waiting-for-input`, exposes one sanitized clarification through status and interaction discovery, accepts an offered choice or free text through Web, CLI, HTTP, or UDS, resumes the live tool with `answered`, resolves a restart-orphaned request with `resolved-after-restart`, returns the original winner on duplicate resolution, and keeps all evidence workspace-isolated; an unbounded pending request shows `deadline: null` with no countdown, stays answerable past the 60s mark while keepalive pings flow, and a late answer returns the answer while a finite policy still falls back with `timed_out`.
entry_points: Web session timeline; compozy__clarify; compozy session clarify pending/answer; GET/POST /api/workspaces/:workspace_id/sessions/:session_id/clarifications; compozy config get/set tools.clarify.timeout
qa_status: untested
bug_ids: BUG-20260917-clarify-timeout-config-set
fix_status: fixed
retest_status: pass
fix_commits: working-tree
evidence: /Users/pedronauck/dev/qa-labs/compozy-qa-rt-current-source-20260730-20260730-061631-252740-lab/qa-artifacts/qa; docs/qa/reports/2026-08-16-herdr-parity.md; /Users/pedronauck/dev/qa-labs/compozy-northstar-pay-20260816-141901-835450-lab/qa-artifacts/qa/bootstrap-manifest.json; docs/qa/reports/2026-09-17-clarify-keepalive.md
last_report: docs/qa/reports/2026-09-17-clarify-keepalive.md
overlaps: RT-021; MS-clarify-timeout-policy
---

Start a managed session with hosted native tools, invoke `compozy__clarify` with choices, and answer from
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

QA impact 2026-08-16: Task 01 made pending clarifications canonical across daemon restart, added
the `waiting-for-input` badge and status/interaction projections, and made orphan resolution resume
through the durable input queue. Reset to `untested`; task_08 owns the cross-surface walk.

Forensic evidence contract (SD-006) — each item cites timestamp, exact command, observed output:

- SSE question-event capture and the answer commands (CLI and HTTP variants) with the tool-result
  payload carrying the resolved choice.
- A finite-policy timeout run showing the explicit unanswered sentinel (Choice=nil, Text="", Fallback=true)
  treated as a non-answer.
- A >4-choices validation error, a rejected second concurrent question, and a cross-workspace list
  probe returning nothing.

src: .compozy/tasks/hermes-comparison/_user_stories.md#us-003-human-clarification-during-agent-work

QA 2026-08-16 Herdr parity: The full runtime E2E exercised the public HTTP, UDS, CLI, and native-tool paths, including matching persisted projections, restart recovery, scoped denials, bounded wait/notify/cancel/stop races, and stable negative outcomes (65/66/69/75/78, agent_scope_denied, and queue-full).

QA impact 2026-09-17 (clarify-keepalive tasks 01–03): omitted/`0s` `tools.clarify.timeout` is now
unbounded by default and a pending clarification emits `_compozy/clarify_ping` keepalive on a 30s
cadence, so the prior `pass` no longer describes this surface. Reset to `untested`; historical
evidence above is preserved. Task 05 owns the cross-surface walk. No web rows: zero-deadline
rendering is owned by `.compozy/tasks/clarify-timeout`. E2E-001/E2E-002 (see
`.compozy/tasks/clarify-keepalive/_tests.md`) are referenced as live-walk journeys for that
execution, not as automated-test claims.

Unbounded-wait and keepalive evidence expectations (all cite timestamp, exact command, observed output):

- An unbounded pending projection carries `"deadline": null` (never epoch/1970, never a fabricated
  countdown); the wait passes the 60s mark with `_compozy/clarify_ping` notifications observed every
  30s (`seq` from 1, daemon debug logs keyed by session/request ID), and a late `--choice` answer
  returns the exact answer (`fallback: false`), never the sentinel.
- A finite-policy run with no answer produces the fallback sentinel with `timed_out`; an invalid
  policy value fails with the exact `tools.clarify.timeout` error while the last valid policy stays.
- Answer/cancel/stop races resolve exactly once with deterministic loser errors; a second
  same-session ask conflicts naming the live request; extension-asked questions share the same wait
  and ping with no separate vocabulary; pings never cross sessions.
