---
id: RT-pressure-context-compaction
area: RT
title: Never compact in CompozyOS and record the agent's own compaction exactly once
persona: Théo
journey: J-11
expected: CompozyOS never compacts a session itself: usage at 0.95 of the context window creates no child session, archives no rows, writes no checkpoint, and records no `session.compaction_fired` event unless the agent reports a compaction. When an agent honors the experimental ACP compaction capability, each compaction id yields raw `compaction` snapshot rows in the history ledger, exactly one folded Compaction item in the transcript projection, one `session.compaction_fired` event with the `{compaction_id, trigger, context_used, context_size}` payload, one usage marker, and one `context.pre_compact` plus one `context.post_compact` hook call, and the context reading stays unknown until the agent sends a later usage report.
entry_points: daemon session prompts; compozy session history -o json; GET /api/workspaces/{workspace_id}/sessions/{session_id}/transcript; compozy session events; compozy session usage -o json; GET /api/workspaces/{workspace_id}/sessions/{session_id}/usage/turns; registered context.pre_compact and context.post_compact hooks
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

- over the UDS raw prompt stream the same compaction update frames carry the typed, already-redacted
  `compaction` object (`compaction_id`, `status`, `summary?`, `error?`) and no raw payload data;
- the transcript projection (`GET …/transcript`, the layer the Web timeline reads) folds exactly one Compaction
  item (a `data-compozy-compaction` part) with `status: completed`, `started_at`, `ended_at` (the first terminal
  snapshot), and the summary (redacted first, then bounded to 16 KiB UTF-8-safe, ` [summary truncated]` appended
  past the cap, so a secret in the summary never survives and the bound holds after redaction); streaming chunks
  are never shown individually;
- `compozy session history -o json` and `GET …/history` do not fold: they return the raw grouped ledger rows for
  the id, at least two `compaction` snapshot rows ending `completed` plus the `session.compaction_fired` row;
- one `session.compaction_fired` event with `trigger: "agent"`, `context_used`, and `context_size` (both `null`
  in the event when no earlier context reading existed); the removed `from_sequence`, `to_sequence`, `pressure`,
  and `strategy` fields are absent;
- `GET …/usage/turns` lists one marker `{turn_id, sequence, at, compaction_id, trigger, status}` with
  `context_used` and `context_size` present only when the earlier reading was known (absent, not `null`,
  otherwise);
- `context.state` is `unknown` with `used`, `size`, and `ratio` absent (not `null`) immediately after the terminal
  frame, on a reread, after reopening the session, and after a daemon restart, until the agent sends a later usage
  report that carries a context `used` value (a counter-only report never restores it; a used-only report restores
  `reported`, or `estimated_size` when the model catalog knows the window; the Goal context reader is stricter, see
  GL-018). The prompt-response token totals that close the compaction turn keep the token counters and costs but
  never restore the reading, even when the agent reported occupancy earlier in that same turn: only a later genuine
  usage observation does;
- registered `context.pre_compact` (first update for the id) and `context.post_compact` (first terminal
  status) hooks each ran exactly once with the payloads in the public contract (`ContextCompactionPayload`;
  the pre event carries no `status`, `summary`, or `error`), and a hook patch that returns `deny`,
  `deny_reason`, `summary`, or `context_blocks` is rejected (the hook run records an `unknown field` error and the
  compaction is unaffected) while a `{"labels": {…}}` patch is recorded as applied.

Repeat with the recorded Codex frames: the Compaction item has no summary. Replay a duplicate terminal
frame and a corrected terminal update: the item updates in place and neither the event nor the hooks
fire again. The compaction's current status is what the CLI `outcome` and a Goal compaction turn report, so a
correction before the prompt completes (completed → failed, or failed → completed) changes them (owned by
ET-session-compact-now step 2 and RT-session-compact-real-adapters step 3). Replay an intermediate vendor status (for example `_paused`): the transcript item shows it verbatim
and is not treated as finished. A failed compaction shows `status: failed` with its `error` in the transcript
item and in the final raw snapshot row.

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
