---
id: LR-7f4c2a-run-read-agent-journey
area: LR
title: Explain and inspect one Loop run through agent-readable projections
persona: Ada
journey: J-configure-and-run-loop
expected: The briefing, complete node roster, and durable timeline agree on current run truth over HTTP, UDS, and CLI; unblocker commands execute verbatim, attempt history survives recovery, timeline resume has no gaps or duplicates, and foreign positions fail deterministically.
entry_points: compozy loop why; compozy loop nodes --run --all; compozy loop events --after --follow --view; GET /api/workspaces/:workspace_id/loop-runs/:run_id/{briefing,nodes,timeline}
qa_status: untested
bug_ids:
fix_status:
retest_status:
fix_commits:
evidence:
last_report:
overlaps:
---

QA impact 2026-08-20: Task 03 adds the computed Loop run read layer and its agent-facing CLI verbs. This is a planning flag only; the workflow QA phase owns the real-daemon walk and evidence.
