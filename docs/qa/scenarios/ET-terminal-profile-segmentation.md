---
id: ET-terminal-profile-segmentation
area: ET
title: Keep terminal work isolated by profile
persona: Ada
journey: J-scope-work-by-profile
expected: A terminal belongs to the profile that opened it; profile switches re-scope the list, dock badge, stream, and journal; aggregate reads label every owner; archiving closes owned terminals but preserves history; workspace deletion removes both.
entry_points: Web profile switcher; Terminal app; --profile; --all-profiles; HTTP and UDS profile selectors
qa_status: untested
bug_ids:
fix_status:
retest_status:
fix_commits:
evidence:
last_report:
overlaps: ET-profile-aggregate-owner-labels; ET-profile-stream-isolation
---

Flagged by integrated-terminal task 06. Task 10 owns the real-user walk, evidence, and verdict.

Walk:

1. Open a terminal under profile A, switch to B, and verify list, badge, catalog stream, and journal re-scope.
2. Switch back to A and confirm the terminal is still running.
3. Use the all-profiles read view and verify every terminal and journal row labels its owner.
4. Archive A and verify its terminals close while history stays readable; delete the workspace and verify both disappear.

