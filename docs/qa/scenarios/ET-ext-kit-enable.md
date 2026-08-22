---
id: ET-ext-kit-enable
area: ET
title: Publish an extension kit on enable
persona: Bruno
journey: J-extension-kit-lifecycle
expected: Install enables packaged agents, sidecars, automation, and layouts by default; a profile disable removes only resources owned by that extension and visible in that profile; re-enable restores the same placed set without changing another profile.
entry_points: /docs/extensions/install|develop|manifest; compozy --profile <name> extension enable|disable -o json|jsonl|toon; GET|PUT /api/extensions/:name/enablement over HTTP and UDS; GET /api/extensions/:name/inventory?profile=<name>; compozy__extensions_enable|disable
qa_status: untested
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

QA impact 2026-08-22: install is default-on and enablement is profile-scoped. Reset for the final
profiles QA cycle.
