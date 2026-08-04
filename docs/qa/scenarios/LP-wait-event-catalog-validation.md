---
id: LP-wait-event-catalog-validation
area: LP
title: Reject a wait event outside the hook catalog before execution
persona: Bruno
journey: J-improve-loop-with-feedback
expected: `compozy loop validate` rejects a wait whose event kind is outside the supported hook catalog, identifies the authored node, returns `watch_events_kind_unknown`, and does not require starting a run to discover the error.
entry_points: compozy loop validate; POST /api/workspaces/:workspace_id/loops/validate over HTTP and UDS
qa_status: pass
bug_ids: BUG-20260803-wait-event-rejected-too-late
fix_status: fixed
retest_status: pass
fix_commits: Task 08 checkpoint
evidence: /Users/pedronauck/dev/qa-labs/compozy-loop-operator-lifecycle-ui-20260803-044343-123901-lab/qa-artifacts/qa/screenshots/task08/28-invalid-wait-event-validation.json
last_report: docs/qa/reports/2026-08-03-loop-node-lifecycle-task08.md
overlaps: LP-runtime-validation-preflight
---

story: As a Loop builder, I learn that an event wait cannot be executed before I publish or start the Loop, with a diagnostic that points to the exact node and unsupported kind.

Task 08 walked this through the public CLI against the rebuilt isolated daemon. The invalid `qa.acknowledged` event returned `valid: false` and the catalog-specific error before any run was created.

src: .compozy/tasks/loop-node-lifecycle/task_08.md
