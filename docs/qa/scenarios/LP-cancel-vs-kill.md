---
id: LP-cancel-vs-kill
area: LP
title: Distinguish cooperative Loop cancellation from immediate kill
persona: Bruno
journey: J-04
expected: Cancel is durably visible through requested, delivering, and draining before canceled(operator_cancel); kill immediately stops the bound session and ends canceled(operator_kill); repeated verbs are idempotent, races produce one terminal effect, and the removed stop verb is unavailable.
entry_points: `compozy loop cancel|kill`; `compozy loop node cancel|kill`; HTTP/UDS routes; native tools; Web run controls
qa_status: blocked-verify
bug_ids:
fix_status:
retest_status:
fix_commits:
evidence:
last_report:
overlaps: LP-016
---

QA impact 2026-08-02: Task 05 implements the run- and node-level authorities, cancellation
ledger, cleanup, and terminal contract. A real-user walk is blocked until Task 07 ships the public
verbs and Task 08 ships the Web controls; Task 13 owns the cross-surface race walk.
