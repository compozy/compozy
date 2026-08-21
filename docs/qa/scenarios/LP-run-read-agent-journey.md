---
id: LP-run-read-agent-journey
area: LP
title: Explain and inspect one Loop run through agent-readable projections
persona: Ada
journey: J-operate-loop-run-headless
expected: The briefing, complete node roster, and durable timeline agree on current run truth over HTTP, UDS, and CLI; unblocker commands execute verbatim, attempt history survives recovery, timeline resume has no gaps or duplicates, and foreign positions fail deterministically.
entry_points: compozy loop why <run-id>; compozy loop nodes --run <run-id> --all; compozy loop events <run-id> --after <seq> --follow --view <notable|all>; GET /api/workspaces/:workspace_id/loop-runs/:run_id/{briefing,nodes,timeline}; skills/compozy/references/loops.md; /docs/cli/loop/why; /docs/cli/loop/nodes; /docs/cli/loop/events
qa_status: untested
bug_ids: BUG-20260719-autonomous-progress-unobservable
fix_status: pending
retest_status:
fix_commits:
evidence:
last_report:
overlaps: LP-runs-roster-server-ordering; LP-terminal-loop-settlement
---

QA impact 2026-08-20: Task 03 adds the computed Loop run read layer and its agent-facing CLI verbs. This is a planning flag only; the workflow QA phase owns the real-daemon walk and evidence.

QA impact 2026-08-21: task_06 bound this row to the headless journey and added the official skill's loops reference plus the generated `loop why|nodes|events` CLI pages as entry points — the documented invocation is part of the agent contract.

This row carries the read layer that `BUG-20260719-autonomous-progress-unobservable` has been missing across six reproductions: `loop why` and `loop events --after --follow` are the runtime-owned progress stream an observer must be able to read instead of tailing a journey log. A known live defect to confirm rather than rediscover: the briefing publishes a `loop respond` unblocker string that cannot run as printed (`internal/loop/briefing.go:164`); the approval unblocker beside it is correct.
