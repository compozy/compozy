---
id: NB-execution-participation-defaults
area: NB
title: Default execution owners to Local participation
persona: Ada
journey: J-network-local-default
expected: A plain session create, task run, Loop run, or task-backed automation fire persists one immutable `Local`/`built_in_local` snapshot, creates no Network channel or wake, exposes no Network prompt, environment, or coordination tools, and records zero Network usage. Spawn, review, and detached child sessions resolve independently and never inherit a parent conversation.
entry_points: web session/task/Loop/automation create, edit, and start surfaces; HTTP/UDS/CLI/native owner create/start verbs; schedule/webhook/trigger automation fire; Network channel catalog and usage reads
qa_status: pass
bug_ids: BUG-20260715-loop-participation-contract-dropped;BUG-20260715-automation-task-participation-control-missing;BUG-20260715-loop-run-compact-layout-collapsed
fix_status: fixed
retest_status: pass
fix_commits:
evidence: docs/qa/evidence/2026-07-14-network-changes/ch-network-local-default.md
last_report: docs/qa/reports/2026-07-14-network-changes.md
overlaps: NB-006;TA-001;TA-004;TA-052
---

Planning flag for execution-owner participation. The next targeted QA cycle should compare the channel catalog and Network usage before and after each plain create/start path, inspect the persisted owner projection and provider prompt/environment/toolset, and prove that a child session remains Local even when its parent is Live.

The scenario does not own bounded Live delivery or workspace-coordination invitation behavior; those are settled by `NB-run-bounded-live-collaboration` and `NB-coordination-invitation-future-runs`.
