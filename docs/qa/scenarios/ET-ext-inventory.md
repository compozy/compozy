---
id: ET-ext-inventory
area: ET
title: Inspect shipped and live extension resources
persona: Ada
journey: J-extension-kit-lifecycle
expected: Inventory returns the full extension envelope with shipped and live resources per kind, bound environment key names, and no secret values or cross-workspace state.
entry_points: compozy extension inventory <name> -o json|jsonl|toon; GET /api/extensions/:name/inventory over HTTP and UDS; compozy__extensions_inventory
qa_status: untested
bug_ids:
fix_status:
retest_status:
fix_commits:
evidence:
last_report:
overlaps: ET-web-extension-kit-inventory; ET-web-extensions-manage
---

QA impact 2026-08-02: new read-only agent surface. Compare CLI, HTTP, UDS, and native payloads before
and after enable and prove the global published instance never absorbs a workspace dev overlay.
