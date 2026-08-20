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
evidence: /Users/pedronauck/dev/qa-labs/compozy-northstar-pay-20260729-021949-664736-lab/qa-artifacts/qa/evidence/043-agent-fleet-listing
last_report: docs/qa/reports/2026-07-28-untested-full.md
overlaps: RT-agent-overview-canonical-metrics
---

Added by agent-detail tab parity follow-up 2026-07-17 after aligning fleet rows to the shared listing pattern and OpenDesign agents-list.html.

QA 2026-07-29: The live workspace catalog rendered the same ten unique agents in Rows and Cards.
Rows used one ListingRow each with one origin pill, monospace provider/model facts, status and
new-session controls confined to the trail, no sessions Stat, and no provider pill. Cards used plain
CatalogCard.Meta spans and retained the same agents and origins. Returning to Rows lost no data and
the browser reported no console or page errors.

QA impact 2026-08-20: Agents catalog SearchInput height now uses `--height-search` (28px) to match
Filter and Rows/Cards. Reset the listing chrome walk.
