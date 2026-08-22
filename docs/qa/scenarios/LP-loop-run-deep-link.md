---
id: LP-loop-run-deep-link
area: LP
title: Follow the created Loop run from CLI output
persona: Bruno
journey: J-01
expected: A successful non-dry `compozy loop run` prints the effective daemon URL for `/loop-runs/<run_id>` as its final human-readable line and returns the same `web_url` in JSON and TOON; the URL opens the matching persisted run. Dry-run emits no URL in any format.
entry_points: compozy loop run; compozy loop run --dry-run; web /loop-runs/:run_id; POST /api/workspaces/:workspace_id/loops/:name/run over HTTP and UDS
qa_status: pass
bug_ids:
fix_status:
retest_status: pass
fix_commits:
evidence: /Users/pedronauck/dev/qa-labs/compozy-loop-task-legibility-runtime-20260821-1126-20260821-112711-004724-lab/qa-artifacts/qa/headless/authoring-human.txt; /Users/pedronauck/dev/qa-labs/compozy-loop-task-legibility-runtime-20260821-1126-20260821-112711-004724-lab/qa-artifacts/qa/headless/authoring-toon.txt; /Users/pedronauck/dev/qa-labs/compozy-loop-task-legibility-runtime-20260821-1126-20260821-112711-004724-lab/qa-artifacts/qa/headless/authoring-dry-json.json; /Users/pedronauck/dev/qa-labs/compozy-loop-task-legibility-task07-final-web-20260822-131622-550786-lab/qa-artifacts/qa/task07-scenario-walks.md; /Users/pedronauck/dev/qa-labs/compozy-loop-task-legibility-task07-final-web-20260822-131622-550786-lab/qa-artifacts/qa/screenshots/loop-run-runtime-provenance.png
last_report: docs/qa/reports/2026-08-21-loop-task-legibility.md
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

QA result 2026-08-21: the effective daemon URL was the final human line and the structured
`web_url` in JSON and TOON. Human, JSON, and TOON dry-runs emitted no run URL. Rendered-page visual
parity remains owned by the concurrent visual-contract phase.

QA result 2026-08-22: the final daemon-served Web lane reopened the persisted destination and
rendered its runtime provenance. Task 05's complete Visual Contract matrix closes the previously
pending rendered-page parity leg.
