---
id: ET-dormant-extension-placement
area: ET
title: Adopt a dormant extension profile placement
persona: Bruno
journey: J-adopt-extension-profiles
expected: A resource placed in an absent profile stays hidden and appears as dormant in extension detail and install preview; creating that named profile removes the hint and publishes the resource there without changing another profile.
entry_points: /marketplace/extension/{entry_id}; extension install preview; compozy profile create; GET /api/extensions/{name}; POST /api/extensions/preview-install
qa_status: skipped
bug_ids:
fix_status:
retest_status:
fix_commits:
evidence:
last_report: docs/qa/reports/2026-10-02-untested.md
overlaps: ET-declared-profile-install; MS-repo-profile-layer-adoption
---

Flagged by profiles Task 09. The final profiles QA cycle owns the first walk.

Walk: install or inspect a kit with a placement for an absent profile, verify that the resource is
hidden from the active catalog, and inspect extension detail plus install preview for the dormant
placement and profile-create action. Create the named profile, then confirm the placement publishes
there without changing another profile.

2026-10-05: Deferred from this QA cycle by the user's explicit scope reduction.
Coverage remains outstanding; this skip is not a passing result. Resume from the dated
report's session matrix in a future QA cycle.
