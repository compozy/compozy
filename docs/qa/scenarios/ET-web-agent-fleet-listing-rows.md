---
id: ET-web-agent-fleet-listing-rows
area: ET
title: Agent fleet listing rows match shared ListingRow grammar
persona: Bruno
journey: J-31
expected: Agents list rows use ListingRow anatomy like loops/skills — title plus origin pill, meta as category · provider · model mono facts, trail status/invalid/new-session without a duplicate sessions Stat or provider pill in the name line; cards use plain CatalogCard.Meta spans. Under All projects the list reads `/api/agents/aggregate` (User agents plus every project's for the acting profile): same-named agents of different projects are distinct rows, each with a project badge leading the meta line (User for user-layer agents) instead of the origin pill, the toolbar adds the shared Project filter, All profiles shows "Definitions for {profile}", and opening a row opens the agent in place bound to its project (the desktop stays All projects; tab and settings navigation keep the binding). The origin pill reads User for user-layer agents, never All projects.
entry_points: web /agents
qa_status: untested
bug_ids:
fix_status: none
retest_status: untested
fix_commits:
evidence: /Users/pedronauck/dev/qa-labs/compozy-northstar-pay-20260729-021949-664736-lab/qa-artifacts/qa/evidence/043-agent-fleet-listing
last_report: docs/qa/reports/2026-10-02-untested.md
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

2026-10-05: Deferred from this QA cycle by the user's explicit scope reduction.
Coverage remains outstanding; this skip is not a passing result. Resume from the dated
report's session matrix in a future QA cycle.


All-projects A4 backend coverage (2026-10-21): For the backend aggregate, create a same-named agent in User and two projects. Page `/api/agents/aggregate?all_workspaces=true&name=<name>&limit=1`: expect three distinct `(name, workspace_id)` rows, stable continuation, and owner-specific session metrics.
Targeted evidence: `docs/qa/reports/2026-10-21-all-projects-user-scope.md`. The integrated Web walk remains owned by the orchestration QA slice.

All-projects AW4 (2026-10-21): reset to untested for the Web half of the A4 aggregate. Walk: same-named agent in User and two projects → three rows with badges; Project filter to one project; open the ws_b row → the agent window shows the ws_b chrome badge and its sessions; switch to Sessions tab and Settings and back — still ws_b. PR screenshots: aggregate agents list with badges; bound agent window.

QA impact 2026-11-01 (stays-on F2): the fleet, the four tabs and the settings modal are gone, replaced
by the team list and the agent's home. See `ET-web-agents-team-profile`; old `?tab=` and
`settings?section=` links land on the matching panel or page. Status remains untested; no QA replay
ran for this scenario.
