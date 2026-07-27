---
id: LP-runtime-selection-overrides
area: LP
title: Select per-task runtimes with repeatable run overrides
persona: Bruno
journey: J-01
expected: A repeatable `--runtime` flag accepts slash-safe provider/model values and id, type, or complexity selectors; dry-run echoes the ordered effective layers; an equivalent HTTP or UDS request resolves identically; and one mixed task batch can bind distinct provider, model, and reasoning triples without leaking a parent runtime into sub-loops.
entry_points: compozy loop run --runtime ... --dry-run -o json; POST /api/workspaces/:workspace_id/loops/:name/run over HTTP and UDS
qa_status: untested
bug_ids:
fix_status:
retest_status:
fix_commits:
evidence:
last_report:
overlaps: LP-003; TA-079
---

story: As a delivery operator I can choose the right runtime for each task without changing the loop definition.

QA impact 2026-07-26: added by Compozy migration Task 06. Flag only; the next QA cycle owns real-user validation.

src: .compozy/tasks/compozy-migration/task_06.md
