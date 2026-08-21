---
id: LP-run-read-agent-journey
area: LP
title: Explain and inspect one Loop run through agent-readable projections
persona: Ada
journey: J-operate-loop-run-headless
expected: The briefing, complete node roster, and durable timeline agree on current run truth over HTTP, UDS, and CLI; unblocker commands execute verbatim, attempt history survives recovery, timeline resume has no gaps or duplicates, and foreign positions fail deterministically.
entry_points: compozy loop why <run-id>; compozy loop nodes --run <run-id> --all; compozy loop events <run-id> --after <seq> --follow --view <notable|all>; GET /api/workspaces/:workspace_id/loop-runs/:run_id/{briefing,nodes,timeline}; skills/compozy/references/loops.md; /docs/cli/loop/why; /docs/cli/loop/nodes; /docs/cli/loop/events
qa_status: blocked-decision
bug_ids: BUG-20260719-autonomous-progress-unobservable; BUG-20260821-loop-unblocker-invalid-json; BUG-20260821-loop-timeline-head-omitted
fix_status: deferred
retest_status:
fix_commits: a53f470; b0eaf22; 37c101d
evidence: /Users/pedronauck/dev/qa-labs/compozy-loop-unblocker-operator-input-20260821-20260821-124157-149087-lab/qa-artifacts/qa/request-unblocker-required-schema-rewalk.md; /Users/pedronauck/dev/qa-labs/compozy-loop-task-legibility-runtime-20260821-1126-20260821-112711-004724-lab/qa-artifacts/qa/headless/read-parity.sha256; /Users/pedronauck/dev/qa-labs/compozy-loop-task-legibility-runtime-20260821-1126-20260821-112711-004724-lab/qa-artifacts/qa/observation-summary.json
last_report: docs/qa/reports/2026-08-21-loop-task-legibility.md
overlaps: LP-runs-roster-server-ordering; LP-terminal-loop-settlement
---

QA impact 2026-08-20: Task 03 adds the computed Loop run read layer and its agent-facing CLI verbs. This is a planning flag only; the workflow QA phase owns the real-daemon walk and evidence.

QA impact 2026-08-21: task_06 bound this row to the headless journey and added the official skill's loops reference plus the generated `loop why|nodes|events` CLI pages as entry points — the documented invocation is part of the agent contract.

This row carries the read layer that `BUG-20260719-autonomous-progress-unobservable` has been missing across six reproductions: `loop why` and `loop events --after --follow` are the runtime-owned progress stream an observer must be able to read instead of tailing a journey log. A known live defect to confirm rather than rediscover: the briefing publishes a `loop respond` unblocker string that cannot run as printed (`internal/loop/briefing.go:164`); the approval unblocker beside it is correct.

QA result 2026-08-21: CLI, HTTP, and UDS briefing/node/timeline projections matched
semantically; resume after sequence 5 returned exactly 6–10 without duplicates; follow at head 10
closed cleanly. The printed request unblocker and beyond-head diagnostic were fixed and re-walked.
The required-schema unblocker re-walk passed in the fresh remediation lab: the printed command
waited for explicit operator JSON and never supplied a default. The row is `blocked-decision`
because `BUG-20260719-autonomous-progress-unobservable` remains open and its observer contract
requires the product/QA decision recorded in that bug.
