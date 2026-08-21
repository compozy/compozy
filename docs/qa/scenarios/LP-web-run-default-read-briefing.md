---
id: LP-web-run-default-read-briefing
area: LP
title: Absorb a run's state from the default read without opening anything
persona: Lea
journey: J-complete-partial-loop
expected: The run page's default read is exactly four elements in order — briefing strip (served verdict tone, plain headline, detail), needs-you cards, "step N of M" progress with fan-out rollups and attempts as metadata, and the narrated durable story — plus the Usage rail (tokens · cost · budget · rounds · duration) and About. Failure and needs-you never collapse. The briefing strip carries no decision buttons: the needs-you card owns Approve/Reject as the page's only primary, and the strip's quiet "Review the request" only leads to it. No `loop.` or `looprun-` id appears anywhere in the main column; the run id renders only as the About rail's labelled Run row. A terminal run leads with its outcome and produced artifacts, a pruned artifact keeps its name with a "Content no longer stored" note, and a no-op run says plainly that it produced nothing.
entry_points: web /loop-runs/:id; GET /loop-runs/:id/briefing; GET /loop-runs/:id/nodes; GET /loop-runs/:id/timeline
qa_status: untested
bug_ids: BUG-20260719-autonomous-progress-unobservable
fix_status: pending
retest_status:
fix_commits:
evidence:
last_report:
overlaps: LP-run-detail-story-redesign;LP-web-run-page-section-grammar;LP-fanout-progress-naming
---

story: As a supervisor I open any run and understand what is running, what needs me, how far along it is, what it has spent, and what it produced — in under thirty seconds, in plain words, without runtime literacy and without opening a single disclosure.

steps:
1. Start a loop run that reaches a human approval gate and open `/loop-runs/<id>` with every disclosure collapsed.
2. Read the briefing strip, the needs-you card, the progress line and the story without expanding anything.
3. Confirm the run id appears only in the About rail, never in the main column.
4. Confirm the briefing offers no Approve/Reject — only the quiet pointer to the card.
5. Let the run finish, reload, and confirm the outcome and artifacts lead the page.
6. Repeat against a failed run and confirm the failure signal is visible with everything collapsed.
