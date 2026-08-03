---
id: LP-live-pause-repair-resume
area: LP
title: Pause one Loop node, repair it, and resume safely
persona: Bruno
journey: J-recover-loop-node-failure
expected: A manual or rule-driven pause parks only the selected node at a safe boundary with provenance, excludes it from scheduling and clocks, and each resume variant continues once with the requested attempt policy while healthy sibling work remains intact.
entry_points: `compozy loop node pause|resume`; HTTP/UDS node-control routes; native tools; Web run controls
qa_status: untested
bug_ids:
fix_status:
retest_status:
fix_commits:
evidence:
last_report:
overlaps: LP-quarantine-diagnose-requeue; LP-cancel-vs-kill
---

acceptance-walk: Pause one live node at a safe boundary while a sibling continues, verify provenance and clock exclusion, then exercise plain, reset-attempts, and immediate resume variants on separate runs. Compare Web controls with structured CLI and HTTP state after refresh and confirm each accepted resume continues once with the requested attempt policy.
