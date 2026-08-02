---
id: LP-live-pause-repair-resume
area: LP
title: Pause one Loop node, repair it, and resume safely
persona: Bruno
journey: J-04
expected: A manual or rule-driven pause parks only the selected node at a safe boundary with provenance, excludes it from scheduling and clocks, and each resume variant continues once with the requested attempt policy while healthy sibling work remains intact.
entry_points: `compozy loop node pause|resume`; HTTP/UDS node-control routes; native tools; Web run controls
qa_status: blocked-verify
bug_ids:
fix_status:
retest_status:
fix_commits:
evidence:
last_report:
overlaps: LP-quarantine-diagnose-requeue; LP-cancel-vs-kill
---

QA impact 2026-08-02: Task 06 implements durable node pause, ordered auto-pause rules,
provenance, scheduling exclusion, and plain/reset-attempts/immediate resume authority. A real-user
walk is blocked until Task 07 exposes the public node-control verbs; Task 13 owns the isolated
cross-surface walk.
