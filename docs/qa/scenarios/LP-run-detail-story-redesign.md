---
id: LP-run-detail-story-redesign
area: LP
title: Read a live run as a plain-language story on the redesigned run detail
persona: Lea
journey: J-complete-partial-loop
expected: The run detail uses the materialized contract for its plain-language Progress story, shows bounded Goal criterion diagnostics and warnings in the turn timeline, and keeps the raw executed definition plus every operator fact reachable through Inspect.
entry_points: web /loop-runs/:id; GET /loop-runs/:id; SSE /loop-runs/:id/events; topbar ⋯ Inspect
qa_status: untested
bug_ids:
fix_status:
retest_status:
fix_commits:
evidence: docs/qa/evidence/2026-08-10-loop-convergence/run-detail-goal-diagnostics.png; docs/qa/evidence/2026-08-10-loop-convergence/run-detail-inspect.png; docs/qa/evidence/2026-08-10-loop-convergence/raw-loop-definition.png; /Users/pedronauck/dev/qa-labs/compozy-loop-convergence-20260810-034845-371840-lab/qa-artifacts/qa/qa-audit-report.md
last_report: docs/qa/reports/2026-08-10-loop-convergence.md
overlaps: LP-009;LP-014;LP-016;LP-044;LP-action-failure-detail
---

story: As an end user I open a running loop and understand where it stands, what it is doing right now, what already happened, and what comes next — without operator vocabulary — and As a person running agent work I still reach every mechanical fact through Inspect.

design: docs/design/opendesign/loops/loop-run-detail.html + loop-run-detail-states.html (LOOP-RUN-REDESIGN-SPEC.md)

truthful-ui: every rendered value traces to a spec §4 field; cost always renders `~$` + `estimate`; the story derives only from replayed run events; no control renders for a transition the daemon rejects (§7).

evidence-seed: visual-contract bundles at .compozy/tasks/loop-run-redesign/evidence/visual/VC-01..05 (running / needs-approval / watching / paused / failed vs the canonical prototypes).

src: docs/design/opendesign/loops/LOOP-RUN-REDESIGN-SPEC.md

QA impact 2026-08-10: reset because Progress now consumes `materialized_contract` and the Goal turn
timeline exposes durable command output, blockers, and warnings. Inspect continues to own the raw
executed definition.

QA result 2026-08-10: Lea opened the completed Run in the Web app. Progress showed the materialized
Goal and definition of done, the expanded timeline showed both command verdicts and their durable
diagnostics, and Inspect exposed the raw authored definition and runtime facts. The browser console
reported no errors.

reset: the story timeline gains the graph-completion row families — request lifecycle, route taken, branch pruned, node amended, run forked (.compozy/tasks/graph-eng/task_08.md). The recorded pass predates them.
