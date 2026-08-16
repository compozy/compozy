---
id: RT-web-attention-title-count
area: RT
title: Keep the tab title's needs-you count exact
persona: Théo
journey: J-respond-to-agent-attention
expected: While the tab is visible or backgrounded, its title shows the exact cross-workspace needs-you summary, excludes Finished work, survives route changes, clears at zero, and never displays a stale source as current.
entry_points: browser tab title; web route navigation
qa_status: untested
bug_ids:
fix_status:
retest_status:
fix_commits:
evidence:
last_report:
overlaps: RT-session-attention-catalog; RT-web-attention-bell-jump
---

Exercise counts above the menubar's 9+ cap, route navigation, background updates, stream loss, and a
return to zero. The base product title must be restored without accumulating repeated count prefixes.

QA impact 2026-08-16: Task 03 added the summary-fed document title channel. Flag only; Task 08 owns
the real-user walk and evidence.
