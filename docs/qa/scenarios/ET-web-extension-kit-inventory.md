---
id: ET-web-extension-kit-inventory
area: ET
title: Inspect an extension kit in Marketplace
persona: Bruno
journey: J-extension-kit-lifecycle
expected: Extension detail shows shipped and live resource counts by kind, live badges, bound environment key names from daemon responses without inventing controls or leaking secrets.
entry_points: /marketplace/extension/$entryId?installed_name=$name; /marketplace/extensions; browser extension detail and confirmation dialog
qa_status: skipped
bug_ids:
fix_status:
retest_status:
fix_commits:
evidence: /Users/pedronauck/dev/qa-labs/compozy-critical-runtime-ui-fixes-20260807-225222-371495-lab/qa-artifacts/qa/spec-cycle-trusted-detail.png
last_report: docs/qa/reports/2026-10-02-untested.md
overlaps: ET-ext-inventory; ET-web-extension-detail; ET-web-extensions-manage
---

Extension detail shows shipped and live resource counts by kind, live badges, bound environment key names from daemon responses without inventing controls or leaking secrets.

Walk each listed public entry point, then reload and read the stored result independently. Exercise rejection and recovery with the same workspace and profile to confirm that unrelated state remains intact.

2026-09-27 scope update: retired product surfaces were removed from this active scenario. Historical evidence remains in the dated reports; this revised contract requires a fresh walk.

2026-10-05: Deferred from this QA cycle by the user's explicit scope reduction.
Coverage remains outstanding; this skip is not a passing result. Resume from the dated
report's session matrix in a future QA cycle.
