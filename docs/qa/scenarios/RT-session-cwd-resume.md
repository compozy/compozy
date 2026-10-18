---
id: RT-session-cwd-resume
area: RT
title: Preserve a session working directory across launch and resume
persona: Théo
journey: J-11
expected: A session created with a valid working directory below its workspace launches in that directory, persists the choice, and resumes in the same directory without escaping the workspace boundary.
entry_points: session create CWD; daemon session reactivation; provider process launch
qa_status: skipped
bug_ids:
fix_status:
retest_status:
fix_commits:
evidence: /Users/pedronauck/dev/qa-labs/compozy-qa-rt-current-source-20260730-20260730-061631-252740-lab/qa-artifacts/qa
last_report: docs/qa/reports/2026-10-02-untested.md
overlaps: RT-session-context-rebuild
---

A session created with a valid working directory below its workspace launches in that directory, persists the choice, and resumes in the same directory without escaping the workspace boundary.

Walk each listed public entry point, then reload and read the stored result independently. Exercise rejection and recovery with the same workspace and profile to confirm that unrelated state remains intact.

2026-09-27 scope update: retired product surfaces were removed from this active scenario. Historical evidence remains in the dated reports; this revised contract requires a fresh walk.

2026-10-05: Deferred from this QA cycle by the user's explicit scope reduction.
Coverage remains outstanding; this skip is not a passing result. Resume from the dated
report's session matrix in a future QA cycle.
