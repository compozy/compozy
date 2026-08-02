---
id: ET-ext-kit-enable
area: ET
title: Publish an extension kit on enable
persona: Bruno
journey: J-extension-kit-lifecycle
expected: Install leaves packaged agents and sidecars, automation, and layouts inert; enable publishes the complete owned set and reports started automation; disable removes only that extension's live resources.
entry_points: /docs/extensions/install|develop|manifest; compozy extension install|enable|disable -o json|jsonl|toon; POST /api/extensions/:name/enable|disable over HTTP and UDS; GET /api/extensions/:name/inventory; compozy__extensions_enable|disable
qa_status: pass
bug_ids: BUG-20260802-extension-agent-edit-reset;BUG-20260802-manifest-mcp-tool-handler
fix_status: fixed
retest_status: pass
fix_commits: 4f1ceef;881a254
evidence: /Users/pedronauck/dev/qa-labs/compozy-devtool-oss-launch-20260802-195112-911343-lab/qa-artifacts/qa
last_report: docs/qa/reports/2026-08-02-bundles-removal.md
overlaps: ET-019; ET-021; ET-window-manager-hooks-resources
---

QA impact 2026-08-02: new extension-kit lifecycle behavior. Walk install, enable, fresh inventory,
automation startup, agent-conflict refusal, disable, and ownership-isolated cleanup in the isolated
QA cycle. Enable returns `automation_started` as action output rather than status data.
