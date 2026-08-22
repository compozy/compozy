---
id: TA-web-task-detail-loop-provenance
area: TA
title: A Loop execution record's detail page names its run and links back to it
persona: Dora
journey: J-supervise-loop-steady-state
expected: Opening a coordinator or cell record's task detail — by deep link, not only from the list — leads the properties rail with a block headed "Loop run" or "Loop step" carrying the loop name, round, step and item in plain words plus an "Open run" link that lands on that run's page; a record whose run retention deleted keeps the same block and its run id but reads "Run no longer available" with no link to follow; no field on the page is recovered by parsing the task id.
entry_points: web Tasks window -> List (reveal on) -> record; direct URL /tasks/<record-id>
qa_status: pass
bug_ids:
fix_status:
retest_status: pass
fix_commits:
evidence: /Users/pedronauck/dev/qa-labs/compozy-loop-task-legibility-task07-final-web-20260822-131622-550786-lab/qa-artifacts/qa/task07-scenario-walks.md; /Users/pedronauck/dev/qa-labs/compozy-loop-task-legibility-task07-final-web-20260822-131622-550786-lab/qa-artifacts/qa/screenshots/tasks-loop-detail-provenance.png; /Users/pedronauck/dev/qa-labs/compozy-loop-task-legibility-task07-final-web-20260822-131622-550786-lab/qa-artifacts/qa/screenshots/tasks-loop-detail-run-gone.png
last_report: docs/qa/reports/2026-08-21-loop-task-legibility.md
overlaps: TA-web-tasks-calm-default-reveal; TA-task-list-calm-loop-default
---

story: As an operator who followed a link into a loop's execution record I need to know what the record is and get back to the run that owns it, without guessing from an id.

The run page is where loop work is acted on; this page is the mechanism view, so the link back is mandatory and must work on a terminal run as well as a live one. Walk it by deep link with a cold cache — the single-task read carries its own provenance, so the page must not depend on having arrived from a list.

Provenance is projected fields end to end. The web no longer parses `loop.<run>.g<N>.node.<id>.<item>`; if a field is absent the page omits it rather than reconstructing it, and an unrecoverable loop name is what turns the run link into the truthful degrade.

src: web/src/systems/tasks/components/task-loop-provenance.tsx; web/src/systems/tasks/components/task-properties-rail.tsx; web/src/systems/tasks/lib/task-loop-identity.ts

inventory: Needs QA
