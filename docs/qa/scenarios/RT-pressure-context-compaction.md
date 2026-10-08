---
id: RT-pressure-context-compaction
area: RT
title: Never compact in CompozyOS and record the agent's own compaction exactly once
persona: Théo
journey: J-11
expected: CompozyOS never compacts a session itself: usage at 0.95 of the context window creates no child session, archives no rows, writes no checkpoint, and records no `session.compaction_fired` event unless the agent reports a compaction. When an agent honors the experimental ACP compaction capability, each compaction id yields exactly one Compaction item in history, one `session.compaction_fired` event with the `{compaction_id, trigger, context_used, context_size}` payload, one usage marker, and one `context.pre_compact` plus one `context.post_compact` hook call, and the context reading stays unknown until the agent sends a later usage report.
entry_points: daemon session prompts; compozy session history -o json; compozy session events; compozy session usage -o json; GET /api/workspaces/{workspace_id}/sessions/{session_id}/usage/turns; registered context.pre_compact and context.post_compact hooks
qa_status: untested
bug_ids:
fix_status:
retest_status:
fix_commits:
evidence: /Users/pedronauck/dev/qa-labs/compozy-qa-rt-current-source-20260730-20260730-061631-252740-lab/qa-artifacts/qa
last_report: docs/qa/reports/2026-07-28-untested-full.md
overlaps: RT-session-context-rebuild; MS-workspace-checkpoint-continuity; ET-session-compact-now; RT-session-compact-real-adapters; ET-retired-product-surfaces-absent
---

Experimental: this scenario follows an unstable upstream ACP compaction contract. It has two halves and
both must be walked on an acpmock session unless the step says otherwise.

**Half 1 — CompozyOS never compacts.** On an agent that does not advertise the compaction capability,
complete at least one turn, then script `usage_update` reports at 0.95 of the context window and keep
the session idle for several minutes while sending two more prompts. Confirm:

- `compozy session list -o json` shows no child session.
- `compozy session history -o json` returns every row; compaction archives nothing (only a conversation
  rewind archives rows).
- `compozy session events` has no `session.compaction_fired`; no `context.pre_compact` or
  `context.post_compact` hook runs.
- `compozy session usage -o json` has no `pressure_threshold`, `compactions` is empty, and the Web
  context meter shows no warning band and no threshold tooltip sentence.
- The workspace gains no `project_checkpoint_summary.md`, and the agent's own "Compact conversation"
  tool row, if it runs one, stays an ordinary tool row.

**Half 2 — the agent's compaction is observed once.** On an acpmock session that honors the capability
(`clientCapabilities.session.compaction` advertised at initialize), replay the recorded Claude
compaction frames inside one turn. Confirm for the compaction id:

- history shows one `compaction` item with `status: completed` and the summary (redacted, at most
  16 KiB, ` [summary truncated]` appended past the cap); streaming chunks are never shown individually;
- one `session.compaction_fired` event with `trigger: "agent"`, `context_used`, and `context_size`;
  the removed `from_sequence`, `to_sequence`, `pressure`, and `strategy` fields are absent;
- `GET …/usage/turns` lists one marker `{turn_id, sequence, at, compaction_id, trigger, status,
  context_used, context_size}`;
- `context.used` is `null` with `state: "unknown"` immediately after the terminal frame, on a reread,
  after reopening the session, and after a daemon restart, until the agent sends a later usage report;
- registered `context.pre_compact` (first update for the id) and `context.post_compact` (first terminal
  status) hooks each ran exactly once with the payloads in the public contract, and a hook patch that
  returns `deny`, `deny_reason`, `summary`, or `context_blocks` fails patch validation while a
  `{"labels": {…}}` patch is accepted.

Repeat with the recorded Codex frames: the Compaction item has no summary. Replay a duplicate terminal
frame and a corrected terminal update: the item updates in place and neither the event nor the hooks
fire again. Replay an intermediate vendor status (for example `_paused`): the item shows it verbatim
and is not treated as finished. A failed compaction shows `status: failed` with its `error`.

Control: a successful ACP `session/load` performs no replay and adds no recovery marker (owned by
RT-session-context-rebuild). A session compacted by the removed CompozyOS compaction shows its full
history after the upgrade and its old `session.compaction_fired` rows stay as opaque ledger events that
never produce a marker (owned by RT-upgrade-memory-removal-home).

Forensic evidence contract (SD-006) — each item cites timestamp, exact command, observed output.

QA impact 2026-10-07 (memory removal): CompozyOS-side pressure compaction (summary-before-archive,
`project_checkpoint_summary.md`, `[session.compaction]`, `pressure_threshold`, archived spans) was
removed; this scenario was rewritten in place for the new behavior and reset to untested. The earlier
2026-07-28 and 2026-09-09 verdicts and reports describe the retired behavior and stay as history.
src: .compozy/tasks/memory-removal/adrs/adr-002.md
