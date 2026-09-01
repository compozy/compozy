---
id: LP-operator-lifecycle-ui
area: LP
title: Operate node lifecycle from the run page and the workspace node inventories
persona: Bruno
journey: J-recover-loop-node-failure
expected: The run detail renders node lifecycle state from daemon payloads only — attempt and next-attempt time on a retrying lane, pause provenance (who, why, when, manual vs rule) on a paused lane, wait kind and age on a waiting lane, the quarantine set-aside and its attention overlay, cancel-state on a winding-down lane, and the `canceled` terminal as a calm "Why it stopped" with no Happening-now card. Node row menus offer exactly the verbs the payload declares (running/retrying → pause, cancel; paused → the three resume modes + cancel; waiting → resume-with-payload + cancel; quarantined → open entry, requeue + cancel; terminal run → none). Pause asks drain-vs-cancel; destructive Cancel and Requeue confirm against the current state; the daemon's 409/422 answers render as information carrying actual_state, allowed_transitions, and the winning actor. The quarantine sheet shows hint-first, the classified attempt chain with episode boundaries, target, and input ref, and hides Requeue once the node leaves quarantine. Run Cancel is the destructive header action and Kill is absent. The workspace inventories (waiting/quarantined/attention/retrying) filter by loop and run, sort by real time in state, page only while a cursor exists, and never present a loaded page as a total.
entry_points: web /loop-runs/:id; web /loop-runs?nodes=waiting|quarantined|attention|retrying; GET /loop-nodes?state=; POST /loop-runs/:id/nodes/:node/{pause,resume,cancel,requeue}; POST /loop-runs/:id/cancel; SSE /loop-runs/:id/events
qa_status: pass
bug_ids:
fix_status:
retest_status:
fix_commits:
evidence: /Users/pedronauck/dev/qa-labs/compozy-loop-operator-lifecycle-ui-20260803-044343-123901-lab/qa-artifacts/qa/screenshots/task08/03b-waiting-run-zoomed.png;/Users/pedronauck/dev/qa-labs/compozy-loop-operator-lifecycle-ui-20260803-044343-123901-lab/qa-artifacts/qa/screenshots/task08/05-wait-resumed-fresh.png;/Users/pedronauck/dev/qa-labs/compozy-loop-operator-lifecycle-ui-20260803-044343-123901-lab/qa-artifacts/qa/screenshots/task08/07-quarantine-entry.png;/Users/pedronauck/dev/qa-labs/compozy-loop-operator-lifecycle-ui-20260803-044343-123901-lab/qa-artifacts/qa/screenshots/task08/09-requeue-fresh.png;/Users/pedronauck/dev/qa-labs/compozy-loop-operator-lifecycle-ui-20260803-044343-123901-lab/qa-artifacts/qa/screenshots/task08/17-run-killed-terminal.png;/Users/pedronauck/dev/qa-labs/compozy-loop-operator-lifecycle-ui-20260803-044343-123901-lab/qa-artifacts/qa/screenshots/task08/23-retrying-run.png;/Users/pedronauck/dev/qa-labs/compozy-loop-operator-lifecycle-ui-20260803-044343-123901-lab/qa-artifacts/qa/screenshots/task08/25-paused-retrying-node.png;/Users/pedronauck/dev/qa-labs/compozy-loop-operator-lifecycle-ui-20260803-044343-123901-lab/qa-artifacts/qa/screenshots/task08/26-resumed-retrying-node.png;/Users/pedronauck/dev/qa-labs/compozy-loop-operator-lifecycle-ui-20260803-044343-123901-lab/qa-artifacts/qa/screenshots/task08/27-retrying-inventory.png
last_report: docs/qa/reports/2026-08-31-issue-500-forced-loop-cancel.md
overlaps: LP-forced-cancel-owned-sessions; LP-live-pause-repair-resume; LP-quarantine-diagnose-requeue; LP-waiting-inventory-escalation; LP-run-detail-story-redesign
---

story: As an operator I pause a wedged lane while the rest of the run keeps working, read why a lane was set aside and requeue it after repair, and find everything parked across the workspace — without the UI ever offering me a verb the runtime would reject.

design: docs/design/opendesign/loops/loop-run-detail.html + loop-run-detail-states.html + loop-node-controls.html + loop-quarantine-sheet.html + loop-inventories.html

truthful-ui: every lifecycle state traces to `node_controls[]`, `waits[]`, or a generation output field; verbs come from `loopNodeVerbs`, a pure function over that payload; the inventory route publishes no totals, so no count badge or "N of M" renders (SD-007 + eng-data-boundaries catalog contract).

evidence-seed: visual-contract bundles at .compozy/tasks/loop-node-lifecycle/evidence/visual/task_08/vc-r1..vc-r5 (retrying / paused / node controls / quarantine sheet / waiting inventory vs the canonical prototypes); Vitest WT-001..WT-004; Playwright E2E-015.

qa-plan: Task 08 owns the behavior-first browser walk before completion; Task 13 may re-run it as
part of the final release scenario pass.

src: .compozy/tasks/loop-node-lifecycle/task_08.md

QA result 2026-08-03: passed in an isolated live lab. The same public run payload drove a real timer wait and payload resume, a timeout retry with attempt and next-attempt time, drain pause with provenance, resume with the preserved schedule, quarantine inspection and requeue, workspace inventories, run kill, fresh-load/deep-link durability, keyboard navigation, 768px layout, and recoverable inventory failure. No control remained visible after its daemon-declared transition disappeared.

acceptance-walk: Re-run the lifecycle story in the isolated build with the browser driver: retry, pause/resume, wait, quarantine/requeue, and forced Cancel from the run detail plus every workspace inventory. Confirm Kill is absent, capture checkpoint and divergence screenshots, refresh each state, and compare the payload with structured CLI and HTTP reads before accepting the UI.
