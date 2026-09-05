---
id: LP-web-run-default-read-briefing
area: LP
title: Absorb a run's state from the default read without opening anything
persona: Lea
journey: J-supervise-loop-steady-state
expected: The run page's default read is exactly four elements in order — briefing strip (served verdict tone, plain headline, detail), needs-you cards, "step N of M" progress with fan-out rollups and attempts as metadata, and the narrated durable story — plus the Usage rail (tokens · cost · budget · rounds · duration) and About. Failure and needs-you never collapse. The briefing strip carries no decision buttons: the needs-you card owns Approve/Reject as the page's only primary, and the strip's quiet "Review the request" only leads to it. No `loop.` or `looprun-` id appears anywhere in the main column; the run id renders only as the About rail's labelled Run row. A terminal run leads with its outcome and produced artifacts, a pruned artifact keeps its name with a "Content no longer stored" note, and a no-op run says plainly that it produced nothing.
entry_points: web /loop-runs/:id; GET /loop-runs/:id/briefing; GET /loop-runs/:id/nodes; GET /loop-runs/:id/timeline
qa_status: pass
bug_ids: BUG-20260719-autonomous-progress-unobservable
fix_status: fixed
retest_status: pass
fix_commits:
evidence: /Users/pedronauck/dev/qa-labs/compozy-loop-task-legibility-task07-final-web-20260822-131622-550786-lab/qa-artifacts/qa/task07-scenario-walks.md; .compozy/tasks/loop-task-legibility/evidence/visual/task_05/VC-01; .compozy/tasks/loop-task-legibility/evidence/visual/task_05/VC-16
last_report: docs/qa/reports/2026-08-21-loop-task-legibility.md
overlaps: LP-run-detail-story-redesign;LP-web-run-page-section-grammar;LP-fanout-progress-naming;LP-web-runs-roster-rerank;LP-web-strategy-progress
---

story: As a supervisor I open any run and understand what is running, what needs me, how far along it is, what it has spent, and what it produced — in under thirty seconds, in plain words, without runtime literacy and without opening a single disclosure.

QA dependency update 2026-08-21: BUG-20260719-autonomous-progress-unobservable is verified through
a fresh public-read observer replay. This Web row remains `untested` because the focused closure did
not walk its rendered briefing, needs-you, progress, story, Usage, or About contracts.

steps:
1. Start a loop run that reaches a human approval gate and open `/loop-runs/<run-id>` with every disclosure collapsed.
2. Read the briefing strip, the needs-you card, the progress line and the story without expanding anything.
3. Confirm the run id appears only in the About rail, never in the main column.
4. Confirm the briefing offers no Approve/Reject — only the quiet pointer to the card.
5. Let the run finish, reload, and confirm the outcome and artifacts lead the page.
6. Repeat against a failed run and confirm the failure signal is visible with everything collapsed.

QA result 2026-09-04: a real two-round built-in review/fix run exposed a 2,644-character JSON
headline and false retention warnings. The corrected default read has a 19-character outcome
headline and three previews of five actual action results, with latest-round results first.
Content and remaining results expand on demand; partial/pruned counts remain visible while folded.
About metadata starts collapsed and preserves its links, inputs, identity, and copy control when
opened. The final empty review and the earlier finding were both opened in Chrome after a daemon
restart. CLI and HTTP returned matching result identity, availability, and headline.
Evidence: `docs/qa/reports/2026-09-04-loop-stability.md`, cycle 2.
