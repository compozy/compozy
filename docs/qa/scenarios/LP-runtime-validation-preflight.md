---
id: LP-runtime-validation-preflight
area: LP
title: Reject invalid runtime selections before provider spawn
persona: Dora
journey: J-02
expected: Static loop validation reports definition-owned runtime errors, while dry-run and submission report effective workspace/run errors as deterministic `runtime_validation` items; authoritative provider catalogs reject unknown models, non-authoritative providers accept arbitrary model IDs, missing bound secrets fail before ACP spawn, and every rejected path materializes no session.
entry_points: compozy loop validate; compozy loop run --dry-run; POST /api/workspaces/:workspace_id/loops/:name/validate and /run over HTTP and UDS
qa_status: untested
bug_ids:
fix_status:
retest_status:
fix_commits:
evidence:
last_report:
overlaps: LP-022; LP-runtime-selection-overrides
---

story: As an operator I receive actionable runtime diagnostics before any invalid provider process starts.

QA impact 2026-07-26: added by Compozy migration Task 06. Flag only; the next QA cycle owns adversarial preflight validation.

src: .compozy/tasks/compozy-migration/task_06.md
