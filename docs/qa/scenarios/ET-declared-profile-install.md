---
id: ET-declared-profile-install
area: ET
title: Install an extension that declares profiles
persona: Bruno
journey: J-adopt-extension-profiles
expected: Install preview names every profile as create or bind, lists credential needs and placements, and changes no state; confirmed install creates each missing profile once without activating it, preserves existing profiles, and keeps needs-setup state after update, restart, and extension removal until the profile secret is set.
entry_points: /marketplace/extensions; compozy extension install; POST /api/extensions/preview-install; POST /api/extensions
qa_status: skipped
bug_ids:
fix_status:
retest_status:
fix_commits:
evidence:
last_report: docs/qa/reports/2026-10-02-untested.md
overlaps: ET-dormant-extension-placement; ET-extension-profile-enablement
---

Flagged by profiles Task 09. The final profiles QA cycle owns the first walk.

2026-10-05: Deferred from this QA cycle by the user's explicit scope reduction.
Coverage remains outstanding; this skip is not a passing result. Resume from the dated
report's session matrix in a future QA cycle.
