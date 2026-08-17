---
id: ET-agent-plugin-dev-reload
area: ET
title: Reload a dev-linked Agent Plugins package
persona: Bruno
journey: J-extension-dev-lifecycle
expected: A portable package dev-links without install trust, publishes its mapped skills and MCP servers, reloads changed package content through the existing workspace-scoped generation loop, retains its instance data, and leaves other workspaces and any published instance unchanged.
entry_points: compozy extension dev <path>|reload <name>|status <name> --workspace|remove <name>; POST /api/extensions/dev and POST /api/extensions/:name/reload over HTTP and UDS; compozy__extensions_dev|reload|info|remove
qa_status: pass
bug_ids:
fix_status:
retest_status: pass
fix_commits:
evidence: docs/qa/reports/2026-08-16-agent-plugins.md#session-debriefs
last_report: docs/qa/reports/2026-08-16-agent-plugins.md
overlaps: ET-extension-dev-reload-loop; ET-agent-plugin-data-removal
---

QA impact 2026-08-16: the established dev overlay lifecycle now accepts the Agent Plugins format.
Task 08 owns only the portable mapping and reload parity; the existing scenario remains canonical for
generic generation, last-good, logs, and workspace-isolation behavior.

QA 2026-08-16: the dev-linked portable package published its resources without install trust, reload
advanced only its workspace generation, retained instance data, and left the published instance and
the second workspace unchanged.
