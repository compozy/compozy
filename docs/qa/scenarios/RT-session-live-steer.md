---
id: RT-session-live-steer
area: RT
title: Steer a live turn without killing it (default busy send)
persona: Théo
journey: J-13
expected: An unmarked send during a live turn resolves session.busy_input.default_mode (steer) and returns the full SendOutcome envelope; when the runtime announces steering or the provider capability allows it, the guidance is delivered into the live turn (steer_delivery injected, same turn id, prompt_steered marker, no cancel); when it cannot inject, the same send reports interrupt_fallback and the replacement runs after the old turn is cancelled; the opposite modifier queues with a position; session status answers busy_input.steer_capability before any send; every disposition is answered inline in the Web composer and the draft survives a refusal.
entry_points: compozy session prompt (second send during a turn); POST /api/workspaces/{workspace_id}/sessions/{session_id}/prompt over HTTP and UDS; compozy__session_prompt; web session window composer (Enter / Cmd+Enter)
qa_status: pass
bug_ids:
fix_status:
retest_status:
fix_commits:
evidence: docs/qa/reports/2026-09-05-sessions-stability-task01-02.md; /Users/pedronauck/dev/qa-labs/compozy-sessions-stability-task01-02-20260905-154017-502928-lab/qa-artifacts/qa/evidence/walkD-prompt-steer.jsonl; /Users/pedronauck/dev/qa-labs/compozy-sessions-stability-task01-02-20260905-154017-502928-lab/qa-artifacts/qa/evidence/walkD-events.json; /Users/pedronauck/dev/qa-labs/compozy-sessions-stability-task01-02-20260905-154017-502928-lab/qa-artifacts/qa/evidence/walkC-prompt-steer.jsonl; /Users/pedronauck/dev/qa-labs/compozy-sessions-stability-task01-02-20260905-154017-502928-lab/qa-artifacts/qa/evidence/walkC-events.json; /Users/pedronauck/dev/qa-labs/compozy-sessions-stability-task01-02-20260905-154017-502928-lab/qa-artifacts/qa/evidence/screenshots/f3-03-claude-steering-injected.png; /Users/pedronauck/dev/qa-labs/compozy-sessions-stability-task01-02-20260905-154017-502928-lab/qa-artifacts/qa/evidence/screenshots/f1-03-steer-disposition.png; /Users/pedronauck/dev/qa-labs/compozy-sessions-stability-task01-02-20260905-154017-502928-lab/qa-artifacts/qa/evidence/screenshots/f4-01-queued-disposition.png
last_report: docs/qa/reports/2026-09-05-sessions-stability-task01-02.md
overlaps: RT-018; RT-019
---

Start a long turn, send a second prompt without a mode, and compare the CLI envelope, the persisted
transcript markers and the Web disposition. On a runtime that announces steering the guidance must
land inside the same turn; on one that cannot inject the send must fall back to interrupt-and-replace
and say so. Send once more with the opposite modifier and confirm the entry queues with a position.

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
