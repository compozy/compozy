---
id: ET-web-marketplace-sources-manage
area: ET
title: Manage source availability in Settings
persona: Bruno
journey: J-marketplace-acquisition
expected: "Preset switches and custom removal update Browse while installed extensions and their origins survive; degraded rows expose diagnostics."
entry_points: "/settings/marketplace; /marketplace/installed"
qa_status: pass
bug_ids:
fix_status:
retest_status: pass
fix_commits:
evidence: docs/qa/evidence/2026-09-13-marketplace-catalog/source-removed-inventory.json
last_report: docs/qa/reports/2026-09-13-marketplace-catalog.md
overlaps:
---

QA 2026-09-13: the current hard-cut contract passed the scoped live/API/browser walks and applicable unchanged owning integration checks. See the dated report for exact evidence and boundaries; historical notes below do not redefine the current catalog.


Toggle a preset, make a custom source unreachable, inspect the last-read state and diagnostics, remove it, re-add its original ref, and reject a different ref under the retained name. Save catalog URL/TTL/timeout and re-read Settings.

Task_08 defines the behavior; task_10 owns the live walk and screenshots. Focused integration receipts are not QA verdicts.
