---
id: LP-web-bell-loop-requests
area: LP
title: Open a pending Loop request from the attention bell
persona: Bruno
journey: J-supervise-loop-request
expected: The OS attention badge adds exact pending Loop-request aggregates to exact session needs-you totals without counting loaded rows. Each live request row names its workspace, request kind, Loop, and age. Selecting a row switches workspace when needed, opens the owning run, and focuses that request form. The Loop runs list counts every pending request page for the owning run. Answered, canceled, expired, or stale requests contribute no live count.
entry_points: web OS shell attention bell; web Loop runs list; web Loop run request form
qa_status: pass
bug_ids: ""
fix_status:
retest_status:
fix_commits: ""
evidence: "web/src/systems/loops/routes/__tests__/loop-runs.test.tsx (E2E-030: Loop request attention routes)"
last_report: docs/qa/reports/2026-08-18-graph-eng.md
overlaps: LP-ask-answer; RT-web-attention-bell-jump
---

story: As a Loop operator, I can see every live human request in one attention surface and land on the exact form that can resolve it.

src: .compozy/tasks/graph-eng/task_09.md

QA impact 2026-08-17: Task 09 added loop-request composition to the existing attention bell and run list. Flagged for the graph-eng QA tail.


QA impact 2026-10-17 (web performance): Unused Loop-node existence probes were removed from the shell. The existing Loop request-attention hook still owns rows, exact pending aggregates, scope, and polling visibility. Its current CI suite must preserve count/route behavior; no populated-request browser replay is claimed locally.

Scope, measured results, and verification limits: [Web performance report](../reports/2026-10-17-web-performance.md). Historical scenario verdicts above are not new verification of this revision. Heavy local gates are deferred to CI by explicit user instruction.
