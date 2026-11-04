---
id: RT-session-live-steer
area: RT
title: Steer a live turn without killing it (default busy send)
persona: Théo
journey: J-13
expected: An unmarked send during a live turn resolves session.busy_input.default_mode (steer) and returns the full SendOutcome envelope; when the runtime announces steering or the provider capability allows it, the guidance is delivered into the live turn (steer_delivery injected, same turn id, prompt_steered marker, no cancel); when it cannot inject, active tools finish before pending_boundary cancels and reprompts, bounded by steer_boundary_timeout (45s); without tool events it reports after_turn and queues; 0s and explicit interrupt cancel immediately; the opposite modifier queues with a position; session status answers busy_input.steer_capability before any send; every disposition is answered inline in the Web composer and the draft survives a refusal.
entry_points: compozy session prompt (second send during a turn); POST /api/workspaces/{workspace_id}/sessions/{session_id}/prompt over HTTP and UDS; compozy__session_prompt; web session window composer (Enter / Cmd+Enter)
qa_status: pass
bug_ids: BUG-20260906-injected-guidance-missing-history
fix_status: pending
retest_status:
fix_commits:
evidence: docs/qa/reports/2026-09-06-sessions-stability.md
last_report: docs/qa/reports/2026-10-18-issue788-steer-attachments.md
overlaps: RT-018; RT-019
---

Start a long turn, send a second prompt without a mode, and compare the CLI envelope, the persisted
transcript markers and the Web disposition. On a runtime that announces steering the guidance must
land inside the same turn. On a provider without injection, start two tools and steer: no cancellation
may occur until both finish or the configured timeout expires. Confirm `pending_boundary` in the
receipt and Web feedback. Repeat with a provider emitting no tool events and confirm `after_turn`
without cancellation. Repeat with `steer_boundary_timeout="0s"`, then explicit interrupt while
tools run: both must cancel immediately. Send once more with the opposite modifier and confirm the entry queues with a position.

QA 2026-09-05 sessions-stability task_01: Claude Code 2.1.261 (claude-sonnet-5) announced steering and
the second CLI prompt returned disposition steering / steer_delivery injected on the live turn; the
transcript carried transcript_marker.prompt_steered inside that turn and the agent answered
STEER_ACK_7731 on the same turn id with done end_turn. The Web composer on a live Claude turn answered
"Steering — delivered into the live turn injected" and the same marker followed. The acpmock stubborn
agent (steer_capability none) reported interrupt_fallback: old turn cancelled with prompt_steered +
prompt_cancel markers, replacement answered STUBBORN_ACK; the Web read "Interrupted and replaced —
this agent can't take guidance mid-turn interrupt_fallback". Cmd+Enter queued "Queued #1" with a
durable after_turn entry. session status exposed busy_input.default_mode steer and steer_capability
before any send. Not walked: Codex and Cursor binaries (no steering announcement observed on their
current ACP builds; they fall back truthfully) and the stale-fence refusal from the Web (covered by
the canonical composer store tests and the RefusedTurnChanged story).

QA plan 2026-09-06: the integrated sessions-stability task10 re-walk is scoped by
`.compozy/tasks/sessions-stability/memory/qa.md`. Earlier evidence remains historical;
the changed queue/transcript/connection behavior has no current integrated verdict yet.

QA 2026-09-06 integrated verdict: Real Claude injected msg_handbook_guidance_02 in the same turn; authored text, identity and Steered provenance survive settlement, cold history and daemon restart. Task01 real capability/fallback evidence remains current. The missing-history and stopped-reader repairs were re-walked; see the integrated report.

QA support 2026-09-10: the real-ACP steer fallback integration passed twenty repetitions after making its stubborn process fixture ignore request cancellation as intended. Escalation, verified process replacement, replacement-before-queued ordering, and preserved active session assertions remain intact. Evidence: `docs/qa/reports/2026-09-10-release-integration-repair.md`.

QA 2026-10-18 (issue #788): image-bearing steer passed with real Claude/Sonnet 5 and
Codex/GPT-6.1-Sol in an isolated lab, both injected into the original turn and identifying the image.
Web Enter with an attachment, CLI `--steer --attachment` with exact retry, queue-to-steer image
promotion and history reload passed. Manager/SQLite race tests cover durable attachments and
fallback; the real ACP fallback integration passed. See
`docs/qa/reports/2026-10-18-issue788-steer-attachments.md` for scope and platform limitations.

QA 2026-11-04 (issue #1292): real Claude ACP 0.100.0 announced `steer_ext`, injected guidance
while a tool was active, and answered the steer marker in the same turn. Focused manager/SQLite
race cases verify boundary wait, overlapping tools, timeout, no-event queue, zero timeout and
explicit interrupt. Root Turbo Web checks cover the new delivery read models; no interactive
browser walk is claimed. See `docs/qa/reports/2026-11-04-issue1292-boundary-steer.md` for exact
scope, the historical-detection limitation, and delivery checks still pending.
