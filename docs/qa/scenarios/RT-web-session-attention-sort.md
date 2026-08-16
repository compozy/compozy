---
id: RT-web-session-attention-sort
area: RT
title: Bring sessions needing attention forward without losing selection
persona: Théo
journey: J-respond-to-agent-attention
expected: Choosing Attention first uses daemon ordering to place auth, input, and failed sessions ahead of other work with stable ties, preserves keyboard selection through live transitions, renders unknown honestly, and persists the global sort without overlapping full-section writes.
entry_points: web Sessions catalog sort menu; web session-window sidebar sort menu
qa_status: untested
bug_ids:
fix_status:
retest_status:
fix_commits:
evidence:
last_report:
overlaps: RT-session-attention-catalog
---

Walk keyboard and pointer selection while sessions enter and leave the needs-you class, including
equal timestamps and an unreporting session. Every badge must carry a distinct glyph and accessible
label; color alone is not an acceptable signal.

QA impact 2026-08-16: Task 03 added the daemon-backed attention-first sort and unified badge
dictionary. Flag only; Task 08 owns the real-user walk and evidence.
