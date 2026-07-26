---
id: RT-agent-overview-canonical-metrics
area: RT
title: Agent overview Active Runtime Failed Last activity
persona: Bruno
journey: J-31
expected: Overview metrics are Active, Runtime, Failed, and Last activity from workspace-scoped canonical data; Active uses server totals; Runtime/Failed show honest unavailable when the session page is incomplete and no daemon aggregate exists; never substitutes Total sessions or Resumable.
entry_points: web /agents/$name?tab=overview
qa_status: untested
bug_ids:
fix_status:
retest_status:
fix_commits:
evidence:
last_report:
overlaps: RT-076
---

Added by agent-details remediation 2026-07-12 after replacing Total sessions / Resumable with the approved metric contract.
