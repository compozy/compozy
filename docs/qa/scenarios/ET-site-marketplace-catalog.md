---
id: ET-site-marketplace-catalog
area: ET
title: Public /marketplace renders the checked-in catalog snapshot with daemon-search CTAs
persona: Dora
journey: J-evaluate-compozy-beta
expected: /marketplace renders one searchable extension catalog from v3, including the 17 packaged MCP servers and three existing extensions. Direct /marketplace/<entry_id> details show actual metadata, inputs, provenance and current extension commands. Retired kind paths are not found. Bundled resources remain separate and usable. No old feed fallback, invented runtime state, popularity or secret values appear.
entry_points: compozy.com /marketplace; /marketplace/context7; /marketplace/herdr-bridge; /marketplace/bundled/spec-cycle
qa_status: skipped
bug_ids:
fix_status:
retest_status:
fix_commits:
evidence: docs/qa/evidence/2026-09-13-marketplace-catalog/site-readback.json
last_report: docs/qa/reports/2026-10-02-untested.md
overlaps: ET-site-docs-single-tree-ia
---

/marketplace renders one searchable extension catalog from v3, including the 17 packaged MCP servers and three existing extensions. Direct /marketplace/<entry_id> details show actual metadata, inputs, provenance and current extension commands. Retired kind paths are not found. Bundled resources remain separate and usable. No old feed fallback, invented runtime state, popularity or secret values appear.

Walk each listed public entry point, then reload and read the stored result independently. Exercise rejection and recovery with the same workspace and profile to confirm that unrelated state remains intact.

2026-09-27 scope update: retired product surfaces were removed from this active scenario. Historical evidence remains in the dated reports; this revised contract requires a fresh walk.

2026-10-05: Deferred from this QA cycle by the user's explicit scope reduction.
Coverage remains outstanding; this skip is not a passing result. Resume from the dated
report's session matrix in a future QA cycle.
