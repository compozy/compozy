---
id: ET-web-marketplace-sources-manage
area: ET
title: Manage source availability in Settings
persona: Bruno
journey: J-marketplace-acquisition
expected: "Preset switches and custom removal update Browse while installed extensions and their origins survive; degraded rows expose diagnostics."
entry_points: "/settings/marketplace; /marketplace/installed"
qa_status: untested
bug_ids:
fix_status:
retest_status:
fix_commits:
evidence:
last_report:
overlaps:
---

Toggle a preset, make a custom source unreachable, inspect the last-read state and diagnostics, remove it, re-add its original ref, and reject a different ref under the retained name. Save catalog URL/TTL/timeout and re-read Settings.

Task_08 defines the behavior; task_10 owns the live walk and screenshots. Focused integration receipts are not QA verdicts.
