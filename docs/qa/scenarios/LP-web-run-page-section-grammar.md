---
id: LP-web-run-page-section-grammar
area: LP
title: Run page sections, parked regions, and terminal beats follow the state contract
persona: Dora
journey: J-recover-loop-node-failure
expected: Every main-column section is a collapsible `LoopSection` with its fixed icon (bell, gauge, activity, history, hourglass, triangle-alert, circle-slash) and a truthful count gist. "Happening now" holds only working lanes — waits render once in Waiting, and a quarantined lane surfaces in a neutral Needs-you panel (warning glyph the only color) with an "Open entry" row, shown for needs-approval or any quarantine. Older generations fold behind `Generation N · n events` and the best-result anchor forces its group open. A canceled run shows one circle-slash "Run canceled" story beat. The progress bar renders quarantined segments in danger tint distinct from info-tinted parked. Approval waits show the real escalation cursor strip (`step N` + next stamp) and promote Decide; event waits state their expect condition in the micro trail and promote "Resume with payload…" when intervention is required. Origin, rounds, run id, and elapsed time live only in the rail (Usage + About this run); the breadcrumb includes the loop; rail counts deep-link into the inventory scoped to the run; two or more attention flags render as a two-up cardpair with an inventory link; Inspect exists only in the rail foot.
entry_points: web /loop-runs/:runId
qa_status: pass
bug_ids:
fix_status:
retest_status:
fix_commits: e49a9abe
evidence: /Users/pedronauck/dev/qa-labs/compozy-issue-500-forced-loop-cancel-20260831-195541-194552-lab/qa-artifacts/qa/evidence/web-canceled-terminal.png
last_report: docs/qa/reports/2026-08-31-issue-500-forced-loop-cancel.md
overlaps: LP-forced-cancel-owned-sessions; LP-loop-run-deep-link; TA-loop-failure-breaker
---

Added by the loops visual-contract parity pass (2026-08-14). The original walk required distinct
cancel and Kill beats; that requirement is historical after the 2026-08-31 hard cut.

QA impact 2026-08-31: the distinct killed beat was removed. Re-walk the unified canceled beat and
verify Kill is absent from the run page.
