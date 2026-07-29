---
id: ET-extension-passive-update-discovery
area: ET
title: Discover extension updates without flags
persona: Ada
journey: J-extension-distribution
expected: Installed extensions advertise a newer remote version in list and search human output and structured fields without an update-check flag, while a degraded discovery source never blocks local inventory.
entry_points: compozy extension list; compozy extension search; GET /api/extensions; GET /api/extensions/search
qa_status: untested
bug_ids:
fix_status:
retest_status:
fix_commits:
evidence:
last_report:
overlaps: ET-015; ET-016; ET-019
---

Added by ext-improvs Task 07. Exercise both a healthy catalog and one degraded source, then compare
the human Update column with `update_available`, `installed_version`, and `remote_version` in the
structured responses.
