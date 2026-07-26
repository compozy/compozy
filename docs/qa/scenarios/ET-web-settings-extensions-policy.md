---
id: ET-web-settings-extensions-policy
area: ET
title: Manage extension marketplace policy
persona: Vera
journey: J-extension-policy-admin
expected: Settings Extensions exposes only registry, base_url, and allow_unverified; saving preserves hidden resource policy, reports the real config lifecycle, and a live allow_unverified flip immediately refreshes Marketplace trust affordances.
entry_points: /settings/extensions; Marketplace blocked extension detail
qa_status: untested
bug_ids:
fix_status:
retest_status:
fix_commits:
evidence: /Users/pedronauck/dev/qa-labs/agh-marketplace-task11-final-20260715-20260716-011529-818379-lab/qa-artifacts/qa/notes/marketplace-management-lifecycle.json; /Users/pedronauck/dev/qa-labs/agh-marketplace-task11-final-20260715-20260716-011529-818379-lab/qa-artifacts/qa/web/extensions-policy-after.png
last_report: docs/qa/reports/2026-07-15-marketplace.md
overlaps: ET-045; ET-web-ext-policy-block
---

Added by marketplace Task 07. The deleted hooks-extensions Web route is not a valid recovery path.

2026-07-21: qa_status reset to untested — the opendesign redesigns restructured this scenario's web entry surface (task detail/run detail 3-tab IA, settings takeover shell, or providers page); the pass verdict predates that surface.
