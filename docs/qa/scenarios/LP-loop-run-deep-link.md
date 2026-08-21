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
retest_status: pass
fix_commits:
evidence: /Users/pedronauck/dev/qa-labs/compozy-compozy-migration-beta-20260727-135201-116083-lab/qa-artifacts/qa/cross-surface/web-task-detail.png; /Users/pedronauck/dev/qa-labs/compozy-northstar-pay-20260801-135009-390014-lab/qa-artifacts/qa/loop/browser-url-v6.txt; /Users/pedronauck/dev/qa-labs/compozy-northstar-pay-20260801-135009-390014-lab/qa-artifacts/qa/loop/loop-run-v6-full.png; /Users/pedronauck/dev/qa-labs/compozy-northstar-pay-20260801-135009-390014-lab/qa-artifacts/qa/loop/same-run-parity-v6.json; /Users/pedronauck/dev/qa-labs/compozy-loop-operator-lifecycle-ui-20260803-044343-123901-lab/qa-artifacts/qa/screenshots/task08/15b-cora-deep-link-stable.png
last_report: docs/qa/reports/2026-08-03-loop-node-lifecycle-task08.md
overlaps: LP-003; LP-005
---

story: As a delivery operator I can move directly from a successful CLI submission to the exact web run that was created.

QA impact 2026-07-27: added by Compozy migration Task 09. Flag only; the next QA cycle owns real-user and rendered-route validation.

src: .compozy/tasks/compozy-migration/task_09.md

QA impact 2026-08-01: reset to `untested` because the run detail payload and rendered timeline now
include scored verdicts, best-generation links, parent generation, and origin. Re-open the CLI URL
after restart and confirm those persisted fields agree with structured reads without duplicate
generation anchors; historical deep-link evidence remains recorded in frontmatter.

QA result 2026-08-03: the exact persisted run deep link reopened after live layout synchronization and rendered the matching run id, durable terminal state, and run story on a fresh browser read.
