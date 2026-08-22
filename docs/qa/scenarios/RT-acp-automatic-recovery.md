---
id: RT-acp-automatic-recovery
area: RT
title: Complete one turn after an ACP runtime disconnect
persona: Théo
journey: J-automatic-runtime-recovery
expected: A provider disconnect preserves partial output, reports bounded recovery, replaces the runtime, completes the same turn once, and remains ready on fresh status and event reads without a terminal failure.
entry_points: web session window; compozy session prompt -o jsonl; HTTP session prompt and status; UDS session events; compozy__session_prompt; compozy__session_status; compozy__session_events
qa_status: pass
bug_ids:
fix_status: fixed
retest_status: pass
fix_commits: working-tree
evidence: docs/qa/evidence/2026-08-22-acp-runtime-recovery/CH-acp-automatic-runtime-recovery-retest.png
last_report: docs/qa/reports/2026-08-22-acp-runtime-recovery.md
overlaps: RT-acp-stream-disconnect-recovery; RT-acp-tool-update-burst; RT-visible-session-streaming
---

This scenario owns the successful recovery branch for a disconnect after partial assistant output.
The deterministic provider must force the process boundary while every Compozy surface remains a
production implementation. The replay may repeat an external side effect when the provider cannot
prove completion, so the user-facing documentation must state that trade-off.
