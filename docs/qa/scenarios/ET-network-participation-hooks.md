---
id: ET-network-participation-hooks
area: ET
title: Enforce participation hooks without widening authority
persona: Ada
journey: J-administer-network-live
expected: A network.participation.pre_resolve hook may deny or narrow an authorized request but cannot widen it, and network.participation.resolved publishes the immutable workspace-scoped Spec without raw secrets or a second enrollment path.
entry_points: extension manifest hook declarations; network.participation.pre_resolve; network.participation.resolved; extension host structured events and diagnostics
qa_status: pass
bug_ids: BUG-20260715-participation-hooks-inert-after-boot
fix_status: fixed
retest_status: pass
fix_commits: pending final whole-diff commit
evidence: docs/qa/evidence/2026-07-14-network-changes/ch-network-admin-lifecycle.md
last_report: docs/qa/reports/2026-07-14-network-changes.md
overlaps: ET-026;NB-agent-manages-participation
---

Derived from the extension side effect of the administration flow and the extensibility audit. Exercise allow, deny, narrow, attempted widen, and cross-workspace inputs through a real installed test extension; the hook may influence one resolution only and never mutates an in-flight snapshot.

This is not a browser scenario. Structured extension-host evidence and an independent owner read settle it; bundle confirmation remains owned by `ET-025` through `ET-030`.
