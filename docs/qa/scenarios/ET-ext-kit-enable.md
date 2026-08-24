---
id: ET-ext-kit-enable
area: ET
title: Publish an extension kit on enable
persona: Bruno
journey: J-extension-kit-lifecycle
expected: Install enables packaged agents, sidecars, automation, and layouts by default; a profile disable removes only resources owned by that extension and visible in that profile; re-enable restores the same placed set without changing another profile.
entry_points: /docs/extensions/install|develop|manifest; compozy extension install; POST /api/extensions/preview-install; POST /api/extensions; compozy --profile <name> extension enable|disable -o json|jsonl|toon; GET|PUT /api/extensions/:name/enablement over HTTP and UDS; GET /api/extensions/:name/inventory?profile=<name>; compozy__extensions_enable|disable
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

Walk: install a kit through the Marketplace or CLI, inspect the preview response, and verify the
default-on resources and automation before using profile-scoped enable/disable. Compare the same
install, preview, inventory, and enablement result through HTTP and UDS; then prove disable and
re-enable affect only the selected profile.

2026-08-23 qa-impact (Profiles): enablement authority moved. The machine-wide `extensions.enabled`
column was dropped and replaced by per-profile exception rows, where an absent row means enabled —
so "enabled" is now a per-profile fact and a new profile starts with everything on. Already
`untested`, so no reset was needed. Walk the enable and disable verbs inside one profile and
confirm the other profiles are unaffected and that no machine-wide enabled field survives in any
payload. The cross-profile contract is owned by `ET-extension-profile-enablement`.
