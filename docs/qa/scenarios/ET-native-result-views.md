---
id: ET-native-result-views
area: ET
title: Select compact and full native tool results
persona: Ada
journey: J-agent-marketplace-parity
expected: Native tools default to compact, full preserves the complete record, selectors fail before execution when invalid, and Loop references select the correct contract.
entry_points: compozy tool info; compozy tool invoke; compozy loop
qa_status: pass
bug_ids:
fix_status:
retest_status: pass
fix_commits:
evidence: docs/qa/reports/2026-11-10-native-result-views.md
last_report: docs/qa/reports/2026-11-10-native-result-views.md
---

Walk E2E-001–003 in an isolated provider-free ACP lab. Measure status, create,
archive and spawn in both views. Status/archive/spawn must save at least 80%; create
is recorded without a threshold. Reuse compact IDs through the session API. Inspect
output_views via tool info, reject summary/non-string and a selector on skill_view.
Validate a reference to session.lineage.permission_policy.tools: compact must fail
with result_view_field_requires_full naming inspect, full must validate and complete.
Verify hook pin refusals and MCP both-view schemas through their owning focused suites.
