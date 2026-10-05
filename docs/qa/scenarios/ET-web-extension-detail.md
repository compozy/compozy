---
id: ET-web-extension-detail
area: ET
title: Inspect an installed extension
persona: Bruno
journey: J-marketplace-acquisition
expected: The extension detail route survives refresh and renders runtime state, required and missing environment variables, bound key names, kit inventory, diagnostics and last_error severity, provenance, and trust.
entry_points: /marketplace/extension/$entryId?installed_name=$name; Marketplace Extensions Installed row
qa_status: skipped
bug_ids:
fix_status:
retest_status:
fix_commits:
evidence: docs/qa/evidence/2026-08-10-loop-browser-runtime-closeout/extension-spec-cycle-trust.png; docs/qa/evidence/2026-08-10-loop-browser-runtime-closeout/extension-update-precondition.md
last_report: docs/qa/reports/2026-10-02-untested.md
overlaps: ET-015; ET-022; ET-023; ET-web-extension-kit-inventory
---

Open an installed extension from Marketplace and refresh its detail route. Compare live resource counts, runtime state, environment binding names, diagnostics and provenance with the daemon response. Missing required environment must point to a usable repair action without exposing a value. Retry a failed lifecycle action and verify the last-good generation remains truthful.


2026-09-27 scope update: current coverage follows the surviving product surfaces; a fresh walk is required.

2026-10-05: Deferred from this QA cycle by the user's explicit scope reduction.
Coverage remains outstanding; this skip is not a passing result. Resume from the dated
report's session matrix in a future QA cycle.
