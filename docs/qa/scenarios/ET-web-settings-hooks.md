---
id: ET-web-settings-hooks
area: ET
title: Manage hooks from the dedicated Settings page
persona: Vera
journey: J-extension-policy-admin
expected: Settings Hooks survives refresh, renders hook and notification preset management without any installed-extension or extension-policy controls, and reports restart-required hook mutations truthfully.
entry_points: /settings/hooks; Settings section navigation
qa_status: untested
bug_ids:
fix_status:
retest_status:
fix_commits:
evidence: docs/qa/reports/2026-07-15-marketplace.md; web/e2e/settings-hooks.spec.ts
last_report: docs/qa/reports/2026-07-15-marketplace.md
overlaps: ET-044; NB-044; MS-031
---

Added by marketplace Task 07. `/settings/hooks-extensions` has no alias or redirect.
