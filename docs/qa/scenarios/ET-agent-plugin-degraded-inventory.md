---
id: ET-agent-plugin-degraded-inventory
area: ET
title: Inspect a partially ingested Agent Plugins package
persona: Ada
journey: J-extension-kit-lifecycle
expected: A package with valid skills and MCP servers plus invalid or unsupported siblings installs its usable components, retains ordered `extension_agent_plugin_component_skipped` diagnostics, distinguishes later `extension_mcp_server_unhealthy` live diagnostics, and never presents a fully skipped portable package as a native empty extension.
entry_points: compozy extension status|inventory|list -o human|json|jsonl|toon; GET /api/extensions, /api/extensions/:name, and /api/extensions/:name/inventory over HTTP and UDS; compozy__extensions_info|list|inventory; Web /marketplace/extension/:entryId?installed_name=:name
qa_status: untested
bug_ids:
fix_status:
retest_status:
fix_commits:
evidence:
last_report:
overlaps: ET-ext-inventory; ET-web-extension-kit-inventory; ET-web-extension-detail
---

QA impact 2026-08-16: portable ingest diagnostics and live health now share the first-class extension
payload. Task 08 must prove their ordering, codes, persistence across restart, and visibility on all
listed read planes, including the all-components-skipped edge case.
