---
id: ET-window-manager-hooks-resources
area: ET
title: Extend layouts without exposing pointer or execution authority
persona: Ada
journey: J-administer-window-manager
expected: Extension and bundle `window_layout` resources are schema-validated, scope-filtered, immutable to consumers, and applicable only through the semantic service; async hooks fire after committed layout apply, desktop create/delete, and window move with workspace/revision/change metadata; preview, rejection, pointer motion, focus, and desktop switching emit no extension hook.
entry_points: extension resources; bundles; compozy__resources_list; compozy__layout_arrange; hook catalog and runs
qa_status: pass
bug_ids:
fix_status:
retest_status: pass
fix_commits:
evidence: docs/qa/evidence/2026-08-01-window-tabs/agent-02-layouts-applies-now.png; docs/qa/reports/2026-08-01-window-tabs.md
last_report: docs/qa/reports/2026-08-01-window-tabs.md
overlaps: ET-window-manager-layout-recovery; ET-window-manager-public-parity
---

story: As an extension author, I can contribute deterministic layout intent and observe durable lifecycle changes without receiving mutable internals or pointer churn.

qa-impact: 2026-07-22 added the `window_layout` resource codec/projector, extension `window_layouts` publication grants, strict bundle-authored layout loading and activation projection, plus async `window_manager.*` lifecycle hooks. Flag only; the next QA cycle owns live retesting.

qa-impact: 2026-07-31 added opened, closed, activated, grouped, and ungrouped hooks for committed tab
operations while retaining pointer and focus silence. Reset for hook observability.
