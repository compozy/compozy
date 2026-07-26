---
id: ET-web-agent-detail-tab-parity
area: ET
title: Agent detail tab panelbox and content contracts
persona: Bruno
journey: J-31
expected: Overview Runtime lists Model (live selector), Command, and Permissions inside panelbox surfaces; At a glance shows MCP mono chips and Skills; Instructions AGENT.md shows file-meta Read-only here with markdown prose; Configuration Access deny tools use danger pills and MCP uses hairline rows; Sessions empty folds New session into Empty action.
entry_points: web /agents/$name?tab=overview|instructions|configuration|sessions
qa_status: untested
bug_ids:
fix_status:
retest_status:
fix_commits:
evidence:
last_report:
overlaps: RT-agent-overview-canonical-metrics;RT-076
---

Added by agent-detail OpenDesign tab parity 2026-07-17 after aligning Overview/Instructions/Configuration/Sessions composition to frozen agent-detail.html (SHA-1 4a4c214402cc83a06ff8ab7c607b9c0d6cfc12bc).

QA impact 2026-07-18: Configuration MCP servers now use the shared `ListingRow` primitive while
preserving transport and redacted environment-key metadata. Status remains untested; no QA replay
ran.

QA impact 2026-07-19: the detail-header runtime selector now surfaces failures from its active
provider source and exposes a source-specific retry action instead of remaining silently disabled.
Status remains untested; no QA replay ran.

QA impact 2026-07-22: the live runtime selector moved from the detail topbar into Overview Runtime
as the Model category above Command/Permissions. Status remains untested; no QA replay ran.
