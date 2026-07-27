---
id: LP-loop-run-deep-link
area: LP
title: Follow the created Loop run from CLI output
persona: Bruno
journey: J-01
expected: A successful non-dry `compozy loop run` prints the effective daemon URL for `/loop-runs/<run_id>` as its final human-readable line and returns the same `web_url` in JSON and TOON; the URL opens the matching persisted run. Dry-run emits no URL in any format.
entry_points: compozy loop run; compozy loop run --dry-run; web /loop-runs/:run_id; POST /api/workspaces/:workspace_id/loops/:name/run over HTTP and UDS
qa_status: untested
bug_ids:
fix_status:
retest_status:
fix_commits:
evidence:
last_report:
overlaps: LP-003; LP-005
---

story: As a delivery operator I can move directly from a successful CLI submission to the exact web run that was created.

QA impact 2026-07-27: added by Compozy migration Task 09. Flag only; the next QA cycle owns real-user and rendered-route validation.

src: .compozy/tasks/compozy-migration/task_09.md
