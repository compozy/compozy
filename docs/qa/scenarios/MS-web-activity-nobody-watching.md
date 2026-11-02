---
id: MS-web-activity-nobody-watching
area: MS
title: The Activity page shows what ran with nobody watching and who started it
persona: Dora
journey: J-31
expected: From Dashboard, See everything opens Activity. Nobody watching shows only automation, check-in and extension work; Everything adds yours. Totals and daily bars come from the rollups, and the log names who started each action. Rows from before the upgrade read Unknown, never a guess. With nothing unattended the page says so.
entry_points: web Dashboard → Activity → See everything; `/activity`
qa_status: skipped
bug_ids:
fix_status:
retest_status:
fix_commits:
evidence:
last_report:
overlaps: ET-native-tool-approval-grants
---

Decide a permission by hand, then let an automation trigger one. Compare the page's totals with
`compozy observe activity --days 14 --unattended -o json`, and search the log by agent and by file.
The log is the newest 1,000 decisions; past that the section says Latest 1,000 and the totals stay
exact.

QA impact 2026-11-01 (stays-on F3): Focused suites own the states. Not walked.
