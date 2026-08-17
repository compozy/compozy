---
id: ET-agent-plugin-conformance-walk
area: ET
title: Prove Agent Plugins 1.0.0 minimum conformance item by item
persona: Ada
journey: J-extension-agent-authoring
expected: "One recorded evidence bundle proves all eight Agent Plugins 1.0.0 minimum-conformance items against the shipped binary: directory loading; pinned closed plugin schema selection; ignored unowned extensions; fixed-location discovery; pinned MCP schema plus stdio and streamable-http; absolute PLUGIN_ROOT and PLUGIN_DATA expansion in args/env/cwd; single-token stdio command with plugin-root default cwd; and support for both skills and MCP servers."
entry_points: compozy extension validate <fixture> -o json; compozy extension install <fixture> --allow-unverified --yes; compozy extension enable <name>; compozy session new|prompt|events; compozy extension status|inventory -o json
qa_status: pass
bug_ids: BUG-20260816-agent-plugin-path-projection; BUG-20260816-agent-plugin-validation-exit; BUG-20260816-hosted-mcp-bootstrap-projection
fix_status: fixed
retest_status: pass
fix_commits: 35100d40b55c
evidence: docs/qa/evidence/2026-08-16-agent-plugins/conformance-checklist.json; docs/qa/evidence/2026-08-16-agent-plugins/provider-matrix.json
last_report: docs/qa/reports/2026-08-16-agent-plugins.md
overlaps: ET-agent-plugin-validation; ET-agent-plugin-source-install; ET-agent-plugin-provider-delivery
---

Added by Agent Plugins task 07 as the external-claim evidence gate. Task 08 must record one numbered
result per standard item, the exact pinned schema identifiers, the stamped build SHA, the command or
session observable, and an evidence path. A summary pass is insufficient: the report must link
`docs/qa/evidence/2026-08-16-agent-plugins/conformance-checklist.json`, whose eight records each carry
`item`, `status`, `observable`, and `evidence` fields.

The walk also probes the conformance boundary: invalid manifest rejects the package, invalid
components preserve valid siblings, unowned extension namespaces are ignored without validation,
and no validation-only action writes registry, data, process, or resource state.

QA 2026-08-16: all eight numbered checks passed against the rebuilt isolated-lab binary. The two
provider-capable sessions proved the runtime expansion and dual resource delivery items; the
checklist carries exact schema ids and one observable/evidence pair per item.
