---
id: ET-cli-session-usage-context
area: ET
title: Session usage exposes current context and per-turn history
persona: Rafa
journey: J-14
expected: CLI, HTTP and UDS agree on ledger-ordered context, nullable cache counts, per-turn usage and delivery union, and replay-span archive facts, including stopped sessions and read failures.
entry_points: compozy session usage; compozy session usage --turns; workspace session usage and usage/turns endpoints
qa_status: pass
bug_ids: BUG-20260912-context-delivery-details
fix_status: fixed
retest_status: pass
fix_commits:
evidence: docs/qa/reports/2026-09-12-session-context.md
last_report: docs/qa/reports/2026-09-12-session-context.md
overlaps: RT-session-cost-provenance, RT-acp-usage-cache-meta
---

Complete turns with context reports and cache counters. Compare CLI human, JSON and TOON output with
HTTP and UDS. Confirm the latest ledger sequence wins even if timestamps differ; totals accumulate
cache while context usage is a snapshot. A counter-only turn and a delivery-only turn remain visible.
Repeat reads after stopping the session. Unknown sessions and cross-workspace reads return 404.

Exercise an agent-reported window, catalog fallback, and no window. Only an agent-reported window can
carry the configured compaction pressure threshold. A fresh session has unknown context with absent
numbers. A failed ledger read retains aggregates with unavailable context; usage/turns reports an error.

Confirm transcript-stream push and polling paths emit session_usage_changed for new usage, done and
prompt_delivery events, without advancing the transcript cursor. Reconnect must preserve transcript
fences while usage refreshes.

Inspect a replay compaction marker before and after its span is archived. The marker changes its
span_archived fact without claiming that the agent's window shrank. Repeat after an initially failed
attempt whose span was later archived by another operation.

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
