---
id: ET-web-agent-detail-tab-parity
area: ET
title: Agent detail tab panelbox and content contracts
persona: Bruno
journey: J-31
expected: Overview Setup panel lists Model (live selector) and Permissions in plain language (no Command row, no At a glance rail); Instructions AGENT.md renders markdown prose without a meta strip and SOUL/HEARTBEAT tabs carry no missing-file warning pills; Configuration Runtime shows "Defined in" with layer/override provenance, Access lists Allowed/Blocked tools and Tool groups as neutral pills, and MCP uses hairline rows; Sessions empty New session opens the launch dialog and, after creation, navigates through the created session owner workspace to its composer.
entry_points: web /agents/$name?tab=overview|instructions|configuration|sessions
qa_status: untested
bug_ids:
fix_status:
retest_status:
fix_commits:
evidence: /Users/pedronauck/dev/qa-labs/compozy-qa-et-current-source-20260730-061655-910372-lab/qa-artifacts/qa;docs/qa/evidence/2026-07-30-session-runtime-selector/09-agent-detail-sessions.png;docs/qa/evidence/2026-07-30-session-runtime-selector/runtime-selector-proof.md;docs/qa/evidence/2026-08-01-loops-paper-adoption/session-create-dialog-narrow.png;docs/qa/evidence/2026-08-01-loops-paper-adoption/session-create-dialog-desktop.png
last_report: docs/qa/reports/2026-08-01-loops-paper-adoption.md
overlaps: RT-agent-overview-canonical-metrics; RT-076
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

QA impact 2026-08-01: session-create focus and workspace ownership changed during adjacent Loop QA.
The scenario was reset to untested, then re-walked through the official Web E2E lane: same-workspace
creation navigated to its composer, and E2E-022 created a session in a second workspace and opened
that workspace/session route. Narrow and desktop Storybook captures verified the workspace selector,
runtime deferral notice, and reachable Start session action. Status returned to pass.

QA impact 2026-09-17 (issue #653): the Instructions tab renders AGENT.md through `DescriptionCard`,
which now takes the reading-tier prose ladder (H1 22 px, H2 18 px, H3 16 px, heading top margins up to
32 px), semibold emphasis, accent-strong links, and framed tables. Panel geometry and the tab contracts
are untouched. Not walked: the Instructions tab in a live runtime; confirm a long AGENT.md still reads
comfortably inside the panelbox and that no horizontal overflow appears.

QA impact 2026-09-28 (ui-normie-pass): Overview drops the At a glance rail and the Command row
(Command stays in Configuration), panels move from `AgentPanelBox` to the shared `Panel`, permission
values render through one plain vocabulary ("Ask before every action" / "Ask only before changes" /
"Never ask" / "Use the provider's setting"), the Sessions tab drops its duplicate stats grid in favour of
filter counts and loses the Iterations column, and a failed session list offers Retry. Status reset to
untested; not walked.
