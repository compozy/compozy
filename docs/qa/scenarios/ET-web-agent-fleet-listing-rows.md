---
id: ET-web-agent-fleet-listing-rows
area: ET
title: Agent fleet listing rows match shared ListingRow grammar
persona: Bruno
journey: J-31
expected: Agents list rows use ListingRow anatomy like loops/skills — title plus origin pill, meta as category · provider · model mono facts, trail status/invalid/new-session without a duplicate sessions Stat or provider pill in the name line; cards use plain CatalogCard.Meta spans.
entry_points: web /agents
qa_status: untested
bug_ids:
fix_status:
retest_status:
fix_commits:
evidence:
last_report:
overlaps: RT-agent-overview-canonical-metrics
---

Added by agent-detail tab parity follow-up 2026-07-17 after aligning fleet rows to the shared listing pattern and OpenDesign agents-list.html.
