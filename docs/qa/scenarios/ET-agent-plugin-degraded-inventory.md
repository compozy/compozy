---
id: ET-agent-plugin-degraded-inventory
area: ET
title: Inspect a partially ingested Agent Plugins package
persona: Ada
journey: J-extension-kit-lifecycle
expected: A package with valid skills and MCP servers plus invalid or unsupported siblings installs its usable components, retains ordered `extension_agent_plugin_component_skipped` diagnostics, distinguishes later `extension_mcp_server_unhealthy` live diagnostics, and never presents a fully skipped portable package as a native empty extension.
entry_points: compozy extension status|inventory|list -o human|json|jsonl|toon; GET /api/extensions, /api/extensions/:name, and /api/extensions/:name/inventory over HTTP and UDS; compozy__extensions_info|list|inventory; Web /marketplace/extension/:entryId?installed_name=:name
qa_status: pass
bug_ids: BUG-20260816-daemon-stop-timeout
fix_status: pending
retest_status: pass
fix_commits:
evidence: docs/qa/reports/2026-08-16-agent-plugins.md#session-debriefs
last_report: docs/qa/reports/2026-08-16-agent-plugins.md
overlaps: ET-ext-inventory; ET-web-extension-kit-inventory; ET-web-extension-detail
---

QA impact 2026-08-16: portable ingest diagnostics and live health now share the first-class extension
payload. Task 08 must prove their ordering, codes, persistence across restart, and visibility on all
listed read planes, including the all-components-skipped edge case.

QA 2026-08-16: CLI, HTTP/UDS-backed native reads, and Web inventory agreed on two skills, two MCP
servers, and three ordered component-skipped diagnostics. Restart preserved ingest diagnostics and
recomputed live health. The scenario passed; the adjacent daemon stop timeout is tracked separately.
