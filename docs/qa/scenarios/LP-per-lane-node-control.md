---
id: LP-per-lane-node-control
area: LP
title: Control one fan-out lane without changing its siblings
persona: Bruno
journey: J-complete-partial-loop
expected: The --item and item_index variants of respond, amend, pause, resume, cancel, and rerun resolve one exact fan-out cell, preserve sibling state, carry the addressed lane through provenance and events, and reject unknown or stale lane identities deterministically across CLI, HTTP, UDS, and native tools.
entry_points: compozy loop respond --item; compozy loop node amend|pause|resume|cancel --item; compozy loop rerun --item; HTTP and UDS item_index routes; compozy__loop_respond; compozy__loop_node_amend|pause|resume|cancel|requeue; compozy__loop_rerun
qa_status: pass
bug_ids:
fix_status:
retest_status:
fix_commits:
evidence:
last_report: docs/qa/reports/2026-08-31-issue-500-forced-loop-cancel.md
overlaps: LP-forced-cancel-owned-sessions; LP-amend-rerun; LP-fail-fast-lane-cancel
---

Use a multi-lane run and compare the addressed cell with at least one healthy sibling after every verb through an independent read surface.

QA impact 2026-08-31: Kill was removed; addressed Cancel now owns immediate fencing and session cleanup.
