---
id: ET-cli-session-usage-context
area: ET
title: Session usage exposes current context and per-turn history
persona: Rafa
journey: J-14
expected: CLI, HTTP and UDS agree on ledger-ordered context, nullable cache counts, per-turn usage and delivery union, and observed agent-compaction markers, including stopped sessions and read failures.
entry_points: compozy session usage; compozy session usage --turns; workspace session usage and usage/turns endpoints
qa_status: untested
bug_ids: BUG-20260912-context-delivery-details
fix_status: fixed
retest_status: pass
fix_commits:
evidence: docs/qa/reports/2026-09-12-session-context.md
last_report: docs/qa/reports/2026-09-12-session-context.md
overlaps: RT-session-cost-provenance; RT-acp-usage-cache-meta
---

Complete turns with context reports and cache counters. Compare CLI human, JSON and TOON output with
HTTP and UDS. Confirm the latest ledger sequence wins even if timestamps differ; totals accumulate
cache while context usage is a snapshot. A counter-only turn and a delivery-only turn remain visible.
In TOON, verify the context report timestamp and delivered rows. Join the separate turn, usage,
delivery, span, and compaction arrays by `turn_id`; compare raw values with JSON.
Human output uses the delivered section label, falling back to its key when no label exists.
Repeat reads after stopping the session. Unknown sessions and cross-workspace reads return 404.

Exercise an agent-reported window, catalog fallback, and no window. No window source carries a CompozyOS compaction
threshold: the usage payload has no `pressure_threshold`. A fresh session has unknown context with absent
numbers. A failed ledger read retains aggregates with unavailable context; usage/turns reports an error.

Confirm transcript-stream push and polling paths emit session_usage_changed for new usage, done,
prompt_delivery, compaction and session.compaction_fired events (so the meter and markers catch up while a compaction
turn is still running), without advancing the transcript cursor. Reconnect must preserve transcript
fences while usage refreshes. Commit a usage or settlement event between the usage read and transcript
projection read: the next refresh must still emit its sequence, turn ID, and kind.

Replay recorded agent compaction frames and inspect the marker `{turn_id, sequence, at, compaction_id, trigger,
status}` plus `context_used` and `context_size` in CLI human, `-o json`, TOON, HTTP, and UDS output; the two context
fields are omitted (absent, not `null`) when the reading before the compaction was unknown, and the removed
`from_sequence`, `to_sequence`, `pressure`, `strategy`, and `span_archived` fields are absent. After a terminal
compaction the context is `state: "unknown"` with `used`, `size`, and `ratio` absent on rereads, reopens, and a
daemon restart until the agent sends a later usage report with a context reading; the prompt-response token totals that
end the compaction turn update the counters and costs but never restore it. A `session.compaction_fired` row
recorded before the upgrade appears in `compozy session events` as opaque history and never produces a marker.

Execution owner: session-context tasks 05/06. Task 04 completes real injected delivery rows before this walk.

## Delivered context extension

1. On a fixture with real startup assembly and skills augmentation, send two turns with unchanged skills.
2. Read usage JSON, turns JSON and session events through CLI and HTTP. Confirm `bytes_div_4`, one full
   owner for each section, the actual stub bytes in turn two, `startup_dedup` on the first-turn stub,
   and a durable `prompt_delivery` receipt after each confirmed prompt.
3. Send a named text attachment and a named image. Confirm estimated text tokens and binary bytes
   without fabricated tokens. Apply a replacing post-assemble hook in a separate session and confirm
   one `system_prompt` owner with `hook_modified`, plus startup-opaque dedup rows without tokens.
4. Script a lower reported context and then a changed full section. Confirm possible summarization
   after the drop, and freshness restored only for the re-delivered section. A late confirmation
   without a drop must remain fresh.
5. Fail a transport dispatch and confirm no delivery receipt. Stop the session and repeat reads to
   verify the same persisted receipt history. Record actual results during task 06.

QA 2026-09-12: session-context final feature pass; runtime, focused integration/browser and 43 visual-pair evidence are separated in the linked report. Unchanged lifecycle/roll-up behavior reuses the earlier owning evidence; this pass verifies the new Context surface and its coexistence.

QA impact 2026-10-07 (memory removal): `pressure_threshold` and the replay-span archive fields left the usage payloads, and `compactions[]` now lists only observed agent compactions. New turns never record Memory or Workspace knowledge delivery rows; rows written before the upgrade render by their raw key. Stale verdict reset to untested; historical evidence preserved; no QA session ran.

Marker after fields (S14 / UT-W08): send a fresh occupancy report within the compaction's own turn. Experimental `compactions[].context_after` must carry that first event's used, optional size and exact sequence, with zero retained. Human output shows before → after; JSON carries the object and TOON carries the three after columns. Counter-only events and later status/summary corrections cannot create or replace it. If another compaction terminates before an occupancy report, the earlier marker has no after. Unknown occupancy exposes experimental `context.cleared_by` with the ID and its first terminal sequence; the cause disappears on fresh occupancy. Automated owning query/contextusage/API/CLI checks cover these invariants; no new CLI/browser scenario walk is claimed.
