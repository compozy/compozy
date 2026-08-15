---
id: LP-web-run-page-section-grammar
area: LP
title: Run page sections, parked regions, and terminal beats follow the state contract
persona: Dora
journey: J-05
expected: Every main-column section is a collapsible `LoopSection` with its fixed icon (bell, gauge, activity, history, hourglass, triangle-alert, circle-slash) and a truthful count gist. "Happening now" holds only working lanes — waits render once in Waiting, and a quarantined lane surfaces in a neutral Needs-you panel (warning glyph the only color) with an "Open entry" row, shown for needs-approval or any quarantine. Older generations fold behind `Generation N · n events` and the best-result anchor forces its group open. A canceled run shows a circle-slash "Run canceled" story beat; a killed run shows a danger "Run killed" beat. The progress bar renders quarantined segments in danger tint distinct from info-tinted parked. Approval waits show the real escalation cursor strip (`step N` + next stamp) and promote Decide; event waits state their expect condition in the micro trail and promote "Resume with payload…" when intervention is required. The subhead states origin and Round N of M; the breadcrumb includes the loop; rail counts deep-link into the inventory scoped to the run; two or more attention flags render as a two-up cardpair with an inventory link; Inspect exists only in the rail foot.
entry_points: web /loop-runs/:runId
qa_status: blocked-verify
bug_ids:
fix_status:
retest_status:
fix_commits: e49a9abe
evidence:
last_report:
overlaps: LP-loop-run-deep-link; TA-loop-failure-breaker
---

Added by the loops visual-contract parity pass (2026-08-14). Walk needs a run driven through quarantine, waits, attention flags, cancel, and kill; deferred to the next seeded QA cycle — 99 targeted run-page tests plus the lifecycle story matrix (including the new ParkedProgress mixed state) are green at 9a694ff2.
